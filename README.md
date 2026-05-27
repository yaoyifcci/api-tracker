# API Tracker

A self-hosted AI API proxy that intercepts, records, and displays requests to OpenAI / Anthropic / any compatible endpoint.

[中文文档](README.zh.md)

## Features

- Transparent proxy for OpenAI Chat Completions, Anthropic Messages, OpenAI Responses API
- Full SSE streaming support with real-time token tracking
- MongoDB storage — full request/response history with pagination
- React frontend with inline expandable rows, JSON viewer, and Markdown content preview
- Token statistics footer (input / completion / cache read / cache write)
- Three-layer config: `config.yaml` → `config.local.yaml` → environment variables; API keys never committed

## Quick Start — Local

**Prerequisites:** Go 1.26+, Node 20+, MongoDB on localhost:27017

```bash
# 1. Add your API keys
cp backend/config.local.yaml.example backend/config.local.yaml
# Edit backend/config.local.yaml

# 2. Start the backend
cd backend && go run ./cmd/server

# 3. Start the frontend (new terminal)
cd frontend && npm install && npm run dev

# 4. Open http://localhost:5173
```

## Quick Start — Docker

```bash
# 1. Set API keys
cp .env.example .env
# Edit .env

# 2. Start everything
make up

# 3. Open http://localhost:3000
```

## Configuration

### config.yaml (safe to commit)

Define your endpoints with URL and type. **No keys here.**

```yaml
proxy_port: 8080
mongodb_uri: mongodb://localhost:27017
mongodb_db: api-tracker
default_endpoint: openai
default_anthropic_endpoint: anthropic
endpoints:
  - name: openai
    url: https://api.openai.com
    type: openai
  - name: openai-responses
    url: https://api.openai.com
    type: openai_responses
  - name: anthropic
    url: https://api.anthropic.com
    type: anthropic
```

### config.local.yaml (gitignored)

Overlay keys for local development:

```yaml
endpoints:
  - name: openai
    key: sk-...
  - name: anthropic
    key: sk-ant-...
```

### Endpoint types

| Type | Auth header | Use case |
|------|-------------|----------|
| `openai` | `Authorization: Bearer` | OpenAI Chat Completions and any compatible API |
| `anthropic` | `x-api-key` | Anthropic Messages API |
| `openai_responses` | `Authorization: Bearer` | OpenAI Responses API |

### Three-layer key injection

Priority (highest → lowest):

1. **Environment variables** — `APITRACKER_ENDPOINT_{NAME}_KEY`
2. **`backend/config.local.yaml`** — gitignored, local development
3. **`backend/config.yaml`** — committed, no secrets

### Environment variables

| Variable | Description |
|----------|-------------|
| `APITRACKER_PROXY_PORT` | Proxy listen port (default: `8080`) |
| `APITRACKER_MONGO_URI` | MongoDB connection string |
| `APITRACKER_MONGO_DB` | MongoDB database name |
| `APITRACKER_DEFAULT_ENDPOINT` | Default endpoint name for `/v1/chat/completions` |
| `APITRACKER_DEFAULT_ANTHROPIC_ENDPOINT` | Default endpoint name for `/v1/messages` |
| `APITRACKER_ENDPOINT_{NAME}_KEY` | API key for a named endpoint |
| `APITRACKER_ENDPOINT_{NAME}_URL` | URL override for a named endpoint |

Name normalization: `openai-responses` → `OPENAI_RESPONSES`.

## Proxy Routes

| Path | Routed to |
|------|-----------|
| `/v1/chat/completions[/*]` | `default_endpoint` |
| `/v1/messages[/*]` | `default_anthropic_endpoint` |
| `/v1/responses[/*]` | `default_openai_responses_endpoint` |
| `/{name}/*` | Named endpoint |
| anything else | `404 {"error":"unknown endpoint"}` |

The client does not need to send an `Authorization` header — the proxy injects the configured key automatically.

## Makefile

```bash
make build   # cross-compile backend for linux/amd64
make up      # docker compose up --build -d
make down    # docker compose down
make dev     # start backend + frontend locally
```

## Tech Stack

- **Backend:** Go 1.26, Gin, MongoDB driver v2
- **Frontend:** React 18, Vite, Arco Design, TypeScript
- **Storage:** MongoDB
- **Deploy:** Docker Compose
