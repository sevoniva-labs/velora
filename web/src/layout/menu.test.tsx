import { describe, expect, it } from 'vitest'
import { APPROVAL_REQUEST_READ, PORTAL_MANAGE, SYSTEM_ACCESS_REVIEW_READ, SYSTEM_CONFIG_READ, SYSTEM_TEMPORARY_GRANT_READ, SYSTEM_USER_READ } from '../auth/permissions'
import { visibleNavigation } from './AdminLayout'
import { adminActiveKey, adminNavItems } from './menu'

describe('admin navigation', () => {
  it('only exposes sections covered by the current permissions', () => {
    const navigation = visibleNavigation(adminNavItems, [SYSTEM_USER_READ], [])

    expect(navigation.map((item) => item.label)).toEqual(['工作台', '组织与人员'])
    expect(navigation[1].children?.map((item) => item.label)).toEqual(['用户'])
  })

  it('keeps Casdoor and implementation concepts out of the main navigation', () => {
    const labels = JSON.stringify(visibleNavigation(adminNavItems, [PORTAL_MANAGE, SYSTEM_USER_READ], []))

    expect(labels).not.toContain('Casdoor')
    expect(labels).not.toContain('OIDC')
    expect(labels).not.toContain('身份接入')
    expect(labels).not.toContain('访问策略')
    expect(labels).not.toContain('分类管理')
    expect(labels).not.toContain('标签管理')
  })

  it('exposes governance and configuration through product tasks', () => {
    const navigation = visibleNavigation(adminNavItems, [APPROVAL_REQUEST_READ, SYSTEM_TEMPORARY_GRANT_READ, SYSTEM_ACCESS_REVIEW_READ, SYSTEM_CONFIG_READ], [])
    expect(JSON.stringify(navigation)).toContain('审批')
    expect(JSON.stringify(navigation)).toContain('临时授权')
    expect(JSON.stringify(navigation)).toContain('访问复核')
    expect(JSON.stringify(navigation)).toContain('配置发布')
  })

  it('selects the correct navigation item for nested routes', () => {
    expect(adminActiveKey('/admin/applications/app-1')).toBe('admin-apps')
    expect(adminActiveKey('/admin/users/user-1')).toBe('admin-users')
    expect(adminActiveKey('/admin/access-reviews')).toBe('admin-access-reviews')
    expect(adminActiveKey('/admin/config-changes')).toBe('admin-config-changes')
  })
})
