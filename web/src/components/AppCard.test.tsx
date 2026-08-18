import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AppIcon } from './AppCard'
import type { Application } from '../types'

const baseApp: Application = {
  id: 1,
  code: 'devops',
  name: 'DevOps 平台',
  description: '统一流水线平台',
  icon: '🚀',
  ssoType: 'URL',
  owner: '',
  department: '',
  status: 'ENABLED',
  sort: 0,
  isFeatured: false,
  healthCheckEnabled: false,
  tags: [],
  createdAt: '2025-01-01T00:00:00Z',
  updatedAt: '2025-01-01T00:00:00Z',
}

describe('AppIcon', () => {
  it('emoji 图标渲染', () => {
    render(<AppIcon app={baseApp} />)
    expect(screen.getByText('🚀')).toBeInTheDocument()
  })

  it('URL 图标渲染 img', () => {
    const app = { ...baseApp, icon: 'https://cdn.example.com/icon.png' }
    render(<AppIcon app={app} />)
    const img = screen.getByRole('img')
    expect(img).toHaveAttribute('src', 'https://cdn.example.com/icon.png')
  })

  it('无图标时回退为首字母', () => {
    const app = { ...baseApp, icon: '' }
    render(<AppIcon app={app} />)
    expect(screen.getByText('D')).toBeInTheDocument()
  })
})
