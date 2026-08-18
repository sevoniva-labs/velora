// 全局错误边界：页面渲染或路由级错误时给出兜底界面（避免整站白屏），可重载恢复。
import { Component, type ErrorInfo, type ReactNode } from 'react'
import { Button, Result } from 'antd'
import { useNavigate } from 'react-router-dom'

/** 路由级 errorElement：配合 createBrowserRouter 处理懒加载/渲染错误。 */
export function RouteErrorFallback() {
  const navigate = useNavigate()
  return (
    <Result
      status="500"
      title="页面加载失败"
      subTitle="页面出现异常，请刷新重试，或返回工作台。"
      extra={[
        <Button key="reload" type="primary" onClick={() => window.location.reload()}>
          刷新页面
        </Button>,
        <Button key="home" onClick={() => navigate('/home')}>
          回到工作台
        </Button>,
      ]}
    />
  )
}

interface Props {
  children: ReactNode
}
interface State {
  hasError: boolean
}

/** 渲染级 ErrorBoundary：包裹整个应用，捕获任意组件渲染错误。 */
export class AppErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[Velora] render error:', error, info)
  }

  render() {
    if (this.state.hasError) {
      return (
        <Result
          status="error"
          title="应用出现异常"
          subTitle="请刷新页面重试；若问题持续，请联系系统管理员。"
          extra={
            <Button type="primary" onClick={() => window.location.reload()}>
              刷新页面
            </Button>
          }
          style={{ paddingTop: 80 }}
        />
      )
    }
    return this.props.children
  }
}
