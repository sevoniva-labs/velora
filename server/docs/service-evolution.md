# 模块化单体到独立服务

Forge 默认交付模块化单体。只有领域具备独立数据责任、发布团队、容量或隔离需求，并能承担监控、值班、灾备和安全责任时，才拆为独立服务。

## 参考实现

结算示例同时提供以下层次：

| 层次 | 路径 | 依赖边界 |
|---|---|---|
| Proto 契约 | `api/proto/velora/v1/reference_settlement.proto` | 生成 HTTP、gRPC 和 OpenAPI |
| Domain | `examples/settlement/domain` | 仅依赖 Go 标准库 |
| Application | `examples/settlement/application` | 仅依赖 Domain port |
| Repository | `examples/settlement/repository` | 可替换开发 Adapter |
| Transport | `examples/settlement/transport` | Kratos、生成契约、统一 Principal |
| 独立进程 | `cmd/example-settlement-service` | Composition root、HTTP/gRPC、mTLS、OTel |

`make module-boundaries` 会阻止 Domain/Application 反向依赖 Kratos、gRPC、数据库或中间件 SDK。同一 `QueryService` 可以在 Core 中直接调用，也可以由独立进程通过生成的 Kratos Adapter 暴露。

## 本地回环运行

示例不会保存原始 Token，只读取 SHA-256 摘要。Token 至少 32 字节：

```bash
TOKEN="$(openssl rand -hex 32)"
DIGEST="$(printf '%s' "$TOKEN" | openssl dgst -sha256 -r | awk '{print $1}')"

VELORA_EXAMPLE_SETTLEMENT_ORGANIZATION_ID=org-1 \
VELORA_EXAMPLE_SETTLEMENT_TOKEN_SHA256="$DIGEST" \
go run ./cmd/example-settlement-service
```

默认 HTTP/gRPC 地址分别是 `127.0.0.1:18080` 和 `127.0.0.1:19090`。业务接口要求 Bearer Token、`settlement:read` scope 和完全一致的组织 ID；健康端点只有 `/healthz`、`/readyz` 和标准 gRPC Health。

## 非回环部署

监听任意非回环地址时，进程强制要求完整 mTLS 配置，缺少任一文件会拒绝启动：

```text
VELORA_EXAMPLE_SETTLEMENT_TLS_CERT_FILE
VELORA_EXAMPLE_SETTLEMENT_TLS_KEY_FILE
VELORA_EXAMPLE_SETTLEMENT_TLS_CLIENT_CA_FILE
```

证书和 Token 摘要应由 Kubernetes Secret、CSI 或组织密码平台挂载，不能写入 Git。OTLP 使用主应用相同的 `VELORA_OTLP_*` 企业 CA/mTLS 配置。

Kubernetes 内使用 Service/DNS 发现该服务，Nacos 只承担配置中心。非 Kubernetes 环境如需 Nacos Registry，应在项目 composition root 显式接入 `internal/platform/discovery`，并完成 TLS、鉴权、注册/摘除和故障测试。同一服务禁止同时使用 Kubernetes DNS 与 Nacos 注册。

## 拆分准入

拆分前必须同时满足：

1. 领域不直接查询其他领域表，跨域交互已有版本化 API 或事件契约。
2. 数据归属、事务边界、一致性级别、幂等键、重试、死信和补偿责任人明确。
3. HTTP/gRPC 身份、组织、权限、Trace Context 和审计上下文不会在网络边界丢失。
4. 服务可独立构建、部署、扩缩、回滚、备份和恢复。
5. Buf breaking、OpenAPI、安全、模块边界和故障测试均进入 CI。
6. 拆分收益高于网络调用、最终一致性和运维复杂度。

## 验证与限制

当前自动化已验证契约生成、组织隔离、权限拒绝、Token 定时安全比较、回环默认值、非回环 mTLS fail-closed、HTTP/gRPC 编译和模块依赖边界。

该示例的静态摘要 Token 仅用于展示独立进程的不可匿名调用，不等于银行统一服务身份方案。正式项目应接入机构统一 OIDC/JWT、服务证书或身份内省，并验证吊销、轮换、时钟偏差和故障策略。没有真实 Nacos、APISIX、证书平台和容器环境报告前，本示例状态是 **engineering baseline**，不是生产兼容认证。
