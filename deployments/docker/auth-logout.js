(async () => {
  try {
    if (window.location.hostname === 'home.sevoniva.com') {
      const csrfPrefix = 'velora_csrf='
      const csrfCookie = document.cookie.split(';').map((item) => item.trim()).find((item) => item.startsWith(csrfPrefix))
      const csrf = csrfCookie ? decodeURIComponent(csrfCookie.slice(csrfPrefix.length)) : ''
      const headers = { Accept: 'application/json', 'Content-Type': 'application/json' }
      if (csrf) headers['X-CSRF-Token'] = csrf
      await fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include', headers, body: '{}' })
    } else {
      await Promise.allSettled([
        fetch('/api/logout', {
          method: 'POST',
          credentials: 'include',
          headers: { Accept: 'application/json' },
        }),
        fetch('/_velora/session/logout', {
          method: 'POST',
          credentials: 'include',
          headers: { Accept: 'application/json' },
        }),
      ])
    }
  } finally {
    const target = window.location.hostname === 'home.sevoniva.com'
      ? 'https://home.sevoniva.com/login?logged_out=1'
      : 'https://home.sevoniva.com/_velora/logout'
    window.location.replace(target)
  }
})()
