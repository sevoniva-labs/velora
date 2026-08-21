# Observability

Forge 将日志、指标、链路分开处理，但通过 `request_id` / `trace_id` 关联。

## Logging

- 应用统一 `slog` JSON 输出 stdout。
- `logx` 对 password/token/secret/cookie/authorization 等字段名做兜底脱敏。
- HTTP 日志不记录 query string，避免 URL 参数中的凭据/个人信息进入日志。
- 生产由 Fluent Bit / Vector / Filebeat / OTel Collector 等平台 Agent 采集，框架不强绑某一种日志后端。
- 审计日志与普通应用日志语义不同；需要长期留存/防篡改时应送独立审计存储。

## Metrics

内置 Prometheus 指标包括：请求总量、延迟直方图、响应大小、in-flight、Go runtime/process 指标。

HTTP 指标的 `route` 使用 chi 路由模板（如 `/api/v1/users/{id}`），禁止直接用原始 URL path，防止高基数标签拖垮 Prometheus。

## Tracing

OpenTelemetry 使用 W3C Trace Context，支持 OTLP HTTP exporter。生产通常发送到组织已有 OTel Collector，再转 Jaeger/Tempo/SkyWalking/APM 平台。

## pprof

pprof 默认关闭；且 `compliance.disable_debug_endpoints=true` 时即使误开配置也不会注册 debug endpoint。仅在受控管理网络、短时间故障诊断时开启。
