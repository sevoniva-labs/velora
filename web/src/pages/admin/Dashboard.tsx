import { useQuery } from '@tanstack/react-query'
import { Card, Col, Row, Statistic, Typography } from 'antd'
import {
  AppstoreOutlined,
  FireOutlined,
  HeartOutlined,
  TagsOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons'
import { adminDashboard, queryKeys } from '../../api/api'
import { useMe } from '../../auth/useMe'

export default function AdminDashboard() {
  const { data } = useQuery({ queryKey: queryKeys.dashboard, queryFn: adminDashboard })
  const me = useMe()

  const stats = [
    { title: '应用总数', value: data?.applicationCount ?? '-', icon: <AppstoreOutlined />, color: '#1677FF' },
    { title: '启用应用', value: data?.enabledAppCount ?? '-', icon: <FireOutlined />, color: '#52C41A' },
    { title: '停用应用', value: data?.disabledAppCount ?? '-', icon: <FireOutlined />, color: '#D0D5DD' },
    { title: '应用分类', value: data?.categoryCount ?? '-', icon: <UnorderedListOutlined />, color: '#722ED1' },
    { title: '应用标签', value: data?.tagCount ?? '-', icon: <TagsOutlined />, color: '#FA8C16' },
    { title: '收藏总数', value: data?.favoriteCount ?? '-', icon: <HeartOutlined />, color: '#FA541C' },
    { title: '累计启动', value: data?.totalLaunches ?? '-', icon: <FireOutlined />, color: '#13C2C2' },
  ]

  return (
    <div>
      <div style={{ padding: '8px 0 16px' }}>
        <Typography.Title level={3} style={{ marginBottom: 4 }}>
          门户概览
        </Typography.Title>
        <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 0 }}>
          管理员：{me.data?.displayName || me.data?.username}
        </Typography.Paragraph>
      </div>

      <Row gutter={[16, 16]}>
        {stats.map((s) => (
          <Col xs={12} md={8} lg={6} key={s.title}>
            <Card>
              <Statistic
                title={
                  <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ color: s.color }}>{s.icon}</span>
                    {s.title}
                  </span>
                }
                value={s.value}
              />
            </Card>
          </Col>
        ))}
      </Row>

      <Card style={{ marginTop: 16 }}>
        <Typography.Paragraph style={{ marginBottom: 8, color: 'var(--velora-text)' }}>
          <strong>快速开始：</strong>
        </Typography.Paragraph>
        <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 4 }}>
          1. 在「应用管理」中创建应用，配置名称 / 图标 / 分类 / 地址 / 接入类型；
        </Typography.Paragraph>
        <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 4 }}>
          2. 在「访问策略」中控制哪些组织 / 角色 / 用户组 / 用户可以看到并访问该应用；
        </Typography.Paragraph>
        <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 0 }}>
          3. 在「分类 / 标签管理」中维护门户的分类与标签体系。
        </Typography.Paragraph>
      </Card>
    </div>
  )
}
