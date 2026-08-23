import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { beginOIDCLogin, completeOIDCLogin, getAuthCapabilities, getMe, sessionBridgeFallbackURL } from './api'

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

  it('builds a ticket-free authorization fallback on the trusted auth origin', () => {
    const fallback = new URL(sessionBridgeFallbackURL(
      'https://auth.example.test/_velora/session/bridge',
      '/login/oauth/authorize?client_id=spectra&_velora_bridge_nonce=browser-only&state=opaque',
      'https://home.example.test',
    ))
    expect(fallback.origin).toBe('https://auth.example.test')
    expect(fallback.pathname).toBe('/login/oauth/authorize')
    expect(fallback.searchParams.get('client_id')).toBe('spectra')
    expect(fallback.searchParams.get('state')).toBe('opaque')
    expect(fallback.searchParams.has('_velora_bridge_nonce')).toBe(false)
  })

  it('rejects a bridge action outside the fixed auth endpoint', () => {
    expect(() => sessionBridgeFallbackURL('https://evil.example/_velora/session/bridge?next=x', '/', 'https://home.example.test')).toThrow('统一认证跳转地址无效')
  })

  it('recognizes scaffold administrator roles for the portal entry', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(JSON.stringify({ code: '000000', data: {
      user: { id: 'u1', login_name: 'admin', roles: ['system_admin'], permissions: [] },
    } }), { status: 200 }))
    await expect(getMe()).resolves.toMatchObject({ username: 'admin', admin: true })
  })

  it('does not grant the portal entry from an arbitrary role without permission', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(JSON.stringify({ code: '000000', data: {
      user: { id: 'u2', login_name: 'operator', roles: ['operator'], permissions: [] },
    } }), { status: 200 }))
    await expect(getMe()).resolves.toMatchObject({ username: 'operator', permissions: [], admin: false })
  })
})
