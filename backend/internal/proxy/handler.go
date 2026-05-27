package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yaoyi/aitrace/internal/config"
	"github.com/yaoyi/aitrace/internal/model"
	"github.com/yaoyi/aitrace/internal/storage"
)

var hopByHopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive",
	"Proxy-Authenticate", "Proxy-Authorization",
	"Te", "Trailers", "Transfer-Encoding", "Upgrade",
}

type Handler struct {
	endpointMap        map[string]*config.Endpoint
	defaultEP          *config.Endpoint
	defaultAnthropicEP *config.Endpoint
	defaultResponsesEP *config.Endpoint
	store              *storage.Store
	client             *http.Client
}

func NewHandler(cfg *config.Config, store *storage.Store) *Handler {
	return &Handler{
		endpointMap:        cfg.EndpointMap(),
		defaultEP:          cfg.DefaultEndpoint(),
		defaultAnthropicEP: cfg.EndpointByName(cfg.DefaultAnthropic),
		defaultResponsesEP: cfg.EndpointByName(cfg.DefaultOpenAIResponses),
		store:              store,
		client:             &http.Client{Timeout: 120 * time.Second},
	}
}

func (h *Handler) Handle(c *gin.Context) {
	start := time.Now()

	// resolve endpoint
	ep, upstreamPath := h.resolveEndpoint(c)
	if ep == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown endpoint"})
		return
	}

	// read request body
	reqBodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "read request body failed"})
		return
	}

	// parse body for metadata
	var reqBodyMap map[string]interface{}
	if len(reqBodyBytes) > 0 {
		_ = json.Unmarshal(reqBodyBytes, &reqBodyMap)
	}

	modelName := extractModel(reqBodyMap)
	streaming := isStreaming(reqBodyMap)
	epType := ep.ResolvedType()

	// build target URL
	targetURL := strings.TrimRight(ep.URL, "/") + upstreamPath
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// inject stream_options only for openai-compatible chat completions
	// Anthropic and openai_responses don't support this field
	outBodyBytes := reqBodyBytes
	if streaming && epType == config.TypeOpenAI && len(reqBodyBytes) > 0 {
		outMap := make(map[string]interface{})
		_ = json.Unmarshal(reqBodyBytes, &outMap)
		outMap["stream_options"] = map[string]interface{}{"include_usage": true}
		if b, err := json.Marshal(outMap); err == nil {
			outBodyBytes = b
		}
	}

	// build outbound request
	outReq, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL, bytes.NewReader(outBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "build upstream request failed"})
		return
	}

	// copy headers
	for key, vals := range c.Request.Header {
		for _, v := range vals {
			outReq.Header.Add(key, v)
		}
	}
	// inject configured key — Anthropic uses x-api-key, others use Authorization: Bearer
	if ep.Key != "" {
		if epType == config.TypeAnthropic {
			outReq.Header.Set("x-api-key", ep.Key)
			outReq.Header.Del("Authorization")
		} else {
			outReq.Header.Set("Authorization", "Bearer "+ep.Key)
		}
	}
	// strip hop-by-hop
	for _, h := range hopByHopHeaders {
		outReq.Header.Del(h)
	}
	outReq.Host = outReq.URL.Host

	// capture headers for storage — mask key values
	storedHeaders := make(map[string]string)
	for k, v := range outReq.Header {
		lower := strings.ToLower(k)
		if lower == "authorization" || lower == "x-api-key" {
			storedHeaders[k] = "***"
		} else {
			storedHeaders[k] = strings.Join(v, ", ")
		}
	}

	if streaming {
		h.forwardStream(c, outReq, ep.Name, epType, modelName, reqBodyMap, storedHeaders, targetURL, start)
	} else {
		h.forwardNonStream(c, outReq, ep.Name, epType, modelName, reqBodyMap, storedHeaders, targetURL, start)
	}
}

