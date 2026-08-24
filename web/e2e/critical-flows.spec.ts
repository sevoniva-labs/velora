import { expect, test, type Page, type Route } from '@playwright/test'

const ok = (data: unknown) => ({ code: '000000', message: 'success', data, request_id: 'e2e-critical' })
const error = (code: string, message: string) => ({ code, message, data: null, request_id: 'e2e-critical' })

async function fulfill(route: Route, status: number, body: unknown) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function captureAudit(page: Page, name: string) {
  const directory = process.env.VELORA_AUDIT_CAPTURE_DIR
  if (directory) await page.screenshot({ path: `${directory}/${name}.png`, fullPage: true, animations: 'disabled' })
}

function application(status = 'ENABLED') {
  return {
    id: 'app-1', code: 'spectra', name: 'Spectra', description: '数据分析平台',
    home_url: 'https://spectra.example.test', launch_url: 'https://spectra.example.test',
    launch_type: 'OIDC', status, lifecycle_status: 'PUBLISHED', config_version: 3,
    tags: [], policies: [],
  }
}

const binding = {
  id: 'binding-1', application_id: 'app-1', provider_key: 'casdoor', protocol: 'OIDC',
  provider_application_ref: 'spectra', public_client_id: 'client', issuer: 'https://auth.example.test',
  redirect_uris: ['https://spectra.example.test/api/v1/auth/oidc/callback'],
  environments: [
    { key: 'DEVELOPMENT', name: '开发环境', redirect_uris: ['http://localhost:8080/auth/callback'] },
    { key: 'TEST', name: '测试环境', redirect_uris: ['https://test.spectra.example.test/auth/callback'] },
    { key: 'PRODUCTION', name: '生产环境', redirect_uris: ['https://spectra.example.test/api/v1/auth/oidc/callback'] },
  ],
  scopes: ['openid', 'profile', 'email'], verification_status: 'PASSED', config_version: 2,
}

const appRole = {
  id: 'role-1', application_id: 'app-1', role_key: 'analyst', name: '分析员', description: '',
  risk_level: 'NORMAL', status: 'ACTIVE', config_version: 1,
}

interface CriticalState {
  authenticated: boolean
  actor: 'admin' | 'member' | 'limited'
  loginStep: number
  appStatus: 'ENABLED' | 'DISABLED'
  approvalStatus: 'PENDING' | 'APPROVED' | 'EXECUTED'
  approvalExecuted: boolean
  loginBodies: Array<Record<string, unknown>>
  applicationBodies: Array<Record<string, unknown>>
  accessBodies: Array<Record<string, unknown>>
  provisioningRetries: number
  provisioningBodies: Array<Record<string, unknown>>
  mfaEnabled: boolean
  mfaBodies: Array<Record<string, unknown>>
  failApprovals: boolean
}

function newState(overrides: Partial<CriticalState> = {}): CriticalState {
  return {
    authenticated: true,
    actor: 'admin',
    loginStep: 0,
    appStatus: 'ENABLED',
    approvalStatus: 'PENDING',
    approvalExecuted: false,
    loginBodies: [],
    applicationBodies: [],
    accessBodies: [],
    provisioningRetries: 0,
    provisioningBodies: [],
    mfaEnabled: false,
    mfaBodies: [],
    failApprovals: false,
    ...overrides,
  }
}

function approval(state: CriticalState) {
  const grant = {
    id: 'grant-approved', application_id: 'app-1', subject_type: 'DEPARTMENT', subject_id: 'dept-1',
    include_descendants: true, effect: 'ALLOW', roles: ['analyst'], status: 'ACTIVE', reason: '季度授权', version: 0,
  }
  return {
    id: 'approval-1', request_type: 'APPLICATION_ACCESS_CHANGE', action: 'portal.application.access_grants.replace',
    resource: 'portal_application', resource_id: 'app-1', summary: '变更 Spectra 使用范围', applicant_id: 'requester',
    status: state.approvalStatus, expires_at: '2026-08-25T10:00:00Z', created_at: '2026-08-24T10:00:00Z',
    tasks: [{ id: 'task-1', assignee_id: 'e2e-user', status: state.approvalStatus === 'PENDING' ? 'PENDING' : 'APPROVED', decision: state.approvalStatus === 'PENDING' ? '' : 'APPROVE', comment: '', decided_at: state.approvalStatus === 'PENDING' ? undefined : '2026-08-24T11:00:00Z' }],
    payload_json: JSON.stringify({ application_id: 'app-1', grants: [grant] }),
  }
}

