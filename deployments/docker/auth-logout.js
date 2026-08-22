(async () => {
  try {
    await fetch('/api/logout', {
      method: 'POST',
      credentials: 'include',
      headers: { Accept: 'application/json' },
    })
  } finally {
    window.location.replace('https://home.sevoniva.com/login?logged_out=1')
  }
})()
