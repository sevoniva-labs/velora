import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import { getSystemReadiness, listUserEffectiveApplicationAccess, messageFromResponse, normalizeUserEffectiveApplicationAccess } from './admin-platform'

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

describe('effective application access contract', () => {
  it('normalizes omitted protobuf repeated fields', () => {
    expect(normalizeUserEffectiveApplicationAccess({
      userId: 'user-1', applicationId: 'app-1', applicationCode: 'spectra', applicationName: 'Spectra',
      status: 'ACTIVE', roles: undefined, sources: undefined,
    } as never)).toMatchObject({ roles: [], sources: [] })
  })

  it('normalizes every item returned by the API', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ accesses: [{
      userId: 'user-1', applicationId: 'app-1', applicationCode: 'spectra', applicationName: 'Spectra', status: 'ACTIVE',
    }] })
    await expect(listUserEffectiveApplicationAccess('user-1')).resolves.toEqual([expect.objectContaining({ roles: [], sources: [] })])
  })
})
