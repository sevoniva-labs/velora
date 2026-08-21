import { describe, expect, it } from 'vitest'
import {
  canAccessAdmin,
  hasPermission,
  hasAnyPermission,
  IDENTITY_CONSOLE,
  IDENTITY_READ,
  PORTAL_MANAGE,
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
    expect(canAccessAdmin([], ['operator'])).toBe(false)
  })
})
