# Velora 本地生产能力验收

本目录下的脚本用于在本地复现生产关键路径，输出的是“本地目标已验证”证据，不等同于云厂商、等保或国密认证。

## Casdoor OIDC

先执行数据库迁移，并在 Velora 中通过正式审批流程建立 Casdoor external identity 绑定；脚本不会创建或修改 Casdoor 用户、应用，也不会绕过审批：

```bash
cd server
VELORA_CONFIG=configs/minimal.yaml \
VELORA_DATABASE_DSN='postgres://velora:velora@127.0.0.1:5433/velora?sslmode=disable' \
go run ./cmd/migrate

cd ..
VELORA_OIDC_CLIENT_ID='从本地配置读取' \
VELORA_OIDC_CLIENT_SECRET='仅通过环境变量注入' \
CASDOOR_TEST_USERNAME='本地测试账号' \
CASDOOR_TEST_PASSWORD='本地测试密码' \
VELORA_ACCEPTANCE_EVIDENCE_DIR=./artifacts/acceptance \
./scripts/local-casdoor-oidc-smoke.sh
```

验收覆盖：授权码 + PKCE、nonce、一次性 state、callback 登录、`/me` 会话、重复 callback 拒绝、CSRF 保护退出登录和会话失效。失败时必须根据报告修复，不允许把失败改成“通过”。

## MinIO Object Lock/WORM

```bash
docker compose -f deployments/compose/local-storage-contract.yml up -d minio
docker compose -f deployments/compose/local-storage-contract.yml run --rm init

cd server
VELORA_CONFIG=configs/minimal.yaml \
VELORA_DATABASE_DSN='postgres://velora:velora@127.0.0.1:5433/velora?sslmode=disable' \
VELORA_STORAGE_PROVIDER=minio \
VELORA_STORAGE_ENDPOINT=http://127.0.0.1:9000 \
VELORA_STORAGE_REGION=us-east-1 \
VELORA_STORAGE_BUCKET=velora-worm \
VELORA_STORAGE_PATH_STYLE=true \
VELORA_STORAGE_TLS=false \
VELORA_STORAGE_ACCESS_KEY=minioadmin \
VELORA_STORAGE_SECRET_KEY='仅本地契约环境使用' \
VELORA_STORAGE_CONTRACT_OUTPUT=../artifacts/acceptance/minio-storage-contract.json \
go run ./cmd/storage-contract-check
```

契约会实测读写、SHA-256 checksum、版本、Object Lock、Compliance retention 和 Legal Hold，并用 SHA-256 绑定 endpoint/region/bucket/prefix。生产启动时设置 `VELORA_STORAGE_CAPABILITY_CONTRACT_FILE`，目标不匹配或证据被篡改会拒绝启动。

MinIO 镜像版本已固定；国内镜像不可用时可显式设置 `DOCKER_REGISTRY=docker.io`，不可在生产环境使用本地默认账号。

## 双库备份与 WAL/PITR

该演练使用随机 Compose project 和临时卷，创建 Velora/Casdoor 两个数据库，验证 `wal_level`、`archive_mode`、`archive_command`、WAL 归档、命名恢复点、恢复后 marker，以及现有双库备份脚本的两份 dump、摘要和隔离恢复：

```bash
DOCKER_REGISTRY=docker.io \
PITR_PORT=0 \
VELORA_ACCEPTANCE_EVIDENCE_DIR=./artifacts/acceptance \
./scripts/local-dual-db-pitr-smoke.sh
```

`PITR_PORT=0` 表示自动分配端口。脚本结束会清理临时容器、卷和恢复数据库；本地通过不代表生产 RPO/RTO、跨可用区或跨地域灾备已认证，生产仍需按计划执行带业务流量的恢复演练。