async function body(route: Route): Promise<Record<string, unknown>> {
  return (route.request().postDataJSON() ?? {}) as Record<string, unknown>
}

async function installCriticalMock(page: Page, state: CriticalState) {
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname.replace('/api/v1', '')
    const method = request.method()

    if (path === '/system/health') return fulfill(route, 200, ok({
      status: 'UP', auth_mode: 'oidc', password_login_enabled: true,
      turnstile_enabled: true, turnstile_site_key: 'e2e-site-key', turnstile_action: 'login',
    }))
    if (path === '/me') {
      if (!state.authenticated) return fulfill(route, 401, error('200001', 'authentication required'))
      const limited = state.actor === 'limited'
      return fulfill(route, 200, ok({ user: {
        id: 'e2e-user', login_name: state.actor, display_name: state.actor === 'admin' ? '系统管理员' : state.actor === 'limited' ? '人员管理员' : '普通成员',
        organization_id: 'default', roles: state.actor === 'admin' ? ['system_admin'] : ['user'],
        permissions: limited ? ['system.user.read', 'system.organization.read'] : [],
      } }))
    }
    if (path === '/auth/login' && method === 'POST') {
      state.loginBodies.push(await body(route))
      state.loginStep += 1
      if (state.loginStep === 1) return fulfill(route, 428, error('200028', '需要多因素认证'))
      if (state.loginStep === 2) return fulfill(route, 403, error('200029', '请完成人机验证'))
      state.authenticated = true
      state.actor = 'member'
      return fulfill(route, 200, ok({}))
    }
    if (path === '/auth/logout' && method === 'POST') {
      state.authenticated = false
      return fulfill(route, 200, ok({ federated_logout_url: '/login' }))
    }
    if (path === '/mfa' && method === 'GET') return fulfill(route, 200, ok({ enabled: state.mfaEnabled }))
    if (path === '/mfa/totp/enrollment' && method === 'POST') {
      state.mfaBodies.push(await body(route))
      return fulfill(route, 200, ok({ secret: 'JBSWY3DPEHPK3PXP', provisioning_uri: 'otpauth://totp/Velora:member?secret=JBSWY3DPEHPK3PXP' }))
    }
    if (path === '/mfa/totp/enrollment/confirmation' && method === 'POST') {
      state.mfaBodies.push(await body(route))
      state.mfaEnabled = true
      return fulfill(route, 200, ok({ recovery_codes: ['RECOVERY-ONE', 'RECOVERY-TWO'] }))
    }

    if (path === '/portal/applications' || path === '/portal/recent') return fulfill(route, 200, ok({ applications: [], items: [], total: 0, page: 1, page_size: 20 }))
    if (path === '/portal/categories' || path === '/portal/favorites' || path === '/portal/tags') return fulfill(route, 200, ok({ categories: [], applications: [], tags: [] }))

    if (path === '/admin/portal/applications/app-1/onboarding') return fulfill(route, 200, ok({
      application: application(state.appStatus), binding, verifications: [], onboarding_checks: [
        { id: 'check-1', application_id: 'app-1', config_version: 3, check_type: 'OIDC_DISCOVERY', result: 'PASSED', occurred_at: '2026-08-24T10:00:00Z' },
      ], roles: [appRole], can_publish: true,
    }))
    if (path === '/admin/portal/applications/app-1' && method === 'PATCH') {
      const input = await body(route)
      state.applicationBodies.push(input)
      state.appStatus = String(input.status).toUpperCase() === 'DISABLED' ? 'DISABLED' : 'ENABLED'
      return fulfill(route, 200, ok({ application: application(state.appStatus) }))
    }
    if (path === '/admin/portal/applications/app-1/roles') return fulfill(route, 200, ok({ roles: [appRole] }))
    if (path === '/admin/portal/applications/app-1/access-grants' && method === 'GET') return fulfill(route, 200, ok({ grants: [] }))
    if (path === '/admin/portal/applications/app-1/access-grants:preview' && method === 'POST') return fulfill(route, 200, ok({ impact: { effective_users: 4, added_users: 4, revoked_users: 0, role_changed_users: 0, privileged_users: 0, provisioning_tasks: 4 }, effective_access: [] }))
    if (path === '/admin/portal/applications/app-1/access-grants' && method === 'PUT') {
      const input = await body(route)
      state.accessBodies.push(input)
      if ((input as { approvalId?: string }).approvalId) state.approvalExecuted = true
      return fulfill(route, 200, ok({ grants: (input as { grants?: unknown[] }).grants ?? [], impact: { effective_users: 4, added_users: 4, revoked_users: 0, role_changed_users: 0, privileged_users: 0, provisioning_tasks: 4 } }))
    }
    if (path === '/admin/portal/applications/app-1/effective-access') return fulfill(route, 200, ok({ effective_access: [] }))

    if (path === '/admin/portal/applications/app-1/provisioning-target' && method === 'GET') return fulfill(route, 200, ok({
      id: 'target-1', application_id: 'app-1', endpoint_url: 'https://spectra.example.test/api/v1/provisioning/users',
      signing_algorithm: 'HMAC-SHA256', secret_fingerprint: 'e2e', active_key_version: 1,
      delivery_status: state.provisioningRetries ? 'HEALTHY' : 'FAILED', last_error_code: state.provisioningRetries ? '' : 'NETWORK_ERROR', config_version: 1,
    }))
    if (path === '/admin/portal/applications/app-1/provisioning-target:retry' && method === 'POST') {
      state.provisioningRetries += 1
      return fulfill(route, 200, ok({ target: { id: 'target-1', application_id: 'app-1', endpoint_url: 'https://spectra.example.test/api/v1/provisioning/users', delivery_status: 'HEALTHY', config_version: 2 }, retried_messages: 3 }))
    }
    if (path === '/admin/portal/applications/app-1/provisioning-target' && method === 'PUT') {
      const input = await body(route)
      state.provisioningBodies.push(input)
      return fulfill(route, 200, ok({ target: { id: 'target-1', application_id: 'app-1', endpoint_url: input.endpointUrl, delivery_status: 'HEALTHY', config_version: 2 }, one_time_provisioning_secret: 'one-time-secret' }))
    }

    if (path === '/admin/departments') return fulfill(route, 200, ok({ departments: [{ id: 'dept-1', organization_id: 'default', parent_id: '', department_key: 'rd', name: '研发部', status: 'ACTIVE', sort_order: 1 }] }))
    if (path === '/admin/user-groups') return fulfill(route, 200, ok({ user_groups: [{ id: 'group-1', organization_id: 'default', group_key: 'core', name: '核心用户', description: '', status: 'ACTIVE', roles: [], member_ids: [], member_count: 0 }] }))
    if (path === '/admin/roles') return fulfill(route, 200, ok({ roles: [{ key: 'operations', name: '运营管理员', description: '', data_scope: 'ALL', permissions: [], data_scope_department_ids: [], status: 'ACTIVE' }] }))
    if (path === '/admin/users') return fulfill(route, 200, ok({ users: [{ id: 'user-1', organization_id: 'default', login_name: 'carson', display_name: 'Carson', email: 'carson@example.test', status: 'ACTIVE', identity_source: 'LOCAL', roles: [], entitlements: [], created_at: '2026-08-24T10:00:00Z' }], total: 1, page: 1, page_size: 50 }))
    if (path === '/admin/portal/categories') return fulfill(route, 200, ok({ categories: [] }))
    if (path === '/admin/portal/tags') return fulfill(route, 200, ok({ tags: [] }))

    if (path === '/approvals' && method === 'GET') {
      if (state.failApprovals) return fulfill(route, 503, error('500001', 'service unavailable'))
      const closed = { ...approval(state), id: 'approval-closed', summary: '已拒绝的历史申请', status: 'REJECTED', tasks: [] }
      return fulfill(route, 200, ok({ approvals: [approval(state), closed] }))
    }
    if (path === '/approvals/approval-1/decisions' && method === 'POST') {
      const input = await body(route)
      state.approvalStatus = String(input.decision) === 'APPROVE' ? 'APPROVED' : 'PENDING'
      return fulfill(route, 200, ok({ approval: approval(state) }))
    }

    if (path === '/admin/portal/applications') return fulfill(route, 200, ok({ applications: [application(state.appStatus)], total: 1, page: 1, page_size: 20 }))
    if (path === '/admin/temporary-role-grants') return fulfill(route, 200, ok({ grants: [] }))
    if (path === '/admin/access-reviews') return fulfill(route, 200, ok({ reviews: [] }))
    if (path === '/admin/audit-logs') return fulfill(route, 200, ok({ audit_logs: [], total: 0, page: 1, page_size: 20 }))
    return fulfill(route, 200, ok({}))
  })
}

