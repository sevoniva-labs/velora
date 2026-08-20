# 软件供应链与发布基线

Forge 把供应链安全分成 **PR/主干门禁** 和 **发布可信** 两部分。

## PR / 主干

- Go：`go vet`、`go test -race`、govulncheck、gosec
- API：OpenAPI lint、统一错误码契约检查
- Frontend：lint/build
- Secret：Gitleaks
- Dependency：Dependency Review
- Filesystem/Image：Trivy
- SBOM：CycloneDX
- Multi-arch：linux/amd64 + linux/arm64 镜像构建

## Tag Release

`.github/workflows/release.yml` 演示：

1. 仅由 `v*` tag 触发；
2. 构建并推送 immutable multi-arch image；
3. 生成 BuildKit provenance 与 SBOM attestation；
4. 使用 Cosign OIDC keyless 对**镜像 digest**签名，而不是签可变 tag；
5. 生产部署侧应增加签名/来源策略校验后才允许拉起 workload。

金融机构内部 Git/制品平台通常需要把这些步骤迁移到内部 CI、镜像仓库和证书/密钥体系；关键是保留“源码版本 → 构建 → SBOM → 镜像 digest → 签名 → 部署”的可追溯链，而不是依赖 GitHub 本身。

## 仍需项目级确定

- 分支保护、双人复核、发布审批
- 依赖白名单与许可证策略
- SAST/SCA/DAST/IAST 阈值和安全通行证
- 制品晋级/不可变策略
- 镜像仓库保留与回收策略
- 内部签名 CA/HSM/KMS 及密钥轮换
- SBOM 存档与漏洞再评估
- 构建节点隔离、网络访问白名单和依赖代理
