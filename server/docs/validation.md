# 验证证据与边界

本文件记录权威门禁，不把接口预留、配置存在或单元测试等同于厂商/金融认证。

## 完整门禁

从仓库根目录执行：

```bash
GOPROXY=https://goproxy.cn \
NPM_REGISTRY=https://registry.npmmirror.com \
make verify
```

门禁覆盖：

- Proto/OpenAPI lint、兼容性、生成文件、认证与 CSRF 契约；
- Go `vet`、全量测试、竞态测试和模块边界；
- 前端 lint、71 条单元测试、TypeScript、生产构建和 Bundle 预算；
- 桌面及移动端 Playwright 关键产品流程；
- Helm/APISIX/PITR/可观测配置静态检查；
- gosec、govulncheck、staticcheck、golangci-lint；
- SBOM、许可证、Gitleaks 和供应链 provenance。

`make verify` 成功只证明当前提交通过仓库门禁。生产仍需执行 `make check-prod-config`、数据库备份、迁移、健康/就绪、真实登录、无权限、账号下发、目录读取、停用、退出和回滚验收。

## 不得据此宣称

以下事项必须在目标环境形成独立证据：多节点高可用、异地灾备、指定 RPO/RTO、真实 Nacos/RocketMQ/Redis/S3 集群兼容、国产数据库、国密 TLS/SM2/HSM/KMS、WORM/SIEM、离线制品库、等保、密评、渗透测试和监管验收。

所有自动下载入口必须使用组织内代理或国内源，不允许 `direct` 静默回退。