async function selectOption(page: Page, label: string, option: string) {
  await page.getByLabel(label).click()
  await page.locator('.ant-select-dropdown:visible .ant-select-item-option:visible').filter({ hasText: option }).last().click()
}

test('MFA 与风险触发 Turnstile 会串联到同一次登录请求', async ({ page }) => {
  const state = newState({ authenticated: false, actor: 'member' })
  await page.route('https://challenges.cloudflare.com/**', (route) => route.fulfill({
    status: 200,
    contentType: 'application/javascript',
    body: `window.turnstile={render:(el,opts)=>{el.textContent='安全验证已完成';setTimeout(()=>opts.callback('e2e-turnstile-token'),0);return 'e2e-widget'},reset:()=>{},remove:()=>{}}`,
  }))
  await installCriticalMock(page, state)
  await page.goto('/login')
  await page.getByPlaceholder('请输入账号或邮箱').fill('member')
  await page.getByPlaceholder('请输入密码').fill('correct-password')
  await page.getByRole('button', { name: '登 录' }).click()
  await expect(page.getByPlaceholder('6 位验证码')).toBeVisible()
  await page.getByPlaceholder('6 位验证码').fill('123456')
  await page.getByRole('button', { name: '登 录' }).click()
  await expect(page.getByTestId('turnstile-widget')).toContainText('安全验证已完成')
  await expect(page.getByRole('button', { name: '登 录' })).toBeEnabled()
  await page.getByRole('button', { name: '登 录' }).click()
  await expect(page).toHaveURL(/\/home$/)
  expect(state.loginBodies).toHaveLength(3)
  expect(state.loginBodies[1]).toMatchObject({ mfaCode: '123456' })
  expect(state.loginBodies[2]).toMatchObject({ mfaCode: '123456', turnstileToken: 'e2e-turnstile-token' })
})

