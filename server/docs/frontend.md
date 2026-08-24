# 前端工程与验收

Velora 前端位于仓库根目录 `web/`，使用 React、TypeScript、Ant Design ProComponents、Vite、TanStack Query、Vitest 和 Playwright。它是单体 SPA，不启用微前端运行时。

## 产品约束

- 员工门户与管理后台共用同一套设计 Token、顶栏、导航和反馈语义。
- 管理页面优先使用 ProTable、ProForm、ProDescriptions、ProCard 和 PageContainer，不另造基础组件体系。
- 菜单、路由和按钮权限只改善体验；后端仍是授权最终裁决者。
- 页面只保留完成任务所需文案；字段名、状态和操作必须是业务语言。
- 列表分页固定在表格底部；空态、错误、加载和无权限是不同状态。
- Secret、Cookie、Token、AccessKey、数据库连接串不得进入前端配置或构建产物。

## 本地验证

从仓库根目录执行：

```bash
GOPROXY=https://goproxy.cn \
NPM_REGISTRY=https://registry.npmmirror.com \
make verify
```

前端门禁包括依赖锁定、Oxlint、TypeScript、71 条单元测试、生产构建、Bundle 预算和桌面/移动 Playwright 流程。浏览器安装使用 `server/scripts/install-playwright-chromium-cn.sh` 或显式的本机 Chrome 路径，不静默回退海外源。

## 生产边界

静态资源由同源 Web 容器提供，API 固定使用 `/api/v1`，认证使用 Secure/HttpOnly Cookie 和 CSRF。CSP、HSTS、frame、referrer 和缓存策略由后端/边缘代理统一输出；构建产物不依赖公网 CDN。
