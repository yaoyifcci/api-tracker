package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/yaoyi/aitrace/internal/model"
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

func (s *Store) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "provider", Value: 1}}},
	})
}

func (s *Store) Save(ctx context.Context, r *model.APIRequest) error {
	_, err := s.coll.InsertOne(ctx, r)
	return err
}

type ListResult struct {
	Data  []model.APIRequest `json:"data"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
}

func (s *Store) List(ctx context.Context, page, limit int) (*ListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	total, err := s.coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))
	cursor, err := s.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, err
	}
	var items []model.APIRequest
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.APIRequest{}
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