test('后台菜单和直达页面使用同一权限边界', async ({ page }) => {
  const state = newState({ actor: 'limited' })
  await installCriticalMock(page, state)
  await page.goto('/admin/users')
  await expect(page.getByText('组织与人员', { exact: true })).toBeVisible()
  await expect(page.getByText('用户', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('应用中心', { exact: true })).toHaveCount(0)
  await expect(page.getByText('安全与审计', { exact: true })).toHaveCount(0)
  await page.goto('/admin/applications')
  await expect(page.getByText('无权访问此页面', { exact: true })).toBeVisible()
})

test('后台列表查询会真实过滤当前数据', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '列表查询只在桌面执行一次')
  const state = newState()
  await installCriticalMock(page, state)
  await page.goto('/admin/approvals')
  await expect(page.getByText('已拒绝的历史申请', { exact: true })).toBeVisible()
  await page.getByText('展开', { exact: true }).click()
  await selectOption(page, '状态', '待审批')
  await page.getByRole('button', { name: /查\s*询/ }).click()
  await expect(page.getByText('已拒绝的历史申请', { exact: true })).toHaveCount(0)
  await expect(page.getByText('变更 Spectra 使用范围', { exact: true })).toBeVisible()
  await captureAudit(page, '04-approvals-filtered')
})

