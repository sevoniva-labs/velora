import { expect, test, type Page, type Route } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

type Session = 'anonymous' | 'member' | 'admin'
const ok = (data: unknown) => ({ code: '000000', message: 'success', data, request_id: 'e2e' })

async function captureAudit(page: Page, name: string) {
  const directory = process.env.VELORA_AUDIT_CAPTURE_DIR
  if (directory) await page.screenshot({ path: `${directory}/${name}.png`, fullPage: true, animations: 'disabled' })
}

async function fulfill(route: Route, status: number, body: unknown) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockVelora(page: Page, session: Session) {
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname.replace('/api/v1', '')
    if (path === '/system/health') return fulfill(route, 200, ok({ status: 'UP', auth_mode: 'oidc', password_login_enabled: true, turnstile_enabled: false }))
    if (path === '/system/ready') return fulfill(route, 200, ok({ status: 'UP', dependencies: [] }))
    if (path === '/me') {
      if (session === 'anonymous') return fulfill(route, 401, { code: '200001', message: 'authentication required', request_id: 'e2e' })
      return fulfill(route, 200, ok({ user: {
        id: 'e2e-user', login_name: session === 'admin' ? 'admin' : 'member', display_name: session === 'admin' ? '系统管理员' : '普通成员',
        organization_id: 'default', roles: session === 'admin' ? ['system_admin'] : ['user'], permissions: [],
      } }))
    }
    if (path === '/portal/applications' || path === '/portal/recent') return fulfill(route, 200, ok({ applications: [], items: [], total: 0, page: 1, page_size: 20 }))
    if (path === '/portal/categories' || path === '/portal/favorites' || path === '/portal/tags') return fulfill(route, 200, ok({ categories: [], applications: [], tags: [] }))
    if (path === '/admin/portal/applications/app-1/onboarding') return fulfill(route, 200, ok({
      application: { id: 'app-1', code: 'spectra', name: 'Spectra', description: '数据分析平台', home_url: 'https://spectra.example.test', launch_url: 'https://spectra.example.test', launch_type: 'OIDC', status: 'ENABLED', lifecycle_status: 'PUBLISHED', config_version: 3, tags: [], policies: [] },
      binding: { id: 'binding-1', application_id: 'app-1', provider_key: 'casdoor', protocol: 'OIDC', provider_application_ref: 'spectra', public_client_id: 'client', issuer: 'https://auth.example.test', redirect_uris: ['https://spectra.example.test/api/v1/auth/oidc/callback'], environments: [{ key: 'PRODUCTION', name: '生产环境', redirect_uris: ['https://spectra.example.test/api/v1/auth/oidc/callback'] }], scopes: ['openid', 'profile', 'email'], verification_status: 'PASSED', config_version: 2 },
      verifications: [], onboarding_checks: [], roles: [], can_publish: true,
    }))
    if (path === '/admin/portal/applications') return fulfill(route, 200, ok({ applications: [], total: 0, page: 1, page_size: 20 }))
    if (path === '/approvals') return fulfill(route, 200, ok({ approvals: [] }))
    if (path === '/admin/temporary-role-grants') return fulfill(route, 200, ok({ grants: [] }))
    if (path === '/admin/access-reviews') return fulfill(route, 200, ok({ reviews: [] }))
    return fulfill(route, 200, ok({}))
  })
}

test('登录入口稳定展示企业账号表单且默认不阻塞于验证码', async ({ page }) => {
  await mockVelora(page, 'anonymous')
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: '登录' })).toBeVisible()
  await expect(page.getByPlaceholder('请输入账号')).toBeVisible()
  await expect(page.getByPlaceholder('请输入密码')).toBeVisible()
  await expect(page.locator('.cf-turnstile')).toHaveCount(0)
  await expect(page).toHaveTitle(/登录/)
  await captureAudit(page, '01-login')
})

test('无应用权限是正常空态，不显示加载失败', async ({ page }) => {
  await mockVelora(page, 'member')
  await page.goto('/home')
  await expect(page.getByText('暂无可用应用')).toBeVisible()
  await expect(page.getByText('加载失败')).toHaveCount(0)
  await expect(page.getByText('网络异常')).toHaveCount(0)
})

