# Architecture

## 目标

Forge 的核心目标是让业务模块稳定，而基础设施可以替换。框架采用模块化单体作为默认形态，不强迫项目为了“架构完整”拆微服务。

```text
React / Ant Design
       │ OpenAPI / HTTP
       ▼
Kratos HTTP / gRPC Adapter
       │
Application Services
       │
Domain
       │ ports / abstractions
       ▼
Platform & Adapters
DB | Cache | Business MQ | Streaming | Search | Storage | Crypto | Observability
```

## 依赖规则

1. `internal/domain` 不依赖 HTTP、数据库、Redis、RocketMQ、Kafka、Elasticsearch、S3 SDK。
2. `internal/app` 编排业务用例，不处理 HTTP 细节。
3. `internal/adapters` 将 HTTP、持久化等外部协议映射到应用层。
4. `internal/platform` 只提供跨业务基础能力。
5. `internal/bootstrap` 是唯一主要 composition root；`cmd/server` 保持极薄。
6. 可选中间件默认 Disabled/Memory Provider，不能成为启动硬依赖。
7. RocketMQ 是业务消息默认实现；Kafka 只实现独立 Streaming 端口，两者不互相降级或替代。
8. Kubernetes 使用 Service/DNS 服务发现，Nacos 只承担配置中心；非 Kubernetes 环境才启用 Nacos Registry，禁止双注册。

## 新业务模块推荐结构

```text
internal/domain/order/
internal/app/order/
internal/adapters/repository/order.go
api/proto/velora/v1/order.proto
internal/adapters/kratosapi/order.go
```

不要默认创建 controller/service/repository/domain/entity/model/dto/vo/bo 等十几层；只有出现清晰边界时才新增抽象。

模块拆分准入、参考结算服务及安全运行方式见 `docs/service-evolution.md`。
