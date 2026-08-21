# 前端工程基座

Forge 前端是可独立演进的 `pnpm workspace`，不是后端仓库中的单体 Demo。默认交付一个企业管理 Shell；微前端能力默认关闭，只有确有团队自治和独立发布需求时才启用。

## 技术基线

- pnpm 11.22.0，锁文件固定依赖解析
- React 19.2.8 + TypeScript 6.0.2
- Ant Design 6.6.0 + Pro Components 3.1.14-6
- Vite 8.2.0 + TanStack Query 5.101.4
- Vitest + React Testing Library
- Playwright 生产构建 E2E
- Wujie 2.1.0，仅作为可选可信微应用运行时

版本集中在 workspace 根配置中治理。不得由业务应用私自引入另一套 React、Ant Design 或构建工具版本。

## Workspace 边界

```text
web/
├── apps/
│   ├── shell/                 企业管理宿主
│   └── example-remote/        微应用契约与回滚示例
├── packages/
│   ├── api-client/            统一 API、Envelope 与错误模型
│   ├── design-system/         主题、反馈与通用 UI
│   ├── host-sdk/              微应用最小能力桥
│   └── e2e/                   生产构建浏览器验收
├── pnpm-lock.yaml
└── pnpm-workspace.yaml
```

Shell 只依赖公共包导出的稳定子路径。业务模块不得反向导入其他应用内部源码，也不得绕过 `api-client` 自行复制认证、错误处理和请求追踪逻辑。

## Shell 基线

默认 Shell 提供布局、菜单、路由、环境标识、权限前置过滤、统一加载/异常状态和平台管理入口。后端始终是授权最终裁决者；前端隐藏菜单、路由和按钮只用于减少误操作，不能替代 API 鉴权和数据权限。

平台管理基线包括用户、组织、角色权限、会话、审计、安全策略、系统状态和账号安全。启用平台管理模块时，这些安全与审计能力必须作为整体交付，不能只保留用户列表。

## Runtime config

`runtime-config.js` 在应用脚本前加载，使同一构建产物可部署到 DEV、SIT、UAT、PRE 和 PROD。它只允许存放公开运行参数，例如 API 基址、环境标识、功能开关和微应用清单地址。

Cookie、Token、AccessKey、SecretKey、签名私钥、数据库连接串和任何可用于提权的值不得进入浏览器 runtime config。

## 微前端信任模型

微前端不是默认架构。单团队、同发布节奏的模块优先使用 Shell 内部懒加载；只有独立团队、独立版本和独立回滚确有收益时才拆分。

### 可信同源应用

- 可使用 Wujie，但默认关闭。
- 生产构建必须显式设置 `VITE_WUJIE_PRODUCTION_APPROVED=true`。
- 服务端必须同时设置 `VELORA_WEB_CSP_WUJIE_ENABLED=true` 和有效的 `VELORA_WEB_CSP_WUJIE_APPROVAL_REF`。
- 只能加载签名清单中声明的版本、入口、资源摘要和回滚版本。
- 主版本和声明的回滚版本都不可用时故障关闭，不临时拼接未知 URL。
- Wujie 不需要也不得启用 CSP `unsafe-eval`。

### 不可信或跨安全域应用

- 只能运行在独立 Origin 的 sandbox iframe 中。
- Origin 必须进入 `VELORA_WEB_CSP_FRAME_SOURCES` 精确白名单，不允许通配符。
- 宿主对远程健康检查或 API 的访问必须单独进入 `VELORA_WEB_CSP_CONNECT_SOURCES`。
- `frame-src` 与 `connect-src` 不互相继承，生产来源必须使用 HTTPS。
- iframe 默认 sandbox 不授予弹窗、顶层导航、下载或同源宿主能力。

### Host SDK

Host SDK 只暴露经版本化、可校验的最小能力，例如主体摘要、组织上下文、权限查询和受控导航。它不向微应用传递 Cookie、Bearer Token、CSRF Token 或原始认证响应，也不提供任意 `fetch` 代理。

## CSP 边界

默认 Shell 使用脚本严格策略：`script-src 'self'`，禁止 `unsafe-inline` 和 `unsafe-eval`。Ant Design/Pro Components 会在运行时生成 style 标签并使用 React 内联样式，因此默认样式策略保留显式 `unsafe-inline` 兼容例外；这不是“所有样式均已 nonce 化”。

服务端仍为每个 HTML 响应生成随机 nonce 并传给支持 CSP nonce 的组件，但不能据此声称第三方 CSS-in-JS 生成的全部样式节点都受 nonce 约束。若项目要求彻底移除样式 `unsafe-inline`，必须采用静态样式提取或替换不兼容组件，并重新执行浏览器 CSP 验收。

启用 Wujie 时仅额外放宽 `script-src 'unsafe-inline'`，仍禁止 `unsafe-eval`，并要求审批引用。银行核心域建议将 Wujie 应用放到独立的非核心宿主，不在核心交易 Shell 中扩大 CSP。

## 构建与预算

Vite 8 使用基于入口归属的代码拆分，Shell、远程应用和公共包按需加载。构建预算脚本检查初始与全量 JS 的 raw/gzip 大小、chunk 数、最大 chunk、CSS、source map 和资产 hash；预算写死在仓库脚本中，不允许通过环境变量临时放宽。

```bash
make ci-web
make web-budget
```

生产 Wujie 构建只有在审批变量存在时才通过。默认构建不包含可执行的 Wujie 生产路径。

## 测试

```bash
make ci-web
PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 \
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/path/to/chrome \
make ci-web-e2e
```

E2E 使用真实 Go SPA/CSP 服务和 Vite 生产构建，覆盖普通 SPA、可信 Wujie、独立域 iframe、权限过滤、双版本故障关闭、签名回滚和脚本严格 CSP。

`make web-e2e-install-cn` 是受控国内浏览器安装入口。当前公开国内 Playwright 浏览器制品只验证了 Linux ARM64 路径；脚本在不支持的平台明确失败，不静默回退海外源。macOS 验证使用调用方显式提供的本地 Chrome，不宣称完成国内镜像安装验证。

## 可观测扩展

浏览器异常统一汇聚到 Shell 的遥测适配点，覆盖 React ErrorBoundary、`window.error` 和 `unhandledrejection`。公共基座不绑定具体 APM 厂商；项目可接入行内监控或经审批的 SDK，但必须完成数据出境、敏感字段和供应链评审。
