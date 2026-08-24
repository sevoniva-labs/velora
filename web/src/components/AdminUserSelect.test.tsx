import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, expect, it, vi } from 'vitest'
import AdminUserSelect from './AdminUserSelect'
import { adminGetUser, adminPageUsers } from '../api/api'
import type { AdminUser } from '../types'

vi.mock('../api/api', () => ({ adminGetUser: vi.fn(), adminPageUsers: vi.fn() }))

describe('AdminUserSelect', () => {
  it('forwards the form id so the visible label names the combobox', async () => {
    vi.mocked(adminPageUsers).mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 50 })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    render(<QueryClientProvider client={client}><label htmlFor="approver">审批人</label><AdminUserSelect id="approver" /></QueryClientProvider>)

    expect(screen.getByLabelText('审批人')).toHaveAttribute('role', 'combobox')
  })

  it('resolves the selected user independently of the first directory page', async () => {
    vi.mocked(adminPageUsers).mockResolvedValue({ items: [], total: 500, page: 1, pageSize: 50 })
    vi.mocked(adminGetUser).mockResolvedValue({ id: 'user-420', loginName: 'carson', displayName: 'Carson', status: 'ACTIVE' } as AdminUser)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    render(<QueryClientProvider client={client}><AdminUserSelect value="user-420" /></QueryClientProvider>)

    expect(await screen.findByText('Carson（carson）')).toBeVisible()
    expect(adminGetUser).toHaveBeenCalledWith('user-420')
  })
})
