import { describe, expect, it } from 'vitest'
import { sanitizeMailHtml } from './MailDetailDrawer'

describe('sanitizeMailHtml', () => {
  it('removes CSS URLs, images, srcset and active content when images are disabled', () => {
    const html = '<div style="background:url(https://tracker.example/p.gif)"><img src="https://tracker.example/p.gif" srcset="https://tracker.example/2x.gif 2x"><svg><a href="https://tracker.example">x</a></svg><script>alert(1)</script></div>'
    const clean = sanitizeMailHtml(html, false)
    expect(clean).not.toContain('tracker.example')
    expect(clean).not.toMatch(/style|srcset|<img|<svg|<script/i)
  })

  it('allows an explicitly requested HTTPS image but still removes style and SVG content', () => {
    const clean = sanitizeMailHtml('<div style="background:url(https://tracker.example/css.gif)"><img src="https://cdn.example/image.png"><svg><circle /></svg></div>', true)
    expect(clean).toContain('<img src="https://cdn.example/image.png">')
    expect(clean).not.toMatch(/style|tracker\.example|<svg/i)
  })
})