func (h *Handler) resolveEndpoint(c *gin.Context) (*config.Endpoint, string) {
	// Named endpoint routes set "endpoint_name" via c.Set in main.go
	if name, ok := c.Get("endpoint_name"); ok {
		nameStr := name.(string)
		ep, exists := h.endpointMap[nameStr]
		if !exists {
			return nil, ""
		}
		return ep, c.Param("path")
	}

	// /v1/*path — explicit path matching only, no default fallback
	upstreamPath := "/v1" + c.Param("path")
	switch {
	case strings.HasPrefix(upstreamPath, "/v1/chat/completions"):
		return h.defaultEP, upstreamPath
	case strings.HasPrefix(upstreamPath, "/v1/messages"):
		return h.defaultAnthropicEP, upstreamPath
	case strings.HasPrefix(upstreamPath, "/v1/responses"):
		return h.defaultResponsesEP, upstreamPath
	default:
		return nil, ""
	}
}

func (h *Handler) forwardNonStream(
	c *gin.Context,
	outReq *http.Request,
	provider, epType, modelName string,
	reqBodyMap map[string]interface{},
	storedHeaders map[string]string,
	targetURL string,
	start time.Time,
) {
	resp, err := h.client.Do(outReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("upstream error: %v", err)})
		return
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "read upstream response failed"})
		return
	}

	var respBodyMap map[string]interface{}
	if len(respBodyBytes) > 0 {
		_ = json.Unmarshal(respBodyBytes, &respBodyMap)
	}

	usage := extractUsageFromResponse(respBodyMap, epType)

	// write response to client
	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	c.Writer.Write(respBodyBytes)

	// async store
	dur := time.Since(start).Milliseconds()
	go func() {
		var respInterface interface{} = respBodyMap
		if respInterface == nil {
			respInterface = string(respBodyBytes)
		}
		_ = h.store.Save(context.Background(), &model.APIRequest{
			ID:               uuid.NewString(),
			Timestamp:        start,
			Provider:         provider,
			TargetURL:        targetURL,
			Method:           outReq.Method,
			Path:             outReq.URL.Path,
			RequestHeaders:   storedHeaders,
			RequestBody:      reqBodyMap,
			StatusCode:       resp.StatusCode,
			ResponseBody:     respInterface,
			Model:            modelName,
			PromptTokens:     usage.Prompt,
			CompletionTokens: usage.Completion,
			TotalTokens:      usage.Total,
			CacheReadTokens:  usage.CacheRead,
			CacheWriteTokens: usage.CacheWrite,
			DurationMS:       dur,
			IsStreaming:      false,
		})
	}()
}

func (h *Handler) forwardStream(
	c *gin.Context,
	outReq *http.Request,
	provider, epType, modelName string,
	reqBodyMap map[string]interface{},
	storedHeaders map[string]string,
	targetURL string,
	start time.Time,
) {
	resp, err := h.client.Do(outReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("upstream error: %v", err)})
		return
	}
	defer resp.Body.Close()

	// set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	// copy other upstream headers (skip content-type we already set)
	for k, vals := range resp.Header {
		if strings.ToLower(k) == "content-type" {
			continue
		}
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		io.Copy(c.Writer, resp.Body)
		return
	}

	var buf bytes.Buffer
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*64), 1024*64)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(c.Writer, "%s\n", line)
		fmt.Fprintf(&buf, "%s\n", line)
		flusher.Flush()
		if line == "data: [DONE]" {
			break
		}
	}
	// final newline
	fmt.Fprintf(c.Writer, "\n")
	flusher.Flush()

	dur := time.Since(start).Milliseconds()
	bufSnapshot := buf.Bytes()

	go func() {
		respBody, usage := parseSSEBufferByType(bufSnapshot, epType)
		var respInterface interface{} = respBody
		if respInterface == nil {
			respInterface = string(bufSnapshot)
		}
		_ = h.store.Save(context.Background(), &model.APIRequest{
			ID:               uuid.NewString(),
			Timestamp:        start,
			Provider:         provider,
			TargetURL:        targetURL,
			Method:           outReq.Method,
			Path:             outReq.URL.Path,
			RequestHeaders:   storedHeaders,
			RequestBody:      reqBodyMap,
			StatusCode:       resp.StatusCode,
			ResponseBody:     respInterface,
			Model:            modelName,
			PromptTokens:     usage.Prompt,
			CompletionTokens: usage.Completion,
			TotalTokens:      usage.Total,
			CacheReadTokens:  usage.CacheRead,
			CacheWriteTokens: usage.CacheWrite,
			DurationMS:       dur,
			IsStreaming:      true,
		})
	}()
}
