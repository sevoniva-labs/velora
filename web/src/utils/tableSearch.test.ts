import { describe, expect, it } from 'vitest'
import { filterClientTableRows } from './tableSearch'

const rows = [
  { name: '研发平台', status: 'ACTIVE', roles: ['developer'], createdAt: '2026-08-24T10:00:00Z' },
  { name: '财务门户', status: 'INACTIVE', roles: ['auditor'], createdAt: '2026-08-25T10:00:00Z' },
]

describe('filterClientTableRows', () => {
  it('matches text without confusing exact enum values', () => {
    expect(filterClientTableRows(rows, { name: '研发', status: 'ACTIVE' }, { exact: ['status'] })).toEqual([rows[0]])
    expect(filterClientTableRows(rows, { status: 'ACTIVE' }, { exact: ['status'] })).not.toContain(rows[1])
  })

  it('matches array values and date ranges', () => {
    expect(filterClientTableRows(rows, { roles: ['auditor'] })).toEqual([rows[1]])
    expect(filterClientTableRows(rows, { createdAt: ['2026-08-25T00:00:00Z', '2026-08-25T23:59:59Z'] })).toEqual([rows[1]])
  })

  it('ignores ProTable pagination parameters', () => {
    expect(filterClientTableRows(rows, { current: 2, pageSize: 20 })).toEqual(rows)
  })
})
