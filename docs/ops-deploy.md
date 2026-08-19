# 生产部署手册（TLS / 环境 overlay / 一键部署）

> Phase A1/A4 配套文档。目标：生产环境全链路 HTTPS、Casdoor 隐藏在 Velora 域名下、
> 密钥不落盘、一键可复现部署。

## 1. 目录结构

```text
deployments/env/
├── dev/                      # 开发（即根 docker-compose.yml，无 overlay）
├── staging/
│   └── docker-compose.staging.yml   # 预发：独立端口 15173/18080/18443/15433
└── prod/
    ├── docker-compose.prod.yml      # 生产：TLS + 密钥强制 + 资源限制
    └── certs/                       # 运维放置 velora.crt / velora.key（不入 git）
```

## 2. 生产部署步骤（全新环境）

```bash
# 0) 准备环境变量（不入 git）：复制 .env.example 并填生产值
cp .env.example .env
#    必填：SESSION_SECRET / CASDOOR_CLIENT_SECRET / CASDOOR_ADMIN_PASSWORD
#          MAIL_CREDENTIAL_KEY（openssl rand -base64 32）
#          VELORA_EXTERNAL_URL（如 velora.example.com）

# 1) 证书：将权威/内网 CA 签发的证书放入 deployments/env/prod/certs/
#    velora.crt / velora.key（含完整链；内网请下发根证书到终端）

# 2) 启动（compose 合并生产 overlay）
DOCKER_REGISTRY=docker.m.daocloud.io \
docker compose -f docker-compose.yml -f deployments/env/prod/docker-compose.prod.yml up -d --build

# 3) 首次初始化 Casdoor OIDC 应用（脚本会自动连容器内 PG）
./scripts/init-casdoor.sh        # 输出 client_id / client_secret → 填入 .env → 重启 server

# 4) 校验
curl -s https://$VELORA_EXTERNAL_URL/healthz          # web 健康
curl -s https://$VELORA_EXTERNAL_URL/api/v1/system/version   # API（经 nginx 反代）
curl -sI https://$VELORA_EXTERNAL_URL/ | grep -i strict   # 启用 HSTS 后应出现
```

## 3. TLS 说明

- Web 容器 `VELORA_TLS_ENABLED=true` 时：80 → 301 → 443，443 挂载 `certs/velora.crt|key`。
- 入口拓扑：`浏览器 → 443 (nginx) → /api → server:8080`；`/casdoor/ → casdoor:8000`。
- Casdoor 侧 `origin=https://$URL/casdoor`：**用户永远看不到 Casdoor 真实地址**，登录页
  位于 Velora 域名下，符合"Casdoor 藏在后面"目标。
- HSTS：`velora-https.conf.template` 中注释，确认证书链可信后取消注释并重载。
- 证书轮换：覆盖 certs/ 下同名文件 → `docker compose restart web`（nginx 自动重载）。

## 4. 密钥强制校验（fail-fast）

生产 overlay 使用 `${VAR:?}` 语法，缺失即拒绝启动：

```text
SESSION_SECRET         必填（≥32 字节随机）
CASDOOR_CLIENT_SECRET  必填
CASDOOR_ADMIN_PASSWORD 必填（仅服务端同步用）
MAIL_CREDENTIAL_KEY    必填（base64 32 字节，独立于 SESSION_SECRET）
```

## 5. 三环境矩阵

| 环境 | 命令 | 端口 | TLS | 用途 |
| --- | --- | --- | --- | --- |
| dev | `make docker-up` | 5173/8080/8443/5433 | 否 | 本地开发 |
| staging | `docker compose -f docker-compose.yml -f deployments/env/staging/docker-compose.staging.yml up -d` | 15173/18080/18443/15433 | 否 | 联调验收 |
| prod | 见上文 §2 | 443/80 | 是 | 生产 |

## 6. 与可观测性组合

```bash
# 生产 + 监控
docker compose -f docker-compose.yml -f deployments/env/prod/docker-compose.prod.yml \
  --profile monitoring up -d
# Prometheus :9090 / Grafana :3000（默认 admin/admin，首次登录改密）
```

## 7. 回滚

```bash
# 代码回滚：切到上一 tag/commit 后
docker compose -f docker-compose.yml -f deployments/env/prod/docker-compose.prod.yml up -d --build server web
# 数据回滚：见 docs/ops-backup.md 恢复流程
```
