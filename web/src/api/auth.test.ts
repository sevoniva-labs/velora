import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { beginOIDCLogin, completeOIDCLogin, getAuthCapabilities } from './api'

describe('OIDC web adapter', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    globalThis.fetch = vi.fn()
    window.sessionStorage.clear()
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
    window.sessionStorage.clear()
  })

  it('reads public production OIDC capability without exposing password mode', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(JSON.stringify({ code: '000000', data: {
      status: 'UP', auth_mode: 'oidc', password_login_enabled: false, casdoor_account_url: 'https://casdoor.example/account',
    } }), { status: 200 }))
    await expect(getAuthCapabilities()).resolves.toEqual({
      authMode: 'oidc', passwordLoginEnabled: false, casdoorAccountUrl: 'https://casdoor.example/account',
    })
  })

  it('stores only an internal return path before beginning OIDC', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(JSON.stringify({ code: '000000', data: {
      redirect_url: 'https://casdoor.example/login?state=opaque',
    } }), { status: 200 }))
    await expect(beginOIDCLogin('https://attacker.example/steal')).resolves.toBe('https://casdoor.example/login?state=opaque')
    expect(window.sessionStorage.getItem('velora.oidc.redirect')).toBe('/')
  })

  it('posts the callback code and state to the backend', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(JSON.stringify({ code: '000000', data: {} }), { status: 200 }))
    await completeOIDCLogin('casdoor', 'code-1', 'state-1')
    const [url, init] = vi.mocked(globalThis.fetch).mock.calls[0]
    expect(url).toBe('/api/v1/auth/federated/oidc/casdoor/callback')
    expect(JSON.parse(String((init as RequestInit).body))).toEqual({ code: 'code-1', state: 'state-1' })
  })
})
