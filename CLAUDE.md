# CLAUDE.md

## Language
- 使用中文回复
- Speak Chinese
- Commit Message写中文
This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

API Tracker is a self-hosted AI API proxy that intercepts requests to OpenAI/Anthropic/compatible endpoints, stores them in MongoDB, and provides a React frontend for browsing history with token analytics.

## Commands

### Backend (Go)

```bash
cd backend
go run ./cmd/server          # start dev server (reads config.yaml + config.local.yaml)
go build ./...               # verify compilation
go build -o api-tracker-server ./cmd/server   # build binary

# cross-compile for Docker (linux/amd64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api-tracker-server ./cmd/server
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
| `internal/storage` | MongoDB CRUD + `GetStats()` aggregation; `ListFilter` server-side filtering + `ensureIndexes()` |
| `internal/proxy` | Manual HTTP + SSE proxy; `UsageInfo` token extraction |
| `internal/api` | REST handlers: `ListRequests`, `GetRequest`, `GetStats`, `ListEndpoints` |

### Proxy routes (registered in `main.go`)

Gin registers `/v1/*path` as a single wildcard catch-all. All `/v1/*` traffic goes to the single `default_endpoint`; the endpoint's `type` field determines auth headers and SSE parsing (no per-path routing).

| Incoming path prefix | Notes |
|----------------------|-------|
| `/v1/*` | forwarded to `default_endpoint` (any path) |
| `/<name>/*` | named endpoint in `endpoints[]`; registered individually at startup |
| `/api/*` | REST API for the frontend |

Named endpoint routes (`/<name>/*`) are registered individually at startup (not via a generic `/:name` wildcard) to avoid conflicting with `/api/*` and `/v1/*`.

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
- Env vars: `APITRACKER_ENDPOINT_{NAME}_KEY`, `APITRACKER_ENDPOINT_{NAME}_URL`, `APITRACKER_MONGO_URI`, etc.
  - Name normalization: `openai-responses` → `OPENAI_RESPONSES`
- `default_endpoint` — single default; all `/v1/*` requests go here; **no per-path routing**
- `DefaultAnthropic` / `DefaultOpenAIResponses` fields **do not exist** — removed; type is determined by the endpoint's `type` field, not by path

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
| `GET /api/endpoints` | Configured endpoint names (`{name,type}`) — feeds the frontend filter dropdown |

#### `/api/requests` filter params (server-side, MongoDB)

All optional; omitted/empty params are ignored. Filtering happens in MongoDB via `storage.ListFilter`, not in the frontend.

| Param | Meaning |
|-------|---------|
| `provider` | exact endpoint name (matches stored `provider`) |
| `status_code` | exact HTTP status; **takes precedence** over `status_class` if both sent |
| `status_class` | bucket `2xx`/`3xx`/`4xx`/`5xx` → `status_code` range query |
| `start_time` / `end_time` | accepts **Unix milliseconds** or **RFC3339**; `timestamp` `$gte`/`$lte` |

### MongoDB indexes

`storage.ensureIndexes()` is called from `NewStore()` at backend startup, so docker compose first-deploy and upgrades both create/refresh indexes automatically — no manual `mongosh` needed. Indexes use **default (auto-generated) names** on purpose: redeclaring an existing index (e.g. the legacy `timestamp_-1`) becomes an idempotent no-op instead of an `IndexKeySpecsConflict` that would fail the whole `CreateMany` batch on upgrade.

Indexes (follow MongoDB ESR: Equality → Sort → Range):
- `{timestamp:-1}` — default sort / time-range only
- `{provider:1, timestamp:-1}` — endpoint filter + sort
- `{status_code:1, timestamp:-1}` — status filter (exact or range bucket) + sort
