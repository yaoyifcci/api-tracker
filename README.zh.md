# API Tracker

自托管的 AI API 代理工具，拦截并记录发往 OpenAI / Anthropic / 任意兼容接口的请求，通过 React 前端可视化浏览历史与 Token 消耗。

**GitHub 项目描述：** 自托管 AI API 代理与请求追踪工具，支持 OpenAI、Anthropic 及兼容接口。

[English](README.md)

## 功能特性

- 透明代理 OpenAI Chat Completions、Anthropic Messages、OpenAI Responses API
- 完整 SSE 流式支持，实时 Token 计数
- MongoDB 持久化存储，分页浏览完整请求/响应历史
- React 前端：行内展开详情、JSON 查看器、Markdown 内容预览
- 底部 Token 汇总栏（输入 / 补全 / 缓存读 / 缓存写）
- 三层配置合并：`config.yaml` → `config.local.yaml` → 环境变量，API Key 永不提交 git

## 快速开始 — 本地运行

**前置依赖：** Go 1.26+、Node 20+、本地 MongoDB（端口 27017）

```bash
# 1. 配置 API Key
cp backend/config.local.yaml.example backend/config.local.yaml
# 编辑 backend/config.local.yaml，填入你的 Key

# 2. 启动后端
cd backend && go run ./cmd/server

# 3. 启动前端（新终端）
cd frontend && npm install && npm run dev

# 4. 打开 http://localhost:5173
```

## 快速开始 — Docker

```bash
# 1. 配置 API Key
cp .env.example .env
# 编辑 .env，填入你的 Key

# 2. 一键启动
make up

# 3. 打开 http://localhost:3000
```

## 配置说明

### config.yaml（可提交 git）

定义 endpoint 的 URL 和类型，**不填写 Key**。

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

### config.local.yaml（已 gitignore）

本地开发时在此覆盖 Key：

```yaml
endpoints:
  - name: openai
    key: sk-...
  - name: anthropic
    key: sk-ant-...
```

### Endpoint 类型

| 类型 | 鉴权头 | 适用场景 |
|------|--------|----------|
| `openai` | `Authorization: Bearer` | OpenAI Chat Completions 及任意兼容接口 |
| `anthropic` | `x-api-key` | Anthropic Messages API |
| `openai_responses` | `Authorization: Bearer` | OpenAI Responses API |

### 三层 Key 注入（优先级从高到低）

1. **环境变量** — `APITRACKER_ENDPOINT_{NAME}_KEY`
2. **`backend/config.local.yaml`** — 已 gitignore，本地开发专用
3. **`backend/config.yaml`** — 提交 git，不含任何 Key

### 环境变量列表

| 变量名 | 说明 |
|--------|------|
| `APITRACKER_PROXY_PORT` | 代理监听端口（默认 `8080`） |
| `APITRACKER_MONGO_URI` | MongoDB 连接字符串 |
| `APITRACKER_MONGO_DB` | MongoDB 数据库名 |
| `APITRACKER_DEFAULT_ENDPOINT` | `/v1/chat/completions` 默认 endpoint 名 |
| `APITRACKER_DEFAULT_ANTHROPIC_ENDPOINT` | `/v1/messages` 默认 endpoint 名 |
| `APITRACKER_ENDPOINT_{NAME}_KEY` | 指定 endpoint 的 API Key |
| `APITRACKER_ENDPOINT_{NAME}_URL` | 覆盖指定 endpoint 的 URL |

名称规范化示例：`openai-responses` → `OPENAI_RESPONSES`。

## 代理路由

| 路径 | 转发目标 |
|------|----------|
| `/v1/chat/completions[/*]` | `default_endpoint` |
| `/v1/messages[/*]` | `default_anthropic_endpoint` |
| `/v1/responses[/*]` | `default_openai_responses_endpoint` |
| `/{name}/*` | 指定名称的 endpoint |
| 其他路径 | `404 {"error":"unknown endpoint"}` |

客户端无需携带 `Authorization` 头，代理会自动注入配置的 Key。

## Makefile 命令

```bash
make build   # 交叉编译后端（linux/amd64）
make up      # docker compose up --build -d
make down    # docker compose down
make dev     # 本地同时启动后端和前端
```

## 技术栈

- **后端：** Go 1.26、Gin、MongoDB driver v2
- **前端：** React 18、Vite、Arco Design、TypeScript
- **存储：** MongoDB
- **部署：** Docker Compose

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。
