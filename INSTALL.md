# INSTALL.md — AI Tracker 部署指南

> 本文档面向 Claude Code（或人工操作者）在新 Linux 服务器上通过 Docker Compose 完成从零部署。
> 按顺序执行每个步骤，每步结束后验证结果再继续。

---

## 前置条件

| 依赖 | 最低版本 | 检查命令 |
|------|---------|---------|
| Docker Engine | 24+ | `docker --version` |
| Docker Compose plugin | v2 | `docker compose version` |
| Git | 任意 | `git --version` |

如果 Docker 未安装，执行：

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # 让当前用户免 sudo 运行 docker
newgrp docker                   # 立即生效（或重新登录）
```

---

## 第一步：克隆仓库

```bash
git clone <REPO_URL> aitrace
cd aitrace
```

---

## 第二步：创建 `.env` 文件（API 密钥）

`.env` 文件不提交到 git，需要手动创建。

```bash
cp .env.example .env
```

编辑 `.env`，填写你的 API 密钥和默认 endpoint：

```bash
# 必填：指定默认 endpoint 名称（必须与 config.yaml 中的 endpoints[].name 之一匹配）
APITRACKER_DEFAULT_ENDPOINT=anthropic

# 按需填写各 endpoint 的 API key（名称大写，连字符转下划线）
APITRACKER_ENDPOINT_ANTHROPIC_KEY=sk-ant-...
APITRACKER_ENDPOINT_OPENAI_KEY=sk-...
APITRACKER_ENDPOINT_OPENAI_RESPONSES_KEY=sk-...
```

> `.env` 中的变量优先级最高，会覆盖 `config.yaml` 中的同名配置。

---

## 第三步：配置 endpoints（`backend/config.yaml`）

`backend/config.yaml` 已提交到 git，包含 endpoint 定义。默认内容：

```yaml
proxy_port: 8080
mongodb_uri: mongodb://localhost:27017
mongodb_db: api-tracker
default_endpoint: anthropic
endpoints: []
```

**`endpoints` 为空时**，代理使用 `default_endpoint` 指向的内置预设（见下表）。  
如需添加自定义 endpoint（如第三方兼容服务），在 `endpoints` 数组里追加：

```yaml
endpoints:
  - name: anthropic
    url: https://api.anthropic.com
    type: anthropic            # openai | anthropic | openai_responses
    # key 留空，由 .env 的 APITRACKER_ENDPOINT_ANTHROPIC_KEY 注入

  - name: routeany
    url: https://routeany.com
    type: openai
    # key 同理
```

### Endpoint type 说明

| type | 认证 header | 适用接口 |
|------|------------|---------|
| `openai`（默认） | `Authorization: Bearer <key>` | OpenAI Chat Completions 及兼容接口 |
| `anthropic` | `x-api-key: <key>` + `anthropic-version: 2023-06-01` | Anthropic Messages API |
| `openai_responses` | `Authorization: Bearer <key>` | OpenAI Responses API |

### URL 填写规则

**不带 `/v1` 后缀**。代理会把客户端请求的完整路径（如 `/v1/messages`）直接拼接到 URL 后。

```
# 正确
url: https://api.anthropic.com

# 错误（会导致 /v1/v1/messages 双重路径）
url: https://api.anthropic.com/v1
```

---

## 第四步：启动服务

```bash
make up
# 等价于: docker compose up --build -d
```

验证三个服务都已启动：

```bash
docker compose ps
# 预期输出：mongodb、backend、frontend 均为 running/healthy
```

查看后端日志确认启动成功：

```bash
docker compose logs backend
# 预期包含: "connected to MongoDB" 和 "api-tracker proxy listening on :8080"
```

---

## 第五步：验证

| 服务 | 地址 | 说明 |
|------|------|------|
| 前端 UI | `http://<HOST>:3000` | 请求历史浏览界面 |
| 代理入口 | `http://<HOST>:8080` | 客户端将 AI API base_url 指向这里 |
| MongoDB | `localhost:27017` | 仅容器内部访问，不对外暴露建议 |

发一条测试请求（以 Anthropic 为例）：

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: $ANTHROPIC_KEY" \
  -d '{"model":"claude-haiku-4-5-20251001","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}'
```

预期返回 HTTP 200 并在 `http://<HOST>:3000` 的列表中出现一条新记录。

---

## 客户端接入方式

将 AI 客户端（如 Hermes Agent、OpenAI SDK、Anthropic SDK）的 `base_url` 改为代理地址：

```
# Anthropic 客户端
base_url = http://<HOST>:8080

# OpenAI 兼容客户端
base_url = http://<HOST>:8080/v1
```

代理透明转发，不需要修改客户端的其他配置（API key 由客户端照常携带，代理优先使用客户端 key，不存在时使用配置文件中的 key）。

也可以通过命名 endpoint 路径发送请求，绕过默认 endpoint 路由：

```
http://<HOST>:8080/<endpoint-name>/v1/messages
```

---

## 升级

```bash
git pull
make up          # 自动重新构建镜像并重启，MongoDB 数据不丢失
```

---

## 常见问题

### 端口已被占用

编辑 `docker-compose.yml`，修改 `ports` 映射：

```yaml
ports:
  - "3001:80"   # 前端改为 3001
  - "8081:8080" # 后端改为 8081
```

如果修改了后端端口，需同步更新客户端 `base_url`。

### 容器内无法访问宿主机的代理（如 Clash/Surge）

在同目录下创建 `docker-compose.local.yaml`（已 gitignore，不会提交）：

```yaml
name: api-tracker
services:
  backend:
    build:
      args:
        HTTPS_PROXY: "http://host.docker.internal:7897"
        HTTP_PROXY: "http://host.docker.internal:7897"
  frontend:
    build:
      args:
        HTTPS_PROXY: "http://host.docker.internal:7897"
        HTTP_PROXY: "http://host.docker.internal:7897"
```

启动时叠加该文件：

```bash
docker compose -f docker-compose.yml -f docker-compose.local.yaml up --build -d
```

### `/v1/models` 等探测接口返回 400

代理已自动为 Anthropic 类型 endpoint 注入 `anthropic-version: 2023-06-01` header，通常不需要手动处理。如仍报错，检查 `.env` 中的 key 是否正确。

### 查看实时日志

```bash
docker compose logs -f backend    # 后端
docker compose logs -f frontend   # 前端 nginx
docker compose logs -f mongodb    # 数据库
```

### 完全重置（清除数据）

```bash
make down
docker volume rm api-tracker_mongo_data
make up
```

---

## 目录结构参考

```
aitrace/
├── backend/
│   ├── config.yaml          # endpoint 定义，提交到 git
│   ├── config.local.yaml    # 本地密钥覆盖，gitignored（本地开发用）
│   └── Dockerfile
├── frontend/
│   ├── Dockerfile
│   └── nginx.conf           # 将 /api/* 反代到 backend:8080
├── docker-compose.yml       # 主 compose 文件
├── docker-compose.local.yaml # 本地代理覆盖，gitignored
├── .env.example             # 密钥模板，复制为 .env 后填写
├── .env                     # 实际密钥，gitignored
└── Makefile                 # make up / make down / make dev
```
