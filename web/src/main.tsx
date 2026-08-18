import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ConfigProvider, App as AntApp } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import './index.css'
import { veloraTheme } from './theme/tokens'
import { router } from './router'
import UiScale from './components/UiScale'
import { AppErrorBoundary } from './components/AppErrorBoundary'

dayjs.locale('zh-cn')

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 30_000,
      gcTime: 10 * 60_000,
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppErrorBoundary>
      <ConfigProvider locale={zhCN} theme={veloraTheme} card={{ variant: 'outlined' }}>
        <AntApp>
          <QueryClientProvider client={queryClient}>
            <UiScale />
            <RouterProvider router={router} />
          </QueryClientProvider>
        </AntApp>
      </ConfigProvider>
    </AppErrorBoundary>
  </StrictMode>,
)
