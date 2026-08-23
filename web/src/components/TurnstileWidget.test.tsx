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

  it('stops waiting after a challenge timeout and permits an explicit retry', async () => {
    let options: WidgetOptions | undefined
    const renderWidget = vi.fn((_element: HTMLElement, value: WidgetOptions) => {
      options = value
      return 'widget-1'
    })
    window.turnstile = { render: renderWidget, reset: vi.fn(), remove: vi.fn() }

    render(<TurnstileWidget siteKey="site-key" onVerify={vi.fn()} />)
    await waitFor(() => expect(renderWidget).toHaveBeenCalledOnce())
    act(() => options?.['timeout-callback']?.())

    expect(await screen.findByRole('alert')).toHaveTextContent('安全验证暂不可用')
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }))
    await waitFor(() => expect(renderWidget).toHaveBeenCalledTimes(2))
  })
})
