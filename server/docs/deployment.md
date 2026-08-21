# Deployment

## Binary / systemd

`deploy/systemd/velora.service` 提供 non-root、ProtectSystem、PrivateTmp 等基础加固。生产建议 Secret 文件 `0600`、应用目录只读、独立数据目录，并由反向代理/API 网关或应用自身 TLS 终止 HTTPS。

## Docker Compose

- `minimal.yaml`：PostgreSQL + API
- `mysql.yaml`：MySQL + API
- `standard.yaml`：PostgreSQL + Redis + S3-compatible 对象存储（可替换为 COS/OSS/MinIO/Ceph 等）
- `full.yaml`：PostgreSQL + Redis + RocketMQ 5 + Elasticsearch + S3-compatible 对象存储（可替换为 COS/OSS/MinIO/Ceph 等）+ Worker
- `kafka-streaming-dev.yaml`：叠加到 `full.yaml` 的可选 Kafka 流处理环境，不替代 RocketMQ 业务消息
- `oceanbase-external.yaml`：外部 OceanBase MySQL mode
- `nacos-dev.yaml`：本地 Nacos 3 开发辅助，不是生产 Nacos 集群模板
- `observability-dev.yaml`：本地 Prometheus/OTel Collector 辅助

如果使用外部 S3 兼容对象存储并由平台统一创建 bucket，可在 compose 使用前设置
`S3_BUCKET_AUTO_CREATE=false`（标准/全量场景都支持），避免 `mc` 在非 MinIO 平台执行初始化导致启动阻塞。

Compose 只用于开发/验证。生产中间件建议使用组织已有 HA 服务，不把开发 Compose 直接搬进生产。

开发 Compose 的 RocketMQ 镜像必须由组织内 Harbor 以 digest 提供。仓库不设置公共镜像回退；未连接真实 ACL/TLS 集群时，仅能把 Compose 结果记录为配置级验证，不能写入生产兼容认证矩阵。

## Kubernetes / Helm

Chart：`deploy/helm/velora`。

内置：non-root、read-only root filesystem、drop ALL capability、RuntimeDefault seccomp、startup/readiness/liveness、rolling update、HPA、PDB、NetworkPolicy、ServiceMonitor、API/Worker 分离、External Secret 引用、可选 PVC。

### 多副本前置条件

Chart 默认 `replicaCount=1` 是刻意的：Local Storage 与 Memory Cache 都不是集群共享状态。生产开启多副本/HPA 前至少：

1. Storage 改为 S3-compatible；
2. Cache 改为 Redis（限流/锁跨 Pod）；
3. Database 使用外部 HA 实例；
4. 如果启用消息发布，开启 Worker 并验证本地可靠消息表；
5. 配置反亲和/TopologySpread 和 PDB；
6. 将 NetworkPolicy egress 从开放兼容模式收紧到 DNS/DB/Redis/MQ/Search/S3/Nacos 等明确目标。

### Secret

生产优先：

```yaml
secretEnv:
  existingSecret: velora-production-secrets
```

该 Secret 建议由 External Secrets、Secrets Store CSI、Vault/KMS/组织密码平台维护，而不是把明文 Secret 写入 Helm values/Git。

### Xinchuang

```bash
helm upgrade --install velora deploy/helm/velora \
  -f deploy/helm/velora/values.yaml \
  -f deploy/helm/velora/values-xinchuang.yaml
```

`values-xinchuang.yaml` 提供 OceanBase/Redis/Nacos Config/GM/S3 的组合样例。Kubernetes 内服务发现固定使用 Service/DNS，不再把 Pod 重复注册到 Nacos；非 Kubernetes 环境可显式启用 Nacos Registry。真实麒麟/UOS/ARM/LoongArch/国产数据库组合必须进入兼容验证矩阵。

## 生产数据库迁移

Helm 默认 `VELORA_DATABASE_AUTO_MIGRATE=false`。生产不要让每个 API Pod 在启动时并发执行 schema migration。推荐发布顺序：

```text
备份/恢复点确认 → velora-migrate 一次性任务 → schema 校验 → API/Worker 滚动发布 → smoke test
```

容器镜像同时包含 `/app/velora-migrate`。发布流水线可使用与应用相同的 Config/Secret 执行该命令；失败时应阻断后续 rollout。涉及不可向后兼容的 DDL，项目还应采用 expand/migrate/contract 等兼容发布策略，不把破坏性 DDL 与新代码同时强切。
