# Velora

> A modern enterprise application portal and unified workspace.

**Velora 企业应用门户 / 企业统一工作台** —— 企业内部系统多、入口散、重复登录、找不到应用？Velora 提供：**一个门户 → 一次登录 → 看到全部有权访问的应用 → 搜索/分类/收藏/最近使用 → 点击直达**。

> **Casdoor manages identity. Velora manages the workspace.**
>
> Casdoor 负责身份 / IAM / SSO，Velora 负责门户 / 工作台 / 应用枢纽。两者通过 OIDC / OAuth 2.0 / Casdoor API 集成，**数据严格隔离**。

---

## 核心能力（Phase 1 MVP）

| 用户侧 | 管理侧 |
| --- | --- |
| Casdoor OIDC 统一登录（Authorization Code + PKCE） | 应用 CRUD（名称 / 图标 / 分类 / 标签 / 地址 / 接入类型 / 状态 / 排序） |
| 首页：欢迎 + 全局搜索 + 最近使用 + 精选应用 | 分类 CRUD |
| 应用中心：关键词搜索 / 分类筛选 / 标签筛选 / 分页 | 标签 CRUD |
| 应用详情与 Launch（服务端生成受信启动地址） | 应用访问策略（Everyone / 组织 / 角色 / 用户组 / 用户） |
| 我的收藏（落库持久化，刷新不丢失） | 审计日志（登录 / 应用操作 / 启动 / 收藏 / 策略变更） |
| 最近使用（按 last_visited_at 排序） | 门户概览统计与门户设置 |
| 管理员专属后台（同一套品牌语言 + 左侧导航） | 基础 HTTP 健康检查（UP / DOWN / UNKNOWN） |

Screenshots：待补充。

---

## Architecture

```text
                         User
                           │
                           ▼
                  ┌────────────────┐
                  │     Velora     │
                  │ Enterprise     │
                  │ Application Hub│
                  └────────┬───────┘
                           │
                    OIDC / OAuth2
                           │
                           ▼
                  ┌────────────────┐
                  │    Casdoor     │
                  │   IAM / SSO    │
                  └────────┬───────┘
                           │
          ┌────────────────┼─────────────────┐
          │                │                 │
         OIDC             SAML              CAS
          │                │                 │
          ▼                ▼                 ▼
       App A            App B             App C
```

