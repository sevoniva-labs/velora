import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RequireAdminPermission from './RequireAdminPermission'
import { SYSTEM_USER_READ } from './permissions'
import { useMe } from './useMe'

vi.mock('./useMe', () => ({ useMe: vi.fn() }))

const mockedUseMe = vi.mocked(useMe)

describe('RequireAdminPermission', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders the page when the user has a required permission', () => {
    mockedUseMe.mockReturnValue({ data: { permissions: [SYSTEM_USER_READ], roles: [] } } as unknown as ReturnType<typeof useMe>)
    render(<MemoryRouter><RequireAdminPermission anyOf={[SYSTEM_USER_READ]}><div>用户页面</div></RequireAdminPermission></MemoryRouter>)
    expect(screen.getByText('用户页面')).toBeVisible()
  })

  it('renders a legal no-access state instead of mounting the page', () => {
    mockedUseMe.mockReturnValue({ data: { permissions: ['system.audit.read'], roles: [] } } as unknown as ReturnType<typeof useMe>)
    render(<MemoryRouter><RequireAdminPermission anyOf={[SYSTEM_USER_READ]}><div>用户页面</div></RequireAdminPermission></MemoryRouter>)
    expect(screen.queryByText('用户页面')).not.toBeInTheDocument()
    expect(screen.getByText('无权访问此页面')).toBeVisible()
    expect(screen.getByRole('button', { name: '返回工作台' })).toBeVisible()
  })
})
