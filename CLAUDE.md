# CLAUDE.md

## Language
- 使用中文回复
- Speak Chinese
This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AI Trace is a self-hosted AI API proxy that intercepts requests to OpenAI/Anthropic/compatible endpoints, stores them in MongoDB, and provides a React frontend for browsing history with token analytics.

## Commands

### Backend (Go)

```bash
cd backend
go run ./cmd/server          # start dev server (reads config.yaml + config.local.yaml)
go build ./...               # verify compilation
go build -o aitrace-server ./cmd/server   # build binary

# cross-compile for Docker (linux/amd64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o aitrace-server ./cmd/server
```

### Frontend (React + Vite)

```bash
cd frontend
npm install
npm run dev      # dev server on :5173 (proxies /api → :8080)
npm run build    # production build
npx tsc --noEmit # type check only
```

### Docker

```bash
make up    # docker compose up --build -d
make down  # docker compose down
```

## Architecture

```
Client App → Go proxy (:8080) → AI APIs
                 ↓
             MongoDB (api_requests collection)
                 ↓
React frontend (:5173 dev / :3000 Docker)
  → /api/* proxied to :8080
```

### Backend package layout

| Package | Responsibility |
|---------|---------------|
| `cmd/server` | Wires config, MongoDB, Gin router |
| `internal/config` | Three-layer config loading (see below) |
| `internal/model` | `APIRequest` struct (bson + json tags) |
| `internal/storage` | MongoDB CRUD + `GetStats()` aggregation |
| `internal/proxy` | Manual HTTP + SSE proxy; `UsageInfo` token extraction |
| `internal/api` | REST handlers: `ListRequests`, `GetRequest`, `GetStats` |

### Proxy routes (registered in `main.go`)

Gin registers `/v1/*path` as a single wildcard catch-all. Routing to the correct endpoint is done inside `resolveEndpoint` based on the path prefix — **only explicitly listed paths are forwarded; anything else returns 404**.

| Incoming path prefix | Config field | Notes |
|----------------------|-------------|-------|
| `/v1/chat/completions` | `default_endpoint` | OpenAI Chat Completions |
| `/v1/messages` | `default_anthropic_endpoint` | Anthropic Messages API |
| `/v1/responses` | `default_openai_responses_endpoint` | OpenAI Responses API |
| `/<name>/*` | named endpoint in `endpoints[]` | explicit override, any path |
| `/api/*` | — | REST API for the frontend |
| anything else under `/v1/` | — | 404 `{"error":"unknown endpoint"}` |

Named endpoint routes (`/<name>/*`) are registered individually at startup (not via a generic `/:name` wildcard) to avoid conflicting with `/api/*` and `/v1/*`.

`resolveEndpoint` uses `strings.HasPrefix` so suffixes like `/v1/chat/completions/stream` or `/v1/messages/batches` route correctly.

### SSE streaming — critical constraint

`httputil.ReverseProxy` is **not used** because its `ModifyResponse` buffers the full body before you can intercept it, breaking SSE. Instead the proxy uses a manual `bufio.Scanner` loop that writes each line to both the client and an accumulator buffer. The accumulator is parsed after the stream ends to extract token counts.

`stream_options: {include_usage: true}` is transparently injected into the outbound request body **only** for `openai`-type endpoints (Anthropic and `openai_responses` don't support it).

### Endpoint types

| Type | Auth header | SSE parsing |
|------|-------------|-------------|
| `openai` (default) | `Authorization: Bearer <key>` | `choices[0].delta.content`; usage from last chunk |
| `anthropic` | `x-api-key: <key>` | Named events: `message_start`, `content_block_delta`, `message_delta` |
| `openai_responses` | `Authorization: Bearer <key>` | Events: `response.output_text.delta`, `response.completed` |

Token extraction (`internal/proxy/extract.go`) returns a `UsageInfo` struct including `CacheRead` and `CacheWrite` fields:
- OpenAI: `usage.prompt_tokens_details.cached_tokens` → CacheRead
- Anthropic: `usage.cache_read_input_tokens` / `cache_creation_input_tokens`

### Config loading (`internal/config/config.go`)

Priority (highest wins): **env vars** → `config.local.yaml` → `config.yaml`

- `config.yaml` — committed to git, no secrets; defines endpoints with URLs and types
- `config.local.yaml` — gitignored; overlay keys for local development
- Env vars: `AITRACE_ENDPOINT_{NAME}_KEY`, `AITRACE_ENDPOINT_{NAME}_URL`, `AITRACE_MONGO_URI`, etc.
  - Name normalization: `openai-responses` → `OPENAI_RESPONSES`
- Default endpoints are set via top-level fields in `config.yaml`:
  - `default_endpoint` — used for `/v1/chat/completions`
  - `default_anthropic_endpoint` — used for `/v1/messages`
  - `default_openai_responses_endpoint` — used for `/v1/responses`

### Frontend components

- `pages/RequestList.tsx` — Arco Design Table with inline expandable rows; `expandedRowRender` content is wrapped in `stopPropagation` div to prevent row-toggle conflicts
- `components/RequestDetail.tsx` — inline expanded row: metadata `Descriptions`, request/response JSON viewers (`@microlink/react-json-view`, `collapsed={2}`), request headers. Clicking a message node in the JSON viewer opens a modal with syntax-highlighted plain-text rendering.
- `components/TokenStatsFooter.tsx` — sticky dark footer bar; polls `GET /api/stats` every 30 s

### REST API

| Endpoint | Description |
|----------|-------------|
| `GET /api/requests?page=N&limit=N` | Paginated list, sorted by timestamp desc |
| `GET /api/requests/:id` | Full request detail |
| `GET /api/stats` | Aggregated token sums across all requests |
