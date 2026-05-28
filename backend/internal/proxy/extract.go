package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"github.com/yaoyifcci/api-tracker/internal/config"
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

type toolUseBlock struct {
	id    string
	name  string
	input strings.Builder
}

// parseAnthropicSSEBuffer handles Anthropic Messages API streaming.
func parseAnthropicSSEBuffer(buf []byte) (map[string]interface{}, UsageInfo) {
	var contentParts []string
	var u UsageInfo
	var messageID, modelName string
	var currentEvent string
	// track tool_use blocks by block index
	toolBlocks := map[int]*toolUseBlock{}
	var toolBlockOrder []int

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
		case "content_block_start":
			if cb, ok := chunk["content_block"].(map[string]interface{}); ok {
				if cbType, _ := cb["type"].(string); cbType == "tool_use" {
					idx := toInt(chunk["index"])
					tb := &toolUseBlock{
						id:   func() string { s, _ := cb["id"].(string); return s }(),
						name: func() string { s, _ := cb["name"].(string); return s }(),
					}
					toolBlocks[idx] = tb
					toolBlockOrder = append(toolBlockOrder, idx)
				}
			}
		case "content_block_delta":
			if delta, ok := chunk["delta"].(map[string]interface{}); ok {
				switch delta["type"] {
				case "text_delta":
					if text, ok := delta["text"].(string); ok {
						contentParts = append(contentParts, text)
					}
				case "input_json_delta":
					if partial, ok := delta["partial_json"].(string); ok {
						idx := toInt(chunk["index"])
						if tb, ok := toolBlocks[idx]; ok {
							tb.input.WriteString(partial)
						}
					}
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

	var contentBlocks []interface{}
	if fullContent != "" {
		contentBlocks = append(contentBlocks, map[string]interface{}{"type": "text", "text": fullContent})
	}
	for _, idx := range toolBlockOrder {
		tb := toolBlocks[idx]
		block := map[string]interface{}{"type": "tool_use", "id": tb.id, "name": tb.name}
		if inputStr := tb.input.String(); inputStr != "" {
			var inputObj interface{}
			if json.Unmarshal([]byte(inputStr), &inputObj) == nil {
				block["input"] = inputObj
			} else {
				block["input"] = inputStr
			}
		}
		contentBlocks = append(contentBlocks, block)
	}
	if len(contentBlocks) == 0 {
		contentBlocks = []interface{}{map[string]interface{}{"type": "text", "text": ""}}
	}

	respBody := map[string]interface{}{
		"id":      messageID,
		"type":    "message",
		"model":   modelName,
		"role":    "assistant",
		"content": contentBlocks,
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

// extractPreviewQuestion returns the last user text message, skipping tool_result-only messages.
func extractPreviewQuestion(body map[string]interface{}) string {
	if body == nil {
		return ""
	}
	messages, ok := body["messages"].([]interface{})
	if !ok {
		return ""
	}
	// Walk backwards: prefer the last user message that has actual text content.
	// If we only find tool_result messages, fall back to the last one and show its content.
	var lastToolResult map[string]interface{}
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok || msg["role"] != "user" {
			continue
		}
		if isToolResultContent(msg["content"]) {
			if lastToolResult == nil {
				lastToolResult = msg
			}
			continue
		}
		return extractTextContent(msg["content"])
	}
	if lastToolResult != nil {
		return extractToolResultPreview(lastToolResult["content"])
	}
	return ""
}

// isToolResultContent reports whether content is an array consisting entirely of tool_result blocks.
func isToolResultContent(content interface{}) bool {
	arr, ok := content.([]interface{})
	if !ok || len(arr) == 0 {
		return false
	}
	for _, p := range arr {
		part, ok := p.(map[string]interface{})
		if !ok || part["type"] != "tool_result" {
			return false
		}
	}
	return true
}

// extractToolResultPreview returns a compact one-line preview for tool_result content.
func extractToolResultPreview(content interface{}) string {
	arr, ok := content.([]interface{})
	if !ok {
		return extractTextContent(content)
	}
	var parts []string
	for _, p := range arr {
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		text := extractTextContent(part["content"])
		if text != "" {
			// trim to keep it short
			if len(text) > 60 {
				text = text[:60] + "…"
			}
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "(工具结果)"
	}
	return "[工具结果] " + strings.Join(parts, " | ")
}

// extractPreviewAnswer returns the assistant response text from a response body.
func extractPreviewAnswer(body map[string]interface{}) string {
	if body == nil {
		return ""
	}
	// OpenAI chat: choices[0].message.content
	if choices, ok := body["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if s, ok := msg["content"].(string); ok {
					return s
				}
			}
		}
	}
	// Anthropic: scan content blocks for text, fall back to tool_use names
	if content, ok := body["content"].([]interface{}); ok && len(content) > 0 {
		var toolNames []string
		for _, item := range content {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if part["type"] == "text" {
				if s, ok := part["text"].(string); ok && s != "" {
					return s
				}
			}
			if part["type"] == "tool_use" {
				if name, ok := part["name"].(string); ok && name != "" {
					toolNames = append(toolNames, name)
				}
			}
		}
		if len(toolNames) > 0 {
			return "(工具调用: " + strings.Join(toolNames, ", ") + ")"
		}
	}
	// OpenAI Responses: content (string)
	if s, ok := body["content"].(string); ok {
		return s
	}
	return ""
}

// extractToolUseNames returns tool names called in a response body (Anthropic + OpenAI).
func extractToolUseNames(body map[string]interface{}) []string {
	if body == nil {
		return nil
	}
	var names []string
	// Anthropic: content[].type == "tool_use"
	if content, ok := body["content"].([]interface{}); ok {
		for _, item := range content {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if part["type"] == "tool_use" {
				if name, ok := part["name"].(string); ok && name != "" {
					names = append(names, name)
				}
			}
		}
	}
	// OpenAI chat: choices[0].message.tool_calls[].function.name
	if choices, ok := body["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
					for _, tc := range toolCalls {
						tcMap, ok := tc.(map[string]interface{})
						if !ok {
							continue
						}
						if fn, ok := tcMap["function"].(map[string]interface{}); ok {
							if name, ok := fn["name"].(string); ok && name != "" {
								names = append(names, name)
							}
						}
					}
				}
			}
		}
	}
	return names
}

func extractTextContent(content interface{}) string {
	if s, ok := content.(string); ok {
		return s
	}
	if arr, ok := content.([]interface{}); ok {
		for _, p := range arr {
			part, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			switch part["type"] {
			case "text":
				if s, ok := part["text"].(string); ok {
					return s
				}
			case "tool_result":
				if s := extractTextContent(part["content"]); s != "" {
					return s
				}
			}
		}
	}
	if content == nil {
		return ""
	}
	b, _ := json.Marshal(content)
	return string(b)
}