test('后台接口失败显示可重试错误而不是空数据', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '错误态只在桌面执行一次')
  await installCriticalMock(page, newState({ failApprovals: true }))
  await page.goto('/admin/approvals')
  await expect(page.getByText('数据加载失败', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: /重\s*试/ })).toBeVisible()
  await expect(page.getByText('暂无数据', { exact: true })).toHaveCount(0)
})

test('应用管理全部页签均加载对应的可操作内容', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '应用管理全页签仅在桌面执行一次')
  const state = newState()
  await installCriticalMock(page, state)
  await page.goto('/admin/applications/app-1')
  const checks = [
    ['概览', '接入状态'],
    ['基本信息', '编辑信息'],
    ['登录设置', '统一登录'],
    ['应用角色', '分析员'],
    ['使用范围', '添加使用范围'],
    ['账号下发', '下发状态'],
    ['上线检查', '是否可上线'],
    ['操作记录', '来源 IP'],
  ] as const
  for (const [tab, content] of checks) {
    await page.getByRole('tab', { name: tab }).click()
    await expect(page.getByText(content, { exact: true }).last()).toBeVisible()
    await expect(page.getByText('加载失败', { exact: true })).toHaveCount(0)
  }
})

test('用户可以自行启用 MFA 并取得一次性恢复码', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', 'MFA 自助设置仅在桌面执行一次')
  const state = newState({ actor: 'member' })
  await installCriticalMock(page, state)
  await page.goto('/user-center')
  await expect(page.getByText('未启用', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '启用多因素认证' }).click()
  const begin = page.getByRole('dialog', { name: '启用多因素认证' })
  await begin.getByLabel('当前密码').fill('correct-password')
  await begin.getByRole('button', { name: /继\s*续/ }).click()
  const bind = page.getByRole('dialog', { name: '绑定身份验证器' })
  await expect(bind.getByText('JBSWY3DPEHPK3PXP')).toBeVisible()
  await bind.getByLabel('验证码').fill('123456')
  await bind.getByRole('button', { name: /确认启用|确\s*认\s*启\s*用/ }).click()
  const recovery = page.getByRole('dialog', { name: '保存恢复码' })
  await expect(recovery.getByText('RECOVERY-ONE')).toBeVisible()
  await recovery.getByRole('button', { name: /我已妥善保存|我\s*已\s*妥\s*善\s*保\s*存/ }).click()
  await expect(page.getByText('已启用', { exact: true })).toBeVisible()
  expect(state.mfaBodies).toEqual([{ currentPassword: 'correct-password' }, { code: '123456' }])
})

test('部门、用户组、平台角色和指定人员均可加入应用使用范围', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '复杂授权表单仅在桌面执行一次')
  const state = newState()
  await installCriticalMock(page, state)
  await page.goto('/admin/applications/app-1')
  await page.getByRole('tab', { name: '使用范围' }).click()

  const cases = [
    { type: '部门', target: '研发部' },
    { type: '用户组', target: '核心用户' },
    { type: '平台角色', target: '运营管理员' },
    { type: '指定人员', target: 'Carson（carson）' },
  ]
  for (const item of cases) {
    await page.getByRole('button', { name: '添加使用范围' }).click()
    if (item.type !== '部门') await selectOption(page, '按什么范围', item.type)
    if (item.type === '指定人员') {
      await page.getByLabel('选择人员').click()
      await page.locator('.ant-select-dropdown:visible .ant-select-item-option:visible').filter({ hasText: item.target }).last().click()
    } else {
      await selectOption(page, '选择对象', item.target)
    }
    await page.getByRole('button', { name: /确\s*定/ }).click()
  }
  for (const name of ['研发部', '核心用户', '运营管理员', 'Carson']) await expect(page.getByText(name, { exact: true }).first()).toBeVisible()
  await page.getByRole('button', { name: '预览并保存' }).click()
  await expect(page.getByText('可使用人数')).toBeVisible()
  await page.getByRole('button', { name: '确认生效' }).click()
  await expect.poll(() => state.accessBodies.length).toBe(1)
  expect((state.accessBodies[0].grants as unknown[])).toHaveLength(4)
})