test('管理后台保留分组导航，工作台卡片不重叠', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === 'mobile-chromium', '桌面卡片栅格检查')
  await mockVelora(page, 'admin')
  await page.goto('/admin')
  await expect(page.getByRole('link', { name: /管理后台/ })).toBeVisible()
  for (const group of ['应用中心', '组织与人员', '权限治理', '安全与审计', '平台设置']) await expect(page.getByText(group, { exact: true })).toBeVisible()
  const cards = page.locator('.velora-dashboard-stat-card')
  await expect(cards).toHaveCount(3)
  const boxes = await cards.evaluateAll((nodes) => nodes.map((node) => { const box = node.getBoundingClientRect(); return { left: box.left, right: box.right, top: box.top, bottom: box.bottom } }))
  for (let i = 0; i < boxes.length; i += 1) for (let j = i + 1; j < boxes.length; j += 1) {
    const overlap = boxes[i].left < boxes[j].right && boxes[i].right > boxes[j].left && boxes[i].top < boxes[j].bottom && boxes[i].bottom > boxes[j].top
    expect(overlap, `工作台卡片 ${i + 1} 与 ${j + 1} 不应重叠`).toBeFalsy()
  }
  await captureAudit(page, '02-admin-dashboard')
})

test('窄屏后台无水平溢出且导航可打开', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'mobile-chromium', '移动端响应式检查')
  await mockVelora(page, 'admin')
  await page.goto('/admin')
  await expect(page.getByRole('button', { name: '打开导航' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1)).toBeFalsy()
  await page.getByRole('button', { name: '打开导航' }).click()
  await expect(page.getByText('组织与人员', { exact: true }).last()).toBeVisible()
})

test('应用详情使用标准页签且信息卡片不重叠', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '桌面应用详情检查')
  await mockVelora(page, 'admin')
  await page.goto('/admin/applications/app-1')
  await expect(page.locator('.ant-page-header-heading-title').getByText('Spectra', { exact: true })).toBeVisible()
  for (const tab of ['概览', '基本信息', '登录设置', '应用角色', '使用范围', '账号下发', '上线检查', '操作记录']) await expect(page.getByRole('tab', { name: tab })).toBeVisible()
  const cards = page.locator('.velora-application-stat-card')
  await expect(cards).toHaveCount(3)
  const boxes = await cards.evaluateAll((nodes) => nodes.map((node) => { const box = node.getBoundingClientRect(); return { left: box.left, right: box.right, top: box.top, bottom: box.bottom } }))
  for (let i = 0; i < boxes.length; i += 1) for (let j = i + 1; j < boxes.length; j += 1) {
    const overlap = boxes[i].left < boxes[j].right && boxes[i].right > boxes[j].left && boxes[i].top < boxes[j].bottom && boxes[i].bottom > boxes[j].top
    expect(overlap, `应用概览卡片 ${i + 1} 与 ${j + 1} 不应重叠`).toBeFalsy()
  }
  await expect(page.getByText('下一项', { exact: true })).toHaveCount(0)
  await captureAudit(page, '03-application-detail')
})

test('配置发布使用业务化字段而不是底层参数向导', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '配置表单只在桌面执行一次')
  await mockVelora(page, 'admin')
  await page.goto('/admin/config-changes')
  await page.getByRole('button', { name: /新建变更/ }).click()
  await expect(page.getByLabel('发布环境')).toBeVisible()
  await expect(page.getByLabel('配置领域')).toBeVisible()
  await expect(page.getByLabel('变更名称')).toBeVisible()
  await expect(page.getByLabel('配置文件')).toBeVisible()
  await expect(page.getByLabel('文件校验值')).toBeVisible()
  await expect(page.getByText('namespace', { exact: true })).toHaveCount(0)
  await expect(page.getByText('dataId', { exact: true })).toHaveCount(0)
  await captureAudit(page, '05-config-change')
})

test('关键操作可通过键盘到达且页面放大后不横向溢出', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '桌面键盘与缩放检查')
  await mockVelora(page, 'anonymous')
  await page.goto('/login')
  for (let step = 0; step < 6 && !(await page.getByPlaceholder('请输入账号').evaluate((node) => node === document.activeElement)); step += 1) await page.keyboard.press('Tab')
  await expect(page.getByPlaceholder('请输入账号')).toBeFocused()
  await page.setViewportSize({ width: 640, height: 720 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1)).toBeFalsy()
})

test('登录页和管理工作台无严重无障碍缺陷', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '桌面无障碍基线')
  await mockVelora(page, 'anonymous')
  await page.goto('/login')
  const login = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
  expect(login.violations.filter((item) => ['serious', 'critical'].includes(item.impact ?? ''))).toEqual([])

  await page.unroute('**/api/v1/**')
  await mockVelora(page, 'admin')
  await page.goto('/admin')
  await expect(page.getByText('工作台', { exact: true }).last()).toBeVisible()
  const admin = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
  expect(admin.violations.filter((item) => ['serious', 'critical'].includes(item.impact ?? ''))).toEqual([])
})
