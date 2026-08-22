(async () => {
  try {
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
  } finally {
    window.location.replace('https://home.sevoniva.com/login?logged_out=1')
  }
})()
