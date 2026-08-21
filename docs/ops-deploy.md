# 生产部署手册（TLS / 独立 Compose / 一键部署）

> 生产部署使用脚手架生成的 Kratos 后端；Casdoor 只作为外部 OIDC IdP，Velora 不实现 OIDC Provider。

## 1. 目录结构

```text
deployments/env/
├── dev/                      # 开发（即根 docker-compose.yml，无 overlay）
├── staging/
│   └── docker-compose.staging.yml   # 预发：独立端口 15173/18080/18443/15433
└── prod/
    ├── docker-compose.yml            # 独立生产编排，不合并开发 Compose
    ├── .env.example                  # 生产变量模板（真实值由 Secret Manager 注入）
    └── certs/                       # 运维放置 velora.crt / velora.key（不入 git）
```

## 2. 生产部署步骤（全新环境）

```bash
# 0) 准备环境变量（不入 git）：复制生产模板并填入 Secret Manager 注入的值
cp deployments/env/prod/.env.example /secure/velora-prod.env
#    必填：VELORA_EXTERNAL_URL / VELORA_DATABASE_DSN_FILE / CASDOOR_DATA_SOURCE_NAME
#          VELORA_OIDC_ISSUER / VELORA_OIDC_CLIENT_ID / VELORA_OIDC_REDIRECT_URL (https://<public-host>/auth/callback)
#          VELORA_CASDOOR_ACCOUNT_URL / Redis TLS / ObjectStore / CryptoProvider Secret 文件
#          VELORA_SESSION_TTL=1h / VELORA_OIDC_POST_LOGOUT_REDIRECT_URL (HTTPS)
#          PostgreSQL 首次初始化账号（POSTGRES_*）

# 1) 证书：将权威/内网 CA 签发的证书放入 deployments/env/prod/certs/
#    velora.crt / velora.key（含完整链；内网请下发根证书到终端）

# 2) 先执行静态配置门禁（不启动容器）
make check-prod-config

# 3) 先执行一次 release migration，再启动应用（不合并开发 Compose）
docker compose --env-file /secure/velora-prod.env -f deployments/env/prod/docker-compose.yml --profile release run --rm migrate
docker compose --env-file /secure/velora-prod.env \
  -f deployments/env/prod/docker-compose.yml up -d --build

# 4) Casdoor 应用/回调由 Casdoor 管理员按审批流程预配置；
#    生产启动不使用 Velora 的 Casdoor 管理员密码。

# 5) 校验
curl -s https://$VELORA_EXTERNAL_URL/healthz          # web 健康
curl -s https://$VELORA_EXTERNAL_URL/api/v1/system/version   # API（经 nginx 反代）
curl -sI https://$VELORA_EXTERNAL_URL/ | grep -i strict   # 启用 HSTS 后应出现
```

## 3. TLS 说明

- Web 容器 `VELORA_TLS_ENABLED=true` 时：80 → 301 → 443，443 挂载 `certs/velora.crt|key`。
- 入口拓扑：`浏览器 → 443 (nginx) → /api → server:8080`；Casdoor 作为配置的 OIDC issuer。
- Velora 不代理或改造 Casdoor 源码；OIDC discovery、授权和账号自助页地址由生产配置提供。
- 生产 OIDC session TTL 不得超过 1 小时。退出登录先撤销 Velora 服务端 session，再按 Casdoor discovery 的 `end_session_endpoint` 执行标准 RP-initiated logout；未提供该端点时仍完成本地撤销，但必须在 Casdoor 目标环境 E2E 中记录上游会话行为。
- HSTS：生产 HTTPS 模板默认开启 `max-age=63072000; includeSubDomains; preload`；证书必须由受控 Secret Manager/发布流程轮换，不能通过关闭 HSTS 或降级 HTTP 绕过门禁。
- 证书轮换：覆盖 certs/ 下同名文件 → `docker compose restart web`（nginx 自动重载）。

## 3.1 ForwardAuth 网关接入（老系统）

老系统只允许通过已登记的 `application_id` 访问 ForwardAuth：

```text
GET /api/v1/auth/forward/{application_id}
```

网关必须把浏览器的 Velora Session Cookie（或受控 Bearer Token）转发到该端点，并仅将后端响应中的以下头复制给上游：
`X-Velora-Authenticated`、`X-Velora-Application-ID`、`X-Velora-User-ID`、
`X-Velora-Login-Name`、`X-Velora-Organization-ID`。网关须在转发前删除所有入站
`X-Velora-*` 头；客户端提供的 app-id、Host 或身份头永远不可信。未知、禁用或未通过
`CanAccess` 的应用统一返回 401/403，不得回退到门户隐藏或仅校验“已登录”。
服务端还要求请求来自 `TRUSTED_PROXIES` 中的网关 CIDR，并要求网关设置单值
`X-Forwarded-Host`；该值必须与应用登记的 HTTPS `home_url`/`launch_url` 主机完全匹配。

