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

## KMS/HSM/国密密钥治理

本地命令只验证密钥管理边界和失败闭环，不会生成、输出或持久化生产密钥：

```bash
cd server
VELORA_ACCEPTANCE_EVIDENCE_DIR=../artifacts/acceptance \
go run ./cmd/crypto-rotation-check
```

验收覆盖：同一操作员不能完成轮换、审批拒绝不会激活新版本、两个不同操作员批准后才激活；`gm` 软件实现完成 SM3/SM4 算法回环，未安装的 `kms`/`hsm`/`pkcs11` 适配器必须失败闭环。软件国密仅是算法接口和回环证据，不是商用密码产品认证；正式上线前必须接入经批准的 KMS/HSM/密码设备适配器、密钥策略、审计和双人审批，并完成机构要求的国密合规证明。

## WORM 审计归档与 SIEM 转发

在 MinIO Object Lock 契约通过后，执行真实归档、校验和保留版本删除拒绝：

```bash
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
VELORA_STORAGE_CAPABILITY_CONTRACT_FILE=../artifacts/acceptance/minio-storage-contract.json \
VELORA_ACCEPTANCE_EVIDENCE_DIR=../artifacts/acceptance \
go run ./cmd/audit-worm-check
```

验收覆盖：审计批次摘要、Object Lock Compliance retention、版本级删除拒绝和删除后收据复核；可靠转发器使用持久消息表作为 SIEM 出站队列。外部 SIEM 地址、网络隔离、告警规则、长期留存与合规审批仍必须在生产环境联调并留存证据，不能以本地通过替代。
