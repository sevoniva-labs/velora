# 验证证据与边界

本文只记录在当前工作区实际执行并得到成功退出码的验证，不把配置存在、接口预留或文档声明等同于兼容认证。

> Velora 使用该目录作为后端子树；前端仍位于父目录 `../web`，且按要求不复制、不修改。脚手架原生的 `make ci-web`/`make ci-web-e2e` 假定 `server/web` 是自己的 workspace，因此不属于本项目的权威前端门禁。

## 当前已验证

### Latest complete gate

- 2026-08-20: 后端 `make ci-go`、`make ci-deploy`、`make security-tools` 通过；仓库根目录 `make test` 通过未修改的前端 lint/test/build。由于前端不属于后端子树，不能把脚手架原生 `make verify` 的前端 workspace 结果当作 Velora 结果。
- The complete gate does not certify a target vendor, hardware, operating system, HSM/KMS, cluster, air-gapped bundle, or disaster-recovery topology; those require the target evidence workflows below.

### Go 与安全边界

- Go 依赖通过 `https://goproxy.cn` 获取，`go.sum` 已固定。
- Kratos v2 后端单元测试、静态检查和 Phase 1 后端门禁已执行通过。
- HTTP SPA 测试覆盖随机 CSP nonce 注入、脚本严格策略、Wujie 审批策略、独立 `connect-src`/`frame-src` 和签名静态资源 URL 不重定向。
- 配置测试覆盖生产来源必须 HTTPS、拒绝通配符/路径/用户信息/查询参数以及重复来源。

### 前端

- `pnpm install --frozen-lockfile`、workspace typecheck、单元测试和 Vite 8 生产构建已执行通过。
- 构建预算检查已执行通过，覆盖初始/全量 JS raw 与 gzip、chunk 数、最大 chunk、CSS、source map 和 hash 文件名。
- 生产 Wujie 构建审批门禁已执行通过；未审批路径按预期拒绝。
- 使用调用方显式指定的本机 Chrome 执行 7 条 Playwright 生产构建 E2E，结果为 `7 passed`。
- 浏览器场景覆盖普通 SPA、Wujie + Host SDK、独立域 iframe、缺权拦截、双版本不可用故障关闭、签名清单回滚和默认脚本严格 CSP。

### 部署模板

- Helm 基础与信创 values 均通过 `helm lint`。
- 基础开发配置与信创生产配置均完成 `helm template` 渲染。
- 生产模板会拒绝关闭 NetworkPolicy、缺少明确 ingress/egress 或使用无 digest 镜像；基础默认 values 不能冒充生产可部署配置。
- `make offline-check` 已具备锁文件、信创配置、公共 OCI 源、离线包 provenance、镜像 digest 和 SHA-256 manifest 的静态门禁；未提供 `OFFLINE_BUNDLE_DIR` 时不会宣称离线包验证完成。
- `make disaster-check` 已具备目标版本元数据、RPO/RTO、节点/网络/数据库/MQ/S3/机房/备份恢复七类场景的证据格式校验；当前示例仍是 `Not certified`，必须由真实目标环境报告通过 `disaster-check-certified`。

## 可复现命令

```bash
GOPROXY=https://goproxy.cn \
GOSUMDB='sum.golang.org https://goproxy.cn/sumdb/sum.golang.org' \
go test ./internal/platform/config ./internal/platform/httpserver ./internal/bootstrap

# 前端保持在父目录，使用仓库根目录的未修改前端门禁
cd .. && make test

PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 \
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/path/to/chrome \
make ci-web-e2e

helm lint deploy/helm/velora
helm lint deploy/helm/velora -f deploy/helm/velora/values-xinchuang.yaml
helm template velora deploy/helm/velora --set config.environment=development >/tmp/velora-base.yaml
helm template velora deploy/helm/velora -f deploy/helm/velora/values-xinchuang.yaml >/tmp/velora-xinchuang.yaml

make offline-check
OFFLINE_BUNDLE_DIR=/path/to/approved-bundle make offline-check
```

所有自动下载入口必须显式使用国内源，并在国内源失败时立即失败。禁止 `direct` 或其他参数造成不受控的海外静默回退；生产 CI 应优先使用组织内 Go Proxy/制品代理。

## 国内浏览器制品边界

受控安装命令使用锁文件固定的 Playwright 版本，并仅从 npmmirror 下载对应 Chromium。Jenkins 在完整门禁前显式执行该命令；下载或安装失败时流水线直接失败，不会回退海外源。macOS 本地验收也可通过显式路径使用已安装 Chrome。

公开镜像适合当前国内开发与交付环境。银行内网 CI 仍应将审核后的 Chromium 制品同步到内部制品库，固定版本和摘要，再通过显式路径执行。

## 尚未验证，不得宣称

- OTel Collector 真实进程联调：当前环境没有提供 `OTELCOL_BIN`。
- 真实 Nacos 3、RocketMQ 5、Kafka、Redis 集群和各云厂商 S3 端点的兼容认证。
- OceanBase、达梦、人大金仓、GaussDB 等国产数据库的驱动、SQL 方言、迁移、故障切换和性能认证。
- 国密 SSL、SM2 证书链、HSM/密码机、KMS 和密钥轮换演练。
- 组织内 Harbor、多架构镜像、离线包、Kubernetes 集群、备份恢复和两地三中心容灾演练。
- 等保测评、密评、金融监管验收或任何厂商认证。

这些项目必须在目标机构的网络、硬件、数据库、中间件和安全设备上形成独立测试报告，不能由脚手架单元测试替代。
