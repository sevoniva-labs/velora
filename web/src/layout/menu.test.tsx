import { describe, expect, it } from 'vitest'
import { PORTAL_MANAGE, SYSTEM_USER_READ } from '../auth/permissions'
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
  })

  it('selects the correct navigation item for nested routes', () => {
    expect(adminActiveKey('/admin/applications/app-1')).toBe('admin-apps')
    expect(adminActiveKey('/admin/users/user-1')).toBe('admin-users')
  })
})
