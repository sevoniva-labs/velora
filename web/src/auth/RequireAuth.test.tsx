import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import RequireAuth from './RequireAuth'
import { ApiError } from '../api/client'

const mockNavigate = vi.fn()

// mock useNavigate（ESM 无法 spyOn，用 vi.mock 局部替换）
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

// mock useMe
let mockUseMe: () => {
  isLoading: boolean
  isError: boolean
  error?: unknown
  data?: unknown
}
vi.mock('./useMe', () => ({
  useMe: () => mockUseMe(),
}))

function renderWithProviders(ui: React.ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/home']}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('RequireAuth', () => {
  beforeEach(() => {
    mockNavigate.mockReset()
    mockUseMe = () => ({ isLoading: true, isError: false, data: undefined })
  })
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('加载中显示 Spin', () => {
    renderWithProviders(<RequireAuth>content</RequireAuth>)
    expect(screen.getByText('正在验证登录状态…')).toBeInTheDocument()
  })

  it('401 时跳转登录页（携带 redirect）', async () => {
    mockUseMe = () => ({
      isLoading: false,
      isError: true,
      error: new ApiError(401, 'A01001', '未登录'),
    })
    renderWithProviders(<RequireAuth>content</RequireAuth>)
    await new Promise((r) => setTimeout(r, 50))
    expect(mockNavigate).toHaveBeenCalledWith('/login?redirect=%2Fhome', { replace: true })
  })

  it('非 401 错误展示错误提示', () => {
    mockUseMe = () => ({
      isLoading: false,
      isError: true,
      error: new ApiError(500, 'A05001', '网络异常'),
    })
    renderWithProviders(<RequireAuth>content</RequireAuth>)
    expect(screen.getByText('无法获取登录状态')).toBeInTheDocument()
    expect(screen.getByText(/网络异常/)).toBeInTheDocument()
  })

  it('已登录渲染 children', () => {
    mockUseMe = () => ({ isLoading: false, isError: false, data: { id: 'u-1' } })
    renderWithProviders(<RequireAuth>门户内容</RequireAuth>)
    expect(screen.getByText('门户内容')).toBeInTheDocument()
  })
})
