import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import TurnstileWidget from './TurnstileWidget'

type WidgetOptions = Parameters<NonNullable<typeof window.turnstile>['render']>[1]

describe('TurnstileWidget', () => {
  afterEach(() => {
    delete window.turnstile
  })

  it('keeps the verification unobtrusive and reports a completed challenge', async () => {
    let options: WidgetOptions | undefined
    const onVerify = vi.fn()
    window.turnstile = {
      render: vi.fn((_element, value) => {
        options = value
        return 'widget-1'
      }),
      reset: vi.fn(),
      remove: vi.fn(),
    }

    render(<TurnstileWidget siteKey="site-key" onVerify={onVerify} />)

    await waitFor(() => expect(window.turnstile?.render).toHaveBeenCalledOnce())
    expect(options?.size).toBe('flexible')
    expect(options?.appearance).toBe('interaction-only')
    expect(options).not.toHaveProperty('before-interactive-callback')
    expect(options).not.toHaveProperty('after-interactive-callback')
    options?.callback('verified-token')
    expect(onVerify).toHaveBeenLastCalledWith('verified-token')
  })

  it('stops unbounded retries and offers an explicit reload after failure', async () => {
    let options: WidgetOptions | undefined
    const onExpire = vi.fn()
    const onVerify = vi.fn()
    const renderWidget = vi.fn((_element: HTMLElement, value: WidgetOptions) => {
      options = value
      return 'widget-1'
    })
    window.turnstile = { render: renderWidget, reset: vi.fn(), remove: vi.fn() }

    render(<TurnstileWidget siteKey="site-key" onVerify={onVerify} onExpire={onExpire} />)
    await waitFor(() => expect(renderWidget).toHaveBeenCalledOnce())
    act(() => options?.['timeout-callback']?.())

    expect(options?.retry).toBe('never')
    expect(options?.['refresh-expired']).toBe('auto')
    expect(options?.['refresh-timeout']).toBe('auto')
    act(() => options?.['error-callback']?.('network-error'))
    expect(onExpire).toHaveBeenCalledOnce()
    expect(onVerify).toHaveBeenLastCalledWith('')
    expect(screen.getByRole('alert')).toHaveTextContent('安全验证暂不可用')
    expect(screen.getByRole('button', { name: '重新加载' })).toBeVisible()
    expect(screen.getByTestId('turnstile-widget')).not.toBeVisible()
  })
})
