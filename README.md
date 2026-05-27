# API Tracker

A self-hosted AI API proxy that intercepts, records, and displays requests to OpenAI / Anthropic / any compatible endpoint.

## Features

- Transparent proxy for OpenAI Chat Completions, Anthropic Messages, OpenAI Responses API
- Full SSE streaming support with token tracking
- MongoDB storage — full request/response history
- React frontend with inline expandable details, message content preview, Markdown rendering
- Token statistics footer (input / completion / cache read / cache write)
- Config desensitization: keys stay out of `config.yaml` and source control

## Quick Start — Local

**Prerequisites:** Go 1.26+, Node 20+, MongoDB running on localhost:27017

```bash
# 1. Configure your API keys
cp backend/config.local.yaml.example backend/config.local.yaml
# Edit backend/config.local.yaml — add your keys

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
# Edit .env — add your keys

# 2. Start everything
make up

# 3. Open http://localhost:3000
```

## Configuration

### config.yaml (safe to commit)

Defines endpoints with URL and type. **No keys here.**

```yaml
proxy_port: 8080
mongodb_uri: mongodb://localhost:27017
mongodb_db: api-tracker
endpoints:
  - name: openai
    url: https://api.openai.com
    type: openai
  - name: anthropic
    url: https://api.anthropic.com
    type: anthropic
```

### Endpoint types

| Type | Description |
|------|-------------|
| `openai` | OpenAI Chat Completions (`/v1/chat/completions`) and compatible APIs |
| `anthropic` | Anthropic Messages API — uses `x-api-key` header, named SSE events |
| `openai_responses` | OpenAI Responses API (`/v1/responses`) |

### Key injection (three-layer priority)

Priority (highest → lowest):

1. **Environment variables** — `APITRACKER_ENDPOINT_{NAME}_KEY` (e.g. `APITRACKER_ENDPOINT_OPENAI_KEY`)
2. **`backend/config.local.yaml`** — gitignored, for local development
3. **`backend/config.yaml`** — base config, no secrets

### Environment variables

| Variable | Description |
|----------|-------------|
| `APITRACKER_PROXY_PORT` | Proxy listen port (default: 8080) |
| `APITRACKER_MONGO_URI` | MongoDB connection string |
| `APITRACKER_MONGO_DB` | MongoDB database name |
| `APITRACKER_ENDPOINT_{NAME}_KEY` | API key for named endpoint |
| `APITRACKER_ENDPOINT_{NAME}_URL` | Override URL for named endpoint |

Name normalization: `openai-responses` → `OPENAI_RESPONSES`.

## Proxy Routes

| Route | Behavior |
|-------|----------|
| `/v1/chat/completions` | Forward to `default_endpoint` |
| `/v1/messages` | Forward to `default_anthropic_endpoint` |
| `/v1/responses` | Forward to `default_openai_responses_endpoint` |
| `/{name}/*` | Forward to the named endpoint |

The client does not need to send an Authorization header — the proxy injects the configured key.

## Makefile

```bash
make build   # cross-compile backend for linux/amd64
make up      # docker compose up --build -d
make down    # docker compose down
make dev     # start backend + frontend locally
```
