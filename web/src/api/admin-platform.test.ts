import { describe, expect, it } from 'vitest'
import { messageFromResponse } from './admin-platform'

describe('messageFromResponse', () => {
  it('accepts flattened protobuf message responses', () => {
    expect(messageFromResponse({ id: 'department-1' }, 'department')).toEqual({ id: 'department-1' })
  })

  it('accepts wrapped message responses', () => {
    expect(messageFromResponse({ department: { id: 'department-1' } }, 'department')).toEqual({ id: 'department-1' })
  })
})
