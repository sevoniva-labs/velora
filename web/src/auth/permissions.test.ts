import { describe, expect, it } from 'vitest'
import {
  canAccessAdmin,
  hasPermission,
  hasAnyPermission,
  IDENTITY_CONSOLE,
  IDENTITY_READ,
  APPROVAL_REQUEST_READ,
  APPROVAL_TASK_DECIDE,
  PORTAL_MANAGE,
  isAdministrativeUser,
  portalRoleLabel,
} from './permissions'

describe('permission-driven administration gates', () => {
  it('allows an identity reader into the admin shell without granting portal management', () => {
    expect(canAccessAdmin([IDENTITY_READ])).toBe(true)
    expect(hasPermission([IDENTITY_READ], PORTAL_MANAGE)).toBe(false)
  })

  it('keeps the identity console behind its dedicated permission', () => {
    expect(hasPermission([IDENTITY_READ], IDENTITY_CONSOLE)).toBe(false)
    expect(hasPermission([IDENTITY_CONSOLE], IDENTITY_CONSOLE)).toBe(true)
  })

  it('supports legacy audit permission compatibility without wildcarding unrelated access', () => {
    expect(hasAnyPermission(['system.audit.read'], ['audit.read'])).toBe(true)
    expect(hasAnyPermission(['portal.application.read'], ['audit.read'])).toBe(false)
  })

  it('keeps the built-in superuser compatibility boundary explicit', () => {
    expect(canAccessAdmin([], ['system_admin'])).toBe(true)
    expect(hasPermission([IDENTITY_READ], PORTAL_MANAGE, ['system_admin'])).toBe(true)
    expect(canAccessAdmin([], ['operator'])).toBe(false)
  })

  it('does not present an approval-only member as an administrator', () => {
    const permissions = [APPROVAL_REQUEST_READ, APPROVAL_TASK_DECIDE]
    expect(canAccessAdmin(permissions, ['user'])).toBe(true)
    expect(isAdministrativeUser(permissions, ['user'])).toBe(false)
    expect(portalRoleLabel(permissions, ['user'])).toBe('审批人')
  })

  it('uses a precise product label for built-in administration roles', () => {
    expect(portalRoleLabel([], ['system_admin'])).toBe('系统管理员')
    expect(portalRoleLabel([IDENTITY_READ], ['iam_admin'])).toBe('身份管理员')
    expect(portalRoleLabel([PORTAL_MANAGE], ['custom_operator'])).toBe('管理成员')
  })
})
