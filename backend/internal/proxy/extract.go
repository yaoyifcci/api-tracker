package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"github.com/yaoyi/aitrace/internal/config"
)

// UsageInfo holds token counts extracted from a response.
type UsageInfo struct {
	Prompt     int
	Completion int
	Total      int
	CacheRead  int // OpenAI: usage.prompt_tokens_details.cached_tokens; Anthropic: cache_read_input_tokens
	CacheWrite int // Anthropic: cache_creation_input_tokens
}

func extractModel(body map[string]interface{}) string {
	if v, ok := body["model"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func isStreaming(body map[string]interface{}) bool {
	if v, ok := body["stream"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func extractUsageFromResponse(body map[string]interface{}, epType string) UsageInfo {
	var u UsageInfo
	switch epType {
	case config.TypeAnthropic:
		if usage, ok := body["usage"].(map[string]interface{}); ok {
			u.Prompt = toInt(usage["input_tokens"])
			u.Completion = toInt(usage["output_tokens"])
			u.CacheRead = toInt(usage["cache_read_input_tokens"])
			u.CacheWrite = toInt(usage["cache_creation_input_tokens"])
			u.Total = u.Prompt + u.Completion
		}
	case config.TypeOpenAIResponses:
		if usage, ok := body["usage"].(map[string]interface{}); ok {
			u.Prompt = toInt(usage["input_tokens"])
			u.Completion = toInt(usage["output_tokens"])
			u.Total = toInt(usage["total_tokens"])
			if u.Total == 0 {
				u.Total = u.Prompt + u.Completion
			}
			if details, ok := usage["input_token_details"].(map[string]interface{}); ok {
				u.CacheRead = toInt(details["cached_tokens"])
			}
		}
	default:
		if usage, ok := body["usage"].(map[string]interface{}); ok {
			u.Prompt = toInt(usage["prompt_tokens"])
			u.Completion = toInt(usage["completion_tokens"])
			u.Total = toInt(usage["total_tokens"])
			if u.Total == 0 {
				u.Total = u.Prompt + u.Completion
			}
			if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
				u.CacheRead = toInt(details["cached_tokens"])
			}
		}
	}
	return u
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

// parseSSEBufferByType dispatches to the appropriate SSE parser.
func parseSSEBufferByType(buf []byte, epType string) (map[string]interface{}, UsageInfo) {
	switch epType {
	case config.TypeAnthropic:
		return parseAnthropicSSEBuffer(buf)
	case config.TypeOpenAIResponses:
		return parseOpenAIResponsesSSEBuffer(buf)
	default:
		return parseOpenAISSEBuffer(buf)
	}
}

// parseOpenAISSEBuffer handles OpenAI chat completions streaming.
func parseOpenAISSEBuffer(buf []byte) (map[string]interface{}, UsageInfo) {
	var contentParts []string
	var lastUsage map[string]interface{}
	var lastChunk map[string]interface{}

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		lastChunk = chunk

		if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok {
						contentParts = append(contentParts, content)
					}
				}
			}
		}
		if usage, ok := chunk["usage"].(map[string]interface{}); ok {
			lastUsage = usage
		}
	}

	fullContent := strings.Join(contentParts, "")
	var respBody map[string]interface{}
	if lastChunk != nil {
		respBody = map[string]interface{}{
			"id":      lastChunk["id"],
			"object":  "chat.completion",
			"model":   lastChunk["model"],
			"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"role": "assistant", "content": fullContent}}},
		}
		if lastUsage != nil {
			respBody["usage"] = lastUsage
		}
	}

	var u UsageInfo
	if lastUsage != nil {
		u.Prompt = toInt(lastUsage["prompt_tokens"])
		u.Completion = toInt(lastUsage["completion_tokens"])
		u.Total = toInt(lastUsage["total_tokens"])
		if details, ok := lastUsage["prompt_tokens_details"].(map[string]interface{}); ok {
			u.CacheRead = toInt(details["cached_tokens"])
		}
	}
	return respBody, u
}

// parseAnthropicSSEBuffer handles Anthropic Messages API streaming.
func parseAnthropicSSEBuffer(buf []byte) (map[string]interface{}, UsageInfo) {
	var contentParts []string
	var u UsageInfo
	var messageID, modelName string
	var currentEvent string

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		switch currentEvent {
		case "message_start":
			if msg, ok := chunk["message"].(map[string]interface{}); ok {
				messageID, _ = msg["id"].(string)
				modelName, _ = msg["model"].(string)
				if usage, ok := msg["usage"].(map[string]interface{}); ok {
					u.Prompt = toInt(usage["input_tokens"])
					u.CacheRead = toInt(usage["cache_read_input_tokens"])
					u.CacheWrite = toInt(usage["cache_creation_input_tokens"])
				}
			}
		case "content_block_delta":
			if delta, ok := chunk["delta"].(map[string]interface{}); ok {
				if text, ok := delta["text"].(string); ok {
					contentParts = append(contentParts, text)
				}
			}
		case "message_delta":
			if usage, ok := chunk["usage"].(map[string]interface{}); ok {
				u.Completion = toInt(usage["output_tokens"])
			}
		}
	}

	u.Total = u.Prompt + u.Completion
	fullContent := strings.Join(contentParts, "")

	respBody := map[string]interface{}{
		"id":    messageID,
		"type":  "message",
		"model": modelName,
		"role":  "assistant",
		"content": []interface{}{map[string]interface{}{
			"type": "text",
			"text": fullContent,
		}},
		"usage": map[string]interface{}{
			"input_tokens":                u.Prompt,
			"output_tokens":               u.Completion,
			"cache_read_input_tokens":     u.CacheRead,
			"cache_creation_input_tokens": u.CacheWrite,
		},
	}

	return respBody, u
}

// parseOpenAIResponsesSSEBuffer handles OpenAI Responses API streaming.
func parseOpenAIResponsesSSEBuffer(buf []byte) (map[string]interface{}, UsageInfo) {
	var contentParts []string
	var responseID string
	var usageMap map[string]interface{}

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		switch chunk["type"] {
		case "response.created", "response.in_progress":
			if resp, ok := chunk["response"].(map[string]interface{}); ok {
				if id, ok := resp["id"].(string); ok {
					responseID = id
				}
			}
		case "response.output_text.delta":
			if delta, ok := chunk["delta"].(string); ok {
				contentParts = append(contentParts, delta)
			}
		case "response.completed":
			if resp, ok := chunk["response"].(map[string]interface{}); ok {
				if usage, ok := resp["usage"].(map[string]interface{}); ok {
					usageMap = usage
				}
			}
		}
	}

	fullContent := strings.Join(contentParts, "")
	var u UsageInfo
	if usageMap != nil {
		u.Prompt = toInt(usageMap["input_tokens"])
		u.Completion = toInt(usageMap["output_tokens"])
		u.Total = toInt(usageMap["total_tokens"])
		if u.Total == 0 {
			u.Total = u.Prompt + u.Completion
		}
		if details, ok := usageMap["input_token_details"].(map[string]interface{}); ok {
			u.CacheRead = toInt(details["cached_tokens"])
		}
	}

	respBody := map[string]interface{}{
		"id":      responseID,
		"object":  "response",
		"content": fullContent,
	}
	if usageMap != nil {
		respBody["usage"] = usageMap
	}

	return respBody, u
}
