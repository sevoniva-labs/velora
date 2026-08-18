# Velora 品牌规范

本文档定义 Velora 门户的品牌资产用法（logo / 字体 / 色彩 / 质感 / 动效 / 文案），
新页面与组件必须遵守，保证视觉一致性。与架构决策见 `architecture.md` 互补。

## 1. 标识 Logo

资产文件：`web/public/sevoniva-mark.svg`

- 图形：2×2 圆角宫格（蓝 `#3B82F6` / 紫 `#6E62E8` / 橙 `#F5A623` / 青 `#1FB6A6`），右下格旋转 14°，透明底。
- 引用位置：`web/index.html`（favicon）、`PortalLayout`、`AdminLayout`、`Login`。
- 展示规则：
  - **蓝色顶栏**（`.velora-brand-mark`）：嵌在 30px 白色圆角 tile 内，img 19px，带投影。
  - **浅色登录页**：`.velora-login .velora-brand-mark` 覆盖为透明底无投影，img 30px 直接展示。
- 禁止：拉伸变形、改色、描边、加外发光、放在与四色冲突的杂色背景上。
- 更新方式：同名覆盖 `sevoniva-mark.svg` 即可（public 文件为 `no-cache` 协商缓存，刷新即生效；不要改引用名、不要加 `?v=` 参数）。

## 2. 字体

### 阿里妈妈数黑体（标题专用）

- 文件：`web/public/fonts/AlimamaShuHeiTi-Bold.woff2`（约 680KB，自托管）。
- 授权：阿里妈妈官方免费商用许可，LICENSE 全文在 `web/public/fonts/AlimamaShuHeiTi-LICENSE.txt`，再分发须保留。
- 定义：`index.css` 登录页区块顶部的 `@font-face`（`font-weight: 700`，`font-display: swap`）。
- **适用范围（仅标题类文字）**：
  - 顶栏品牌名 `.velora-brand-name`
  - 登录页 `.velora-login-brand-name` / `.velora-login-eyebrow` / `.velora-login-headline` / `.velora-login-title` / `.velora-login-submit`
- **禁止使用**：正文、说明文字、表格、表单输入、小字号（<13px）场景。
  数黑体仅有 700 单字重、字面率大，正文使用会失去层级并影响可读性。
- 正文与 UI 一律使用系统栈：`"PingFang SC", "Microsoft YaHei", ui-sans-serif, system-ui, ...`（定义于 `:root`）。
- 全局 `font-synthesis: none`：不得依赖浏览器合成加粗/斜体。

## 3. 色彩体系

主色为品牌蓝 **`#1677FF`**，全部颜色通过 CSS 变量（`:root`，`index.css`）引用，**禁止在组件内硬编码新蓝色**。

| Token | 值 | 用途 |
| --- | --- | --- |
| `--velora-primary` | `#1677ff` | 主操作、链接、强调 |
| `--velora-primary-hover` / `-active` | `#4096ff` / `#0958d9` | 悬停 / 按压 |
| `--velora-primary-soft` / `-softer` | `#e6f4ff` / `#f0f7ff` | 浅蓝底色、选中底 |
| `--velora-header-from/-mid/-to` | `#1d4ed8` / `#2563eb` / `#3b82f6` | 顶栏 100deg 三段渐变 |
| `--velora-title` | `#14203a` | 标题文字 |
| `--velora-text` | `#4f5f7a` | 正文文字 |
| `--velora-secondary` | `#667085` | 次要/说明文字 |
| `--velora-border` / `-light` | `#e5e9f2` / `#f0f2f7` | 边框 |
| `--velora-bg-layout` / `-container` | `#f5f6f8` / `#ffffff` | 页面底 / 容器底 |

产品决策：**只做浅色蓝色主题**，不引入深色模式与深色大底页面。

## 4. 质感约定（玻璃拟态）

- 面板：`linear-gradient(158deg, var(--velora-glass), var(--velora-glass-2))` + `border: 1px solid var(--velora-glass-line)` + `box-shadow: var(--velora-shadow), var(--velora-inset)`。
- 阴影必须带蓝调（`rgba(6,20,55,*)` 系），禁止中性灰投影。
- 圆角：卡片/面板 12–16px，按钮 10px，小元素 8–10px。
- 顶栏保留顶部 1px 高光线（`linear-gradient(rgba(255,255,255,.14), transparent) 100% 1px`）。

## 5. 动效约定

- 只动画 `transform` / `opacity`（合成层属性），动画元素加 `will-change: transform`。
- **禁止**在动画元素上使用 `backdrop-filter`：位移动画中会逐帧重算背景模糊，必卡顿（登录页磁贴已踩过）。
- 标准缓动：`cubic-bezier(0.45, 0.05, 0.55, 0.95)`（正弦感）；漂浮类周期 7–10s、多元素错开 delay。
- 必须处理 `@media (prefers-reduced-motion: reduce)` 降级（关闭非必要动画）。

## 6. 静态资源缓存约定（nginx）

配置：`deployments/docker/nginx.conf`

- `/assets/`（Vite 指纹化产物）：`Cache-Control: public, max-age=31536000, immutable`。
- 其余（index.html、`/fonts/`、logo 等 public 文件）：`expires -1`（`no-cache` 协商缓存，ETag 304 低成本）。
- 推论：public 文件内容更新**直接覆盖同名文件**即可触达所有客户端；历史上因 immutable 缓存导致 logo 更新不生效，已通过"修正缓存头 + 重命名引用"根治，勿回退该配置。

## 7. 文案语气

- 简单、专业、事实陈述；一句话只说一件事。
- 禁止：三段排比、破折号修饰、夸张营销词（如"秒级直达""一体处理""赋能"）、无实际协议支撑的合规声明（如"登录即代表同意…"）。
- 按钮/标签用短动词（登录、保存、重试）；说明文字以名词短语为主。
- 反例 → 正例（登录页副标题）：
  - ✗「统一认证、应用汇聚、待办与邮件——日常工作，从一个入口开始。」
  - ✓「统一认证，直达应用、待办与企业邮件。」
