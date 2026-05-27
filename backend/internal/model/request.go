package model

import "time"

type APIRequest struct {
	ID               string            `bson:"_id"               json:"id"`
	Timestamp        time.Time         `bson:"timestamp"         json:"timestamp"`
	Provider         string            `bson:"provider"          json:"provider"`
	TargetURL        string            `bson:"target_url"        json:"target_url"`
	Method           string            `bson:"method"            json:"method"`
	Path             string            `bson:"path"              json:"path"`
	RequestHeaders   map[string]string `bson:"req_headers"       json:"req_headers"`
	RequestBody      interface{}       `bson:"req_body"          json:"req_body"`
	StatusCode       int               `bson:"status_code"       json:"status_code"`
	ResponseHeaders  map[string]string `bson:"resp_headers"      json:"resp_headers"`
	ResponseBody     interface{}       `bson:"resp_body"         json:"resp_body"`
	Model            string            `bson:"model"             json:"model"`
	PromptTokens     int               `bson:"prompt_tokens"     json:"prompt_tokens"`
	CompletionTokens int               `bson:"completion_tokens" json:"completion_tokens"`
	TotalTokens      int               `bson:"total_tokens"      json:"total_tokens"`
	CacheReadTokens  int               `bson:"cache_read_tokens"  json:"cache_read_tokens"`
	CacheWriteTokens int               `bson:"cache_write_tokens" json:"cache_write_tokens"`
	DurationMS       int64             `bson:"duration_ms"       json:"duration_ms"`
	IsStreaming      bool              `bson:"is_streaming"      json:"is_streaming"`
}
