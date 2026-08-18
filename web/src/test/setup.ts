import '@testing-library/jest-dom/vitest'

// antd 组件在 jsdom 下需要 matchMedia。
if (typeof window !== 'undefined' && !window.matchMedia) {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList
}

// 简单 ResizeObserver mock（antd 表格/组件可能使用）。
if (typeof window !== 'undefined' && !window.ResizeObserver) {
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
}

// 清理 DOM。
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'
afterEach(() => {
  cleanup()
})
