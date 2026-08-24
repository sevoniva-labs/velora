import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import TurnstileWidget from './TurnstileWidget'

type WidgetOptions = Parameters<NonNullable<typeof window.turnstile>['render']>[1]

describe('TurnstileWidget', () => {
  afterEach(() => {
    delete window.turnstile
  })

  it('uses interaction-only mode and reports a completed challenge', async () => {
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
    expect(options?.appearance).toBe('interaction-only')
    expect(options).not.toHaveProperty('before-interactive-callback')
    expect(options).not.toHaveProperty('after-interactive-callback')
    options?.callback('verified-token')
    expect(onVerify).toHaveBeenLastCalledWith('verified-token')
  })

  it('resets a timed-out challenge without hiding the verification control', async () => {
    let options: WidgetOptions | undefined
    const reset = vi.fn()
    const onExpire = vi.fn()
    const onVerify = vi.fn()
    const renderWidget = vi.fn((_element: HTMLElement, value: WidgetOptions) => {
      options = value
      return 'widget-1'
    })
    window.turnstile = { render: renderWidget, reset, remove: vi.fn() }

    render(<TurnstileWidget siteKey="site-key" onVerify={onVerify} onExpire={onExpire} />)
    await waitFor(() => expect(renderWidget).toHaveBeenCalledOnce())
    act(() => options?.['timeout-callback']?.())

    expect(reset).toHaveBeenCalledWith('widget-1')
    expect(onExpire).toHaveBeenCalledOnce()
    expect(onVerify).toHaveBeenLastCalledWith('')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByTestId('turnstile-widget')).toBeVisible()
  })

  it('keeps a widget error recoverable through an explicit reload', async () => {
    let options: WidgetOptions | undefined
    const renderWidget = vi.fn((_element: HTMLElement, value: WidgetOptions) => {
      options = value
      return 'widget-1'
    })
    window.turnstile = { render: renderWidget, reset: vi.fn(), remove: vi.fn() }

    render(<TurnstileWidget siteKey="site-key" onVerify={vi.fn()} />)
    await waitFor(() => expect(renderWidget).toHaveBeenCalledOnce())
    act(() => options?.['error-callback']?.('network-error'))

    expect(await screen.findByRole('alert')).toHaveTextContent('安全验证暂不可用')
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }))
    await waitFor(() => expect(renderWidget).toHaveBeenCalledTimes(2))
  })
})
