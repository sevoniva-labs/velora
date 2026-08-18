import { useQuery } from '@tanstack/react-query'
import { Col, Row } from 'antd'
import {
  AppstoreOutlined,
  CheckCircleOutlined,
  HeartOutlined,
  PauseCircleOutlined,
  RocketOutlined,
  TagsOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons'
import AdminPageHead from '../../components/AdminPageHead'
import { adminDashboard, queryKeys } from '../../api/api'
import QueryErrorState from '../../components/QueryErrorState'
import { usePageTitle } from '../../hooks/usePageTitle'

const QUICK_STEPS = [
  '在「应用管理」创建应用，配置名称、图标、分类、地址与接入类型',
  '在「访问策略」控制哪些组织、角色、用户组可以看到并访问该应用',
  '在「分类 / 标签管理」维护门户的分类与标签体系',
]

export default function AdminDashboard() {
  usePageTitle('门户概览')

  const { data, isError, refetch } = useQuery({ queryKey: queryKeys.dashboard, queryFn: adminDashboard })

  const stats = [
    { title: '应用总数', value: data?.applicationCount ?? '-', icon: <AppstoreOutlined />, color: '#1677FF' },
    { title: '启用应用', value: data?.enabledAppCount ?? '-', icon: <CheckCircleOutlined />, color: '#52C41A' },
    { title: '停用应用', value: data?.disabledAppCount ?? '-', icon: <PauseCircleOutlined />, color: '#98A2B3' },
    { title: '应用分类', value: data?.categoryCount ?? '-', icon: <UnorderedListOutlined />, color: '#722ED1' },
    { title: '应用标签', value: data?.tagCount ?? '-', icon: <TagsOutlined />, color: '#FA8C16' },
    { title: '收藏总数', value: data?.favoriteCount ?? '-', icon: <HeartOutlined />, color: '#FA541C' },
    { title: '累计启动', value: data?.totalLaunches ?? '-', icon: <RocketOutlined />, color: '#13C2C2' },
  ]

  return (
    <div>
      <AdminPageHead title="门户概览" desc="应用、分类、收藏与启动数据一览。" />

      {isError ? (
        <QueryErrorState refetch={refetch} />
      ) : (
        <Row gutter={[16, 16]}>
          {stats.map((s) => (
            <Col xs={12} md={8} lg={6} key={s.title}>
              <div className="velora-admin-stat">
                <span
                  className="velora-admin-stat-icon"
                  style={{ color: s.color, background: `color-mix(in srgb, ${s.color} 10%, white)` }}
                >
                  {s.icon}
                </span>
                <div className="velora-admin-stat-text">
                  <span className="velora-admin-stat-value">{s.value}</span>
                  <span className="velora-admin-stat-label">{s.title}</span>
                </div>
              </div>
            </Col>
          ))}
        </Row>
      )}

      <div className="velora-admin-quick">
        <h3 className="velora-admin-quick-title">快速开始</h3>
        <ol className="velora-admin-quick-steps">
          {QUICK_STEPS.map((step) => (
            <li key={step}>{step}</li>
          ))}
        </ol>
      </div>
    </div>
  )
}
