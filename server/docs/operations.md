# Production Operations Baseline

## HA / 多实例

生产多副本建议同时满足：

- 数据库使用高可用服务并明确连接故障切换策略。
- `cache.provider=redis`，跨 Pod 限流、锁使用 Redis；不要用 Memory Provider 承担集群一致性。
- 文件改为 S3-compatible 对象存储；Local Provider 仅适合单实例或共享存储已明确的场景。
- 异步事件通过本地可靠消息表 + Worker；Worker 多副本依赖数据库原子 claim，PROCESSING 使用租约避免进程崩溃后永久卡死。交付语义为 at-least-once，消费者必须按事件 ID/业务幂等键去重。
- 作业中心必须复用 `jobcontrol` 状态机：重试有上限和退避，达到上限进入死信；补偿、回滚、取消必须审批并留存审计，不能由前端直接改状态。
- Ingress/网关健康检查使用 readiness，而不是仅使用进程 liveness。

## 备份与灾备

项目上线前必须明确 RPO/RTO，而不是由脚手架替业务决定。至少覆盖：

- 数据库全量/增量/PITR 能力和恢复演练
- 对象存储版本/备份策略
- Nacos 配置、密钥元数据、License/Feature 配置备份
- KMS/HSM 密钥备份与恢复流程（由密码产品体系保障）
- 异地/同城灾备切换流程和年度/季度演练记录

## 时间与证书

- 节点、数据库、中间件统一可靠时间源，审计时间必须可关联。
- TLS 证书设置到期告警并演练轮换。
- API Token、账号密码、数据库凭据和对象存储密钥设置轮换策略。

## 监控与告警

至少覆盖：

- 可用性：实例存活、readiness、服务发现注册状态
- API：QPS、5xx 比例、p95/p99 延迟、并发、响应大小
- 资源：CPU、内存、FD、goroutine、GC
- DB：连接池、慢 SQL、锁、复制/集群状态
- Redis/RocketMQ Business MQ/Kafka Streaming/Search/S3：连接、延迟、积压/失败、重试、死信和容量
- 安全：登录失败、账号锁定、权限拒绝、Token 创建/撤销、审计写入失败
- 业务：项目自行定义 SLI/SLO 和关键交易指标

## 发布

建议采用不可变镜像 + GitOps/流水线发布，生产禁止在 Pod 内修改应用文件。Release 必须保留：代码版本、镜像 digest、SBOM、扫描报告、配置版本、数据库 migration 版本和审批记录。