test('审批通过后可深链到应用并执行已批准的使用范围', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '审批执行链路仅在桌面执行一次')
  const state = newState()
  await installCriticalMock(page, state)
  await page.goto('/admin/approvals')
  const table = page.locator('.velora-admin-primary-table .ant-table').first()
  const pagination = page.locator('.velora-admin-primary-table .ant-pagination').first()
  const [tableBox, paginationBox] = await Promise.all([table.boundingBox(), pagination.boundingBox()])
  expect(tableBox && paginationBox && paginationBox.y >= tableBox.y + tableBox.height - 1, '分页器应位于表格底部').toBeTruthy()
  await page.getByRole('button', { name: '处理' }).click()
  await page.getByLabel('处理意见').fill('同意季度授权')
  await page.getByRole('button', { name: /提\s*交/ }).click()
  await expect(page.getByText('已批准', { exact: true }).first()).toBeVisible()
  await page.getByRole('button', { name: '前往执行' }).click()
  await expect(page).toHaveURL(/\/admin\/applications\/app-1$/)
  await page.getByRole('tab', { name: '使用范围' }).click()
  await expect(page.getByText('待执行变更', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '执行变更' }).click()
  await expect.poll(() => state.approvalExecuted).toBe(true)
  expect(state.accessBodies.at(-1)).toMatchObject({ approvalId: 'approval-1' })
})

test('账号下发失败可重试、修改接收地址并交付一次性密钥', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '账号下发写流程仅在桌面执行一次')
  const state = newState()
  await installCriticalMock(page, state)
  await page.goto('/admin/applications/app-1')
  await page.getByRole('tab', { name: '账号下发' }).click()
  await expect(page.getByText('下发异常')).toBeVisible()
  await page.getByRole('button', { name: '重新下发' }).click()
  await expect.poll(() => state.provisioningRetries).toBe(1)
  await page.getByRole('button', { name: '修改接收地址' }).click()
  const endpoint = 'https://spectra.example.test/api/v2/provisioning/users'
  await page.getByLabel('接收地址').fill(endpoint)
  await page.getByRole('button', { name: /保\s*存/ }).click()
  await expect(page.getByText('保存账号下发密钥').last()).toBeVisible()
  await page.getByRole('button', { name: '已保存' }).click()
  expect(state.provisioningBodies.at(-1)).toMatchObject({ endpointUrl: endpoint, credentialDeliveryMode: 'BROWSER' })
})

test('应用停用和恢复会提交明确状态且详情立即更新', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop-chromium', '应用生命周期写流程仅在桌面执行一次')
  const state = newState()
  await installCriticalMock(page, state)
  await page.goto('/admin/applications/app-1')
  await page.getByRole('tab', { name: '基本信息' }).click()

  for (const [option, expectedStatus] of [['停用', 'DISABLED'], ['可用', 'ENABLED']] as const) {
    await page.getByRole('button', { name: '编辑信息' }).click()
    await selectOption(page, '使用状态', option)
    await page.getByRole('button', { name: /保\s*存/ }).click()
    await expect.poll(() => state.appStatus).toBe(expectedStatus)
    await expect(page.getByText(option === '停用' ? '已停用' : '可用', { exact: true }).first()).toBeVisible()
  }
  expect(state.applicationBodies.map((item) => item.status)).toEqual(['DISABLED', 'ENABLED'])
})

test('退出登录清理会话并回到 Velora 登录页', async ({ page }) => {
  const state = newState({ actor: 'member' })
  await installCriticalMock(page, state)
  await page.goto('/home')
  await page.getByRole('button', { name: /当前用户/ }).click()
  await page.getByRole('menuitem', { name: /退出登录/ }).click()
  await expect(page).toHaveURL(/\/login(?:\?.*)?$/)
  await expect(page.getByRole('heading', { name: '登录' })).toBeVisible()
  expect(state.authenticated).toBe(false)
})
