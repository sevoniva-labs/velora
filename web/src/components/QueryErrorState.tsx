// 查询错误态：网络失败 / 服务端错误时的统一兜底，带重试按钮。
// 避免把错误渲染成"空数据"或"不存在"误导用户。
import { Button, Result } from 'antd'

export interface QueryErrorStateProps {
  /** 由 useQuery 返回的 refetch，用于重试。 */
  refetch?: () => unknown
  /** 自定义说明文案。 */
  description?: string
  /** 紧凑模式（用于面板内小块）。 */
  compact?: boolean
}

export default function QueryErrorState({ refetch, description, compact }: QueryErrorStateProps) {
  return (
    <Result
      status="warning"
      title={compact ? '加载失败' : '数据加载失败'}
      subTitle={description ?? '网络异常或服务暂时不可用，请重试。'}
      extra={
        refetch ? (
          <Button type="primary" size={compact ? 'small' : 'middle'} onClick={() => void refetch()}>
            重试
          </Button>
        ) : undefined
      }
      style={compact ? { padding: '24px 0' } : undefined}
    />
  )
}
