import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import { getSystemReadiness, messageFromResponse } from './admin-platform'

vi.mock('./client', () => ({ apiFetch: vi.fn() }))

beforeEach(() => vi.mocked(apiFetch).mockReset())

describe('messageFromResponse', () => {
  it('accepts flattened protobuf message responses', () => {
    expect(messageFromResponse({ id: 'department-1' }, 'department')).toEqual({ id: 'department-1' })
  })

  it('accepts wrapped message responses', () => {
    expect(messageFromResponse({ department: { id: 'department-1' } }, 'department')).toEqual({ id: 'department-1' })
  })
})

describe('getSystemReadiness', () => {
  it('uses the published readiness contract path', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ status: 'UP', dependencies: [] })
    await expect(getSystemReadiness()).resolves.toEqual({ status: 'UP', dependencies: [] })
    expect(apiFetch).toHaveBeenCalledWith('/system/ready')
  })
})
