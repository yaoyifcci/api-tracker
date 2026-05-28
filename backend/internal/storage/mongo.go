package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/yaoyifcci/api-tracker/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Store struct {
	coll *mongo.Collection
}

func NewStore(uri, dbName string) (*Store, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	coll := client.Database(dbName).Collection("api_requests")
	s := &Store{coll: coll}
	s.ensureIndexes()
	return s, nil
}

// ensureIndexes creates the indexes backing the List filters. Default (auto-generated)
// index names are used intentionally so that re-declaring an already-existing index
// (e.g. the legacy {timestamp:-1} / {provider:1} indexes) is an idempotent no-op rather
// than an IndexKeySpecsConflict that would fail the whole CreateMany batch on upgrade.
func (s *Store) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "provider", Value: 1}, {Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "status_code", Value: 1}, {Key: "timestamp", Value: -1}}},
	})
}

func (s *Store) Save(ctx context.Context, r *model.APIRequest) error {
	_, err := s.coll.InsertOne(ctx, r)
	return err
}

// RequestSummary is the lightweight type returned by the list endpoint.
// Heavy fields (req_body, resp_body, headers) are excluded via projection.
type RequestSummary struct {
	ID               string    `bson:"_id"               json:"id"`
	Timestamp        time.Time `bson:"timestamp"         json:"timestamp"`
	Provider         string    `bson:"provider"          json:"provider"`
	Model            string    `bson:"model"             json:"model"`
	StatusCode       int       `bson:"status_code"       json:"status_code"`
	PromptTokens     int       `bson:"prompt_tokens"     json:"prompt_tokens"`
	CompletionTokens int       `bson:"completion_tokens" json:"completion_tokens"`
	TotalTokens      int       `bson:"total_tokens"      json:"total_tokens"`
	CacheReadTokens  int       `bson:"cache_read_tokens"  json:"cache_read_tokens"`
	CacheWriteTokens int       `bson:"cache_write_tokens" json:"cache_write_tokens"`
	DurationMS       int64     `bson:"duration_ms"       json:"duration_ms"`
	IsStreaming      bool      `bson:"is_streaming"      json:"is_streaming"`
	RespID           string    `bson:"resp_id"           json:"resp_id"`
	ToolUseNames     []string  `bson:"tool_names"        json:"tool_names"`
	PreviewQuestion  string    `bson:"preview_question"  json:"preview_question"`
	PreviewAnswer    string    `bson:"preview_answer"    json:"preview_answer"`
}

type ListResult struct {
	Data  []RequestSummary `json:"data"`
	Total int64            `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

// ListFilter holds optional filtering criteria for List. Zero/empty fields are ignored.
// StatusCode (exact) takes precedence over StatusClass (range bucket) when both are set.
type ListFilter struct {
	Provider    string
	StatusCode  int
	StatusClass string // "2xx" / "3xx" / "4xx" / "5xx"
	StartTime   time.Time
	EndTime     time.Time
}

func (f ListFilter) build() bson.D {
	and := bson.A{}
	if f.Provider != "" {
		and = append(and, bson.D{{Key: "provider", Value: f.Provider}})
	}
	if f.StatusCode != 0 {
		and = append(and, bson.D{{Key: "status_code", Value: f.StatusCode}})
	} else if f.StatusClass != "" {
		var lo, hi int
		switch f.StatusClass {
		case "2xx":
			lo, hi = 200, 299
		case "3xx":
			lo, hi = 300, 399
		case "4xx":
			lo, hi = 400, 499
		case "5xx":
			lo, hi = 500, 599
		}
		if hi > 0 {
			and = append(and, bson.D{{Key: "status_code", Value: bson.D{{Key: "$gte", Value: lo}, {Key: "$lte", Value: hi}}}})
		}
	}
	if !f.StartTime.IsZero() || !f.EndTime.IsZero() {
		rng := bson.D{}
		if !f.StartTime.IsZero() {
			rng = append(rng, bson.E{Key: "$gte", Value: f.StartTime})
		}
		if !f.EndTime.IsZero() {
			rng = append(rng, bson.E{Key: "$lte", Value: f.EndTime})
		}
		and = append(and, bson.D{{Key: "timestamp", Value: rng}})
	}
	if len(and) == 0 {
		return bson.D{}
	}
	return bson.D{{Key: "$and", Value: and}}
}

func (s *Store) List(ctx context.Context, f ListFilter, page, limit int) (*ListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	filter := f.build()
	total, err := s.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	pipeline := bson.A{
		bson.D{{Key: "$match", Value: filter}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: -1}}}},
		bson.D{{Key: "$skip", Value: int64((page - 1) * limit)}},
		bson.D{{Key: "$limit", Value: int64(limit)}},
		// extract resp_body.id for all records (including those without a separate resp_id field)
		bson.D{{Key: "$addFields", Value: bson.D{{Key: "resp_id", Value: "$resp_body.id"}}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "req_body", Value: 0},
			{Key: "resp_body", Value: 0},
			{Key: "req_headers", Value: 0},
			{Key: "resp_headers", Value: 0},
			{Key: "target_url", Value: 0},
			{Key: "method", Value: 0},
			{Key: "path", Value: 0},
		}}},
	}
	cursor, err := s.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var items []RequestSummary
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []RequestSummary{}
	}
	return &ListResult{Data: items, Total: total, Page: page, Limit: limit}, nil
}

func (s *Store) GetByID(ctx context.Context, id string) (*model.APIRequest, error) {
	var r model.APIRequest
	err := s.coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&r)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &r, err
}

// StatsResult holds aggregated token counts across all stored requests.
type StatsResult struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
}

func (s *Store) GetStats(ctx context.Context) (*StatsResult, error) {
	pipeline := bson.A{bson.D{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: nil},
		{Key: "prompt_tokens", Value: bson.D{{Key: "$sum", Value: "$prompt_tokens"}}},
		{Key: "completion_tokens", Value: bson.D{{Key: "$sum", Value: "$completion_tokens"}}},
		{Key: "total_tokens", Value: bson.D{{Key: "$sum", Value: "$total_tokens"}}},
		{Key: "cache_read_tokens", Value: bson.D{{Key: "$sum", Value: "$cache_read_tokens"}}},
		{Key: "cache_write_tokens", Value: bson.D{{Key: "$sum", Value: "$cache_write_tokens"}}},
	}}}}

	cursor, err := s.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result StatsResult
	if cursor.Next(ctx) {
		var row struct {
			PromptTokens     int64 `bson:"prompt_tokens"`
			CompletionTokens int64 `bson:"completion_tokens"`
			TotalTokens      int64 `bson:"total_tokens"`
			CacheReadTokens  int64 `bson:"cache_read_tokens"`
			CacheWriteTokens int64 `bson:"cache_write_tokens"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		result = StatsResult{
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
			TotalTokens:      row.TotalTokens,
			CacheReadTokens:  row.CacheReadTokens,
			CacheWriteTokens: row.CacheWriteTokens,
		}
	}
	return &result, nil
}
