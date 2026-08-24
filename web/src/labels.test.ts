import { describe, expect, it } from 'vitest'
import {
  APP_LIFECYCLE_LABEL,
  ONBOARDING_CHECK_LABEL,
  ONBOARDING_OPERATION_LABEL,
  ONBOARDING_STATUS_LABEL,
  OPERATION_STATUS_LABEL,
  enumLabel,
} from './labels'

describe('admin product labels', () => {
  it('maps application lifecycle and onboarding states to product copy', () => {
    expect(enumLabel(APP_LIFECYCLE_LABEL, 'IDENTITY_PENDING')).toBe('待配置登录')
    expect(enumLabel(ONBOARDING_STATUS_LABEL, 'PUBLISHED')).toBe('已上线')
    expect(enumLabel(ONBOARDING_OPERATION_LABEL, 'RECONCILE_PROVIDER')).toBe('同步登录配置')
    expect(enumLabel(OPERATION_STATUS_LABEL, 'SUCCEEDED')).toBe('已完成')
    expect(enumLabel(ONBOARDING_CHECK_LABEL, 'oidc_discovery')).toBe('统一登录配置')
  })

  it('does not expose unknown internal enum values', () => {
    expect(enumLabel(ONBOARDING_STATUS_LABEL, 'INTERNAL_FUTURE_STATE')).toBe('未知')
    expect(enumLabel(ONBOARDING_CHECK_LABEL, 'INTERNAL_CHECK', '接入检查')).toBe('接入检查')
  })
})