生产上线验收必须覆盖：伪造 header、未知 app、禁用应用、撤销角色后旧会话四类直接绕过测试。

## 4. 密钥和生产配置强制校验（fail-fast）

独立生产 Compose 使用 `${VAR:?}` 语法，缺失即拒绝解析；后端启动还会校验 TLS、OIDC、ObjectStore 和 CryptoProvider：

```text
VELORA_OIDC_CLIENT_SECRET_FILE  必填，只读 Secret 文件
VELORA_CRYPTO_KEY_FILE          必填，只读 Secret 文件
VELORA_EXTERNAL_URL    必须是外部 HTTPS host
VELORA_DATABASE_DSN_FILE 必须指向只读 DSN 文件，使用独立 Velora 数据库账号且 sslmode=verify-full
CASDOOR_DATA_SOURCE_NAME  必须使用独立 Casdoor 数据库账号
Redis TLS 证书/密钥、`REDIS_PASSWORD_FILE` 必填；Redis 密码只通过只读 Secret 文件读取并写入 root-only 临时配置，禁止放入 Compose 环境变量或长期进程命令行，生产禁止内存降级。应用 DSN、对象存储访问密钥同样使用 `VELORA_DATABASE_DSN_FILE`、`VELORA_STORAGE_ACCESS_KEY_FILE`、`VELORA_STORAGE_SECRET_KEY_FILE`，由 Secret Manager 渲染为 root-only 文件。
PostgreSQL 初始化密码使用 `POSTGRES_SUPERUSER_PASSWORD_FILE`、`POSTGRES_APP_PASSWORD_FILE`、`POSTGRES_IDP_PASSWORD_FILE`；仅 Casdoor 官方 DSN 配置仍由 `CASDOOR_DATA_SOURCE_NAME` 注入。金融生产模板默认 `VELORA_CRYPTO_PROVIDER=gm` 与 `VELORA_CRYPTO_ADAPTER=hsm`；未接入经审批国密 HSM/PKCS#11 驱动时必须保持不可启动。

对象存储切换或发布前可执行只读检查：`cd server && VELORA_CONFIG=/path/to/approved.yaml VELORA_STORAGE_CHECK_REQUIRE=basic_object_io,checksum ./bin/velora-storage-check`。命令只执行 bucket ping 并输出 capability/evidence；`VELORA_STORAGE_CHECK_REQUIRE` 中的能力必须来自 `Target-tested` 合同，否则非零退出，不创建或删除对象。
TRUSTED_PROXIES        必须显式配置网关网段
```

静态门禁只解析最终配置，不启动容器：

```bash
make check-prod-config
```

## 5. 三环境矩阵

| 环境 | 命令 | 端口 | TLS | 用途 |
| --- | --- | --- | --- | --- |
| dev | `make docker-up` | 5173/8080/8443/5433 | 否 | 本地开发 |
| staging | `docker compose -f docker-compose.yml -f deployments/env/staging/docker-compose.staging.yml up -d` | 15173/18080/18443/15433 | 否 | 联调验收 |
| prod | `deployments/env/prod/docker-compose.yml` | 443/80 | 是 | 生产（独立编排，不合并开发 Compose） |

## 6. 与可观测性组合

```bash
# 生产 + 监控（监控服务不发布 host port，需经内部网络/受控网关访问）
docker compose --env-file /secure/velora-prod.env \
  -f deployments/env/prod/docker-compose.yml \
  --profile monitoring up -d
# Prometheus :9090 / Grafana :3000（仅内部网络，凭据必须由环境注入）
```

## 7. 回滚

```bash
# 代码回滚：切到上一 tag/commit 后，使用同一独立生产 Compose
docker compose --env-file /secure/velora-prod.env \
  -f deployments/env/prod/docker-compose.yml up -d --build server web
# 数据回滚：见 docs/ops-backup.md 恢复流程
```

Wave 1 回滚注意：独立 Compose 使用新的 `velora-prod-*` 数据卷，首次切换前必须完成现有环境的备份和恢复演练；不要删除旧卷。若新编排健康检查或数据库初始化失败，停止新服务并切回上一版本 Compose/镜像，保留新卷和日志供排查。