- **后端**：Go + Gin + GORM + PostgreSQL，Modular Monolith，按业务域组织（auth / application / category / tag / favorite / visit / permission / audit / portal）。
- **前端**：React + TypeScript + Vite + Ant Design 6 + TanStack Query，复用 [Spectra Web](https://github.com/sevoniva-labs/spectra) 的 UI Foundation（Design Token / 布局 / 顶栏 / 请求封装 / 工程配置）。
- **身份**：Casdoor 独立部署，Velora 仅通过 OIDC 消费身份，**永不直连 Casdoor 数据表**。

### 技术栈

| 层 | 技术 |
| --- | --- |
| 前端 | React 19 · TypeScript · Vite 8 · Ant Design 6 · @ant-design/pro-components · TanStack Query 5 · React Router 7 · dayjs · pnpm |
| 后端 | Go 1.25 · Gin · GORM · PostgreSQL 16 · go-oidc v3 · oauth2 |
| 基础设施 | Docker Compose（PostgreSQL / Casdoor / Server / Web；Redis 可选 profile） |

---

## Why Velora

- **系统入口统一**：企业所有数字化应用一个门户直达。
- **认证统一**：不重复建设 IAM，Casdoor 专职身份，Velora 专职工作台。
- **权限前置**：前端隐藏 + 后端强制校验，直接调用 Launch API 也返回 403。
- **安全默认**：OIDC State / Nonce / PKCE、HttpOnly + SameSite Cookie、CSRF 双提交、Open Redirect 防护、输入校验、审计日志。
- **克制实现**：Phase 1 只做真实可用的能力，不为“技术栈完整”引入微服务 / Kafka / ES。

---

## Quick Start

前置：Go 1.25+、Node 22+、pnpm、Docker（compose v2）。

```bash
git clone https://github.com/sevoniva-labs/velora.git
cd velora

cp .env.example .env        # 按需修改（至少替换 SESSION_SECRET）
make init                   # 安装前后端依赖
```

### 方式一：Docker Compose（推荐演示）

```bash
make docker-up
# PostgreSQL :5432 / Casdoor :8443 / Velora Server :8080 / Velora Web :5173
```

Web 打开 <http://localhost:5173>，Casdoor 控制台 <http://localhost:8443>。

### 方式二：本地开发

```bash
make dev-server             # Go API :8080（自动迁移）
make dev-web                # Vite :5173（/api 代理到 :8080）
```

需要本机 PostgreSQL：创建 `velora` database 并设置 `.env` 的 `DATABASE_URL`。

---

## 中国大陆开发环境

```bash
./scripts/bootstrap-cn.sh            # 查看配置建议
./scripts/bootstrap-cn.sh --apply    # 当前终端临时应用（不修改全局配置）
```

- **Go Modules**：`GOPROXY=https://goproxy.cn,direct`（脚本提供，可随时切回官方源）。
- **pnpm**：`web/.npmrc` 项目级配置 `registry=https://registry.npmmirror.com`；切回官方只需改回 `https://registry.npmjs.org/`。
- **Docker**：构建镜像支持 `DOCKER_REGISTRY=docker.m.daocloud.io`（默认）↔ `docker.io` 切换，见 `docker-compose.yml` 与 `deployments/docker/Dockerfile.*`。
- 所有镜像锁定稳定版本，不使用 `latest`。

---

## Casdoor Integration

Casdoor 与 Velora 共用 PostgreSQL Server 但使用**独立 database**（`casdoor` / `velora`），由 `deployments/compose/init-db.sql` 初始化。Velora 永远通过 OIDC / API 获取身份信息。

### OIDC Configuration（一次性，2 分钟）

> Velora 只消费 Casdoor 的身份，**不会**自动在你的 Casdoor 里创建应用——身份体系归 Casdoor 管。
> 下表把 `.env` / `docker-compose.yml` 中的默认值对应到 Casdoor UI 操作：

1. 启动后打开 Casdoor 控制台 <http://localhost:8443>（默认账号 `admin` / `123`）。
2. 打开 **应用 → 添加应用**，填写：
   - 名称：`velora`（对应 `CASDOOR_CLIENT_ID` / compose 环境变量 `CASDOOR_CLIENT_ID=velora`）；
   - 回调地址（Redirect URI）：`http://localhost:8080/api/v1/auth/oidc/callback`（对应 `CASDOOR_REDIRECT_URI`）；
   - 授权类型勾选 **Authorization Code**，并在高级选项中启用 **PKCE**；
   - 保存后复制该应用的 **Client ID / Client Secret** 到 `.env`（compose 环境变量 `CASDOOR_CLIENT_SECRET`）。
3. 用户授权：在 **用户** 中给门户管理员添加角色 `velora_admin`（对应 `VELORA_ADMIN_ROLE`）；普通用户登录后自动拥有门户访问权。
4. 打开 Velora Web <http://localhost:5173> → 未登录自动跳 Casdoor → 认证 → 回到门户。

> 提示：`make docker-up` 启动的 Casdoor 使用 `initData=true`，已内置 `admin` 账号与 `built-in` 示例应用；为 Velora 新建应用即可，无需改动内置数据。

### 应用接入类型

| 类型 | 说明 | Phase 1 |
| --- | --- | --- |
| `URL` | 直链跳转（受信配置的地址） | ✅ |
| `OIDC` | 通过 Casdoor 为该应用签发登录跳转 | ✅ |
| `SAML` / `CAS` / `FORWARD_AUTH` | 数据模型与 Provider 扩展点已预留 | 后续 |

Launch 一律通过 `POST /api/v1/applications/{id}/launch`：服务端读取数据库 → 权限校验 → 状态校验 → 根据受信配置生成启动地址，**不接受客户端 URL 参数**（防 Open Redirect）。

---

## Docker Compose

```bash
make docker-up    # 启动全部
make docker-down  # 停止
# 可选 Redis：docker compose --profile redis up -d
```

服务列表：

| 服务 | 端口 | 说明 |
| --- | --- | --- |
| postgres | 5433 | PostgreSQL 16，初始化 velora / casdoor 两个独立 database（host 侧 5433，避免与本机 PG 冲突） |
| redis（可选） | 6379 | Phase 1 未强制使用 |
| casdoor | 8443 | Casdoor v1.762，身份 / IAM |
| server | 8080 | Velora Go API |
| web | 5173 | Velora Web（Nginx 托管 + /api 反代） |

---

## Directory Structure

```text
velora/
├── web/                     # React 前端（复用 Spectra UI Foundation）
│   ├── src/
│   │   ├── theme/           # Design Token（继承 Spectra 技术蓝体系）
│   │   ├── api/             # 请求封装 + 领域 API
│   │   ├── auth/            # RequireAuth / useMe
│   │   ├── layout/          # PortalLayout（门户）/ AdminLayout（后台）
│   │   ├── components/      # AppCard 等公共组件
│   │   ├── pages/           # 登录 / 首页 / 应用中心 / 详情 / 收藏 / admin/*
│   │   └── utils/           # 纯函数工具（含单测）
│   └── vite.config.ts       # 开发代理 / 产物拆分 / vitest
├── server/                  # Go 后端（Modular Monolith）
│   ├── cmd/velora/          # serve / migrate / seed
│   ├── internal/
│   │   ├── auth/            # Casdoor OIDC + Session
│   │   ├── application/     # 应用域（含 LaunchProvider / 健康检查）
│   │   ├── category/ tag/ favorite/ visit/
│   │   ├── permission/      # 管理员中间件
│   │   ├── audit/ portal/   # 审计 / 门户设置与统计
│   │   └── platform/        # config / db / errs / response / httpserver
│   ├── migrations/          # 幂等 SQL 迁移（embed）
│   └── tests/               # 单元测试
├── deployments/
│   ├── docker/              # Dockerfile.server / Dockerfile.web / nginx.conf
│   └── compose/             # init-db.sql
├── scripts/                 # bootstrap-cn.sh
├── docs/
├── docker-compose.yml
├── Makefile
├── .env.example
└── LICENSE                  # Apache-2.0
```

---

## Configuration

见 `.env.example`（不提交真实凭据）：

| 变量 | 说明 |
| --- | --- |
| `VELORA_PORT` | Server 端口，默认 8080 |
| `DATABASE_URL` | Velora 专属 PostgreSQL 连接串 |
| `CASDOOR_ISSUER` | Casdoor 地址（如 `http://localhost:8443`） |
| `CASDOOR_CLIENT_ID` / `CASDOOR_CLIENT_SECRET` | Casdoor 中 Velora 应用凭据 |
| `CASDOOR_REDIRECT_URI` | OIDC 回调地址 |
| `SESSION_SECRET` | 会话签名密钥（**必须**替换，`openssl rand -hex 32`） |
| `COOKIE_SECURE` | 生产 HTTPS 环境设为 `true` |
| `VELORA_ADMIN_ROLE` | Velora 管理员角色名，默认 `velora_admin` |
| `CORS_ALLOWED_ORIGINS` | 前后端分离开发时允许的来源（逗号分隔） |

---

## Development Guide

```bash
make dev          # 同时启动前后端
make test         # go test ./... + pnpm lint/test/build
make migrate      # 执行数据库迁移
make seed         # 写入开发 Seed（8 分类 + 8 示例应用，全部 EVERYONE 可见）
make build        # 构建 server 二进制 + web 产物
```

- 每个开发阶段持续执行：`go fmt` / `go vet` / `go test ./...` / `pnpm lint` / `pnpm test` / `pnpm build`。
- 前端工程体系（Vite 配置 / tsconfig / oxlint / Design Token / 布局样式）继承自 Spectra Web，请保持同一设计语言，不要在 Velora 再造一套视觉体系。
- **Spectra 目录保持只读**：Velora 只复制通用模块，不修改 Spectra，不运行时依赖其路径。

---

## Security

- OIDC **Authorization Code + PKCE**，State（HMAC 签名 + 过期）与 Nonce 双重校验。
- 会话 Cookie：**HttpOnly + Secure + SameSite**，HMAC 签名防篡改，过期时间可配。
- **CSRF 双提交**：写请求必须携带与 `velora_csrf` Cookie 一致的 `X-CSRF-Token`。
- **Open Redirect 防护**：OIDC 回调落点仅允许站内相对路径；Launch 不接受客户端 URL。
- **权限强制**：应用列表按访问策略过滤（前端隐藏），Launch / 详情接口后端再次校验（403）。
- 输入校验：URL 仅允许 http/https 且含主机名；编码唯一性；枚举白名单。
- 统一响应结构，不向前端返回 SQL 错误 / 堆栈 / 内部路径 / 密钥。
- 审计日志覆盖登录、登出、应用增删改、启动、收藏、策略变更。

---

## Roadmap

- **Phase 1（当前）**：Application Portal · Casdoor SSO · Application Center · Favorites · Recent Apps · Application Permission · Admin
- **Phase 2**：SAML · CAS · Forward Auth（Nginx/APISIX）· Application Access Request · Notifications · Unified Search
- **Phase 3**：Approval · Tasks · Service Catalog · AI Search · AI Assistant · Enterprise Integrations

---

## License

[Apache License 2.0](./LICENSE)

Velora 依赖均优先选择 Apache-2.0 / MIT / BSD / ISC 协议；不使用 GPL / AGPL / SSPL / BUSL / Commons Clause / Non-Commercial 等影响商业化的协议。
