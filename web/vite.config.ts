import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  define: {
    // 构建期注入版本（CI 传 VITE_APP_VERSION=commit hash）
    __APP_VERSION__: JSON.stringify(process.env.VITE_APP_VERSION ?? 'dev'),
  },
  server: {
    port: 5173,
    proxy: {
      // 开发期 API 代理到 Velora Server（默认 8080）
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 1400,
    rolldownOptions: {
      output: {
        advancedChunks: {
          groups: [
            {
              name: 'react-vendor',
              test: /node_modules\/(react|react-dom|react-router|react-router-dom|@tanstack|scheduler|use-sync-external-store)/,
            },
            {
              name: 'pro-vendor',
              test: /node_modules\/@ant-design\/(pro|icons|cssinjs|fast-color|colors)/,
            },
            {
              name: 'rc-vendor',
              test: /node_modules\/(@rc-component|rc-[a-z-]+|dayjs|@babel\/runtime)/,
            },
          ],
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
