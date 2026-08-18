import { Button, Result } from 'antd'
import { useNavigate } from 'react-router-dom'
import { usePageTitle } from '../hooks/usePageTitle'

/** 404：页面不存在。企业门户应给出明确的引导，而不是静默重定向。 */
export default function NotFound() {
  const navigate = useNavigate()
  usePageTitle('页面不存在')

  return (
    <Result
      status="404"
      title="404"
      subTitle="您访问的页面不存在或已被移除。"
      extra={[
        <Button key="home" type="primary" onClick={() => navigate('/home')}>
          回到工作台
        </Button>,
        <Button key="back" onClick={() => navigate(-1)}>
          返回上一页
        </Button>,
      ]}
    />
  )
}
