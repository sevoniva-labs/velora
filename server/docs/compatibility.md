# Compatibility & Xinchuang Matrix

Forge 把“已有代码实现”“协议兼容 profile”“目标环境实测”严格分开。

| Dimension | Target | Scaffold status | Production acceptance |
|---|---|---|---|
| CPU | linux/amd64 | Built-in CI target | CI + target runtime test |
| CPU | linux/arm64 | Built-in CI target | CI + target runtime test |
| CPU | linux/loong64 | Portability target | 必须在龙芯环境构建/压测/回归 |
| OS | mainstream Linux | Baseline | distro/version matrix |
| OS | 银河麒麟 / 统信 UOS | Profile-ready | 指定 OS/内核/容器运行时验证 |
| DB | PostgreSQL | Built-in | supported version integration tests |
| DB | MySQL | Built-in | supported version integration tests |
| DB | OceanBase MySQL mode | Profile | OceanBase 目标版本全量回归 |
| DB | KingbaseES | Adapter slot | 驱动/SQL/DDL/事务/HA 全量验证 |
| DB | 达梦 | Adapter slot | 同上 |
| DB | GaussDB | Adapter slot | 同上 |
| Cache | Redis | Built-in | standalone/Sentinel/Cluster 按部署形态验证 |
| Registry/Config | Nacos | Built-in | 目标 Nacos 版本、鉴权、TLS、集群故障验证 |
| Business MQ | RocketMQ 5.x | Built-in engineering baseline | Proxy/版本/ACL2/TLS/FIFO/延时/事务/消费重试与故障验证 |
| Data streaming | Kafka | Built-in optional | broker/version/security/rebalance/吞吐与保留策略验证 |
| Search | Elasticsearch/OpenSearch | Built-in basic REST | mapping/security/HA/version test |
| Crypto | SHA-256/AES-GCM | Built-in | key management + crypto policy review |
| Crypto | SM3/SM4-GCM | GM baseline | 密评场景还需 SM2/证书/密码机等方案 |
| Container | Docker/containerd/K8s | Built-in manifests | 目标容器云安全策略验证 |

## 信创工程约束

- Core 优先 pure Go，减少无必要 CGO，降低 amd64/arm64/loong64 迁移成本。
- 镜像使用 multi-stage + non-root，不依赖 shell 作为运行时必要条件。
- 数据库差异集中在 driver/dialect/migration/repository；domain 不出现厂商类型。
- 中间件由 Provider 接口接入，国产替换不要求修改业务用例。
- 前端静态资源不依赖公网 CDN，适合内网/隔离网络构建后部署。
- `configs/xinchuang.yaml` 是信创配置基线，不是兼容认证证书。

## 每个“已验证”条目应记录

产品名、版本、补丁、CPU、OS、JDK/Go（如适用）、容器运行时、连接驱动、HA 形态、功能回归、性能基线、故障切换、备份恢复、已知限制和验证日期。
