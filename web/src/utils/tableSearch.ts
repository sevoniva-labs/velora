import { useState } from 'react'

export type ClientTableSearchParams = Record<string, unknown>

interface ClientTableSearchOptions<T> {
  exact?: readonly (keyof T & string)[]
  values?: Partial<Record<keyof T & string, (row: T) => unknown>>
}

const ignoredKeys = new Set(['current', 'pageSize', '_timestamp'])

function empty(value: unknown): boolean {
  return value == null || value === '' || (Array.isArray(value) && value.length === 0)
}

function normalized(value: unknown): string {
  if (Array.isArray(value)) return value.map(normalized).join(' ')
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  return String(value ?? '').trim().toLocaleLowerCase('zh-CN')
}

function dateValue(value: unknown): number | undefined {
  if (value instanceof Date) return value.getTime()
  if (typeof value !== 'string' && typeof value !== 'number') return undefined
  const time = new Date(value).getTime()
  return Number.isFinite(time) ? time : undefined
}

function matches(actual: unknown, expected: unknown, exact: boolean): boolean {
  if (empty(expected)) return true
  if (Array.isArray(expected)) {
    if (expected.length === 2) {
      const actualTime = dateValue(actual)
      const from = dateValue(expected[0])
      const to = dateValue(expected[1])
      if (actualTime != null && from != null && to != null) return actualTime >= from && actualTime <= to
    }
    const actualValues = Array.isArray(actual) ? actual.map(normalized) : [normalized(actual)]
    return expected.map(normalized).every((value) => actualValues.some((candidate) => candidate === value))
  }
  const actualValue = normalized(actual)
  const expectedValue = normalized(expected)
  return exact ? actualValue === expectedValue : actualValue.includes(expectedValue)
}

/**
 * ProTable does not filter a controlled dataSource by itself. This helper keeps
 * its search form honest for small, already-loaded administration lists.
 */
export function filterClientTableRows<T extends object>(
  rows: readonly T[],
  params: ClientTableSearchParams,
  options: ClientTableSearchOptions<T> = {},
): T[] {
  const exact = new Set<string>(options.exact ?? [])
  const filters = Object.entries(params).filter(([key, value]) => !ignoredKeys.has(key) && !empty(value))
  if (!filters.length) return [...rows]
  return rows.filter((row) => filters.every(([key, expected]) => {
    const accessor = options.values?.[key as keyof T & string]
    const actual = accessor ? accessor(row) : (row as Record<string, unknown>)[key]
    return matches(actual, expected, exact.has(key))
  }))
}

export function useClientTableSearch<T extends object>(rows: readonly T[], options: ClientTableSearchOptions<T> = {}) {
  const [params, setParams] = useState<ClientTableSearchParams>({})
  return {
    dataSource: filterClientTableRows(rows, params, options),
    onSubmit: (next: ClientTableSearchParams) => setParams(next),
    onReset: () => setParams({}),
  }
}
