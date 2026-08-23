import { describe, expect, it } from 'vitest'
import { categoryFilterValue } from './Applications'

describe('categoryFilterValue', () => {
  it('keeps UUID category identifiers intact', () => {
    expect(categoryFilterValue('f45836cf-7ae4-4cd2-a0f4-8f258f8d2e92')).toBe(
      'f45836cf-7ae4-4cd2-a0f4-8f258f8d2e92',
    )
  })

  it('omits an empty category filter', () => {
    expect(categoryFilterValue('   ')).toBeUndefined()
  })
})
