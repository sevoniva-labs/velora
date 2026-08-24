import { useCallback, useState } from 'react'
import { Alert, Avatar, Button, Card, Descriptions, Divider, Form, Input, List, Modal, Space, Tag, Typography, message } from 'antd'
import { KeyOutlined, LaptopOutlined, LogoutOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  changePassword,
  getAuthCapabilities,
  getUserProfile,
  listSessions,
  logout,
  revokeAllSessions,
  revokeSession,
  type SessionDevice,
} from '../api/api'

const { Title, Text } = Typography

/** 用户中心（Phase C4 自助）：个人资料 / 修改密码 / 登录设备管理。 */
export default function UserCenter() {
  const [form] = Form.useForm()
  const [msg, msgCtx] = message.useMessage()
  const [deviceModalOpen, setDeviceModalOpen] = useState(false)

  const { data: profile } = useQuery({ queryKey: ['user-center', 'profile'], queryFn: getUserProfile })
  const { data: authCapabilities } = useQuery({ queryKey: ['auth', 'capabilities'], queryFn: getAuthCapabilities })
  const localPasswordManagement = authCapabilities?.authMode === 'password' && authCapabilities?.passwordLoginEnabled === true

  const changePwd = useMutation({
    mutationFn: ({ oldPassword, newPassword }: { oldPassword: string; newPassword: string }) =>
      changePassword(oldPassword, newPassword),
    onSuccess: async (res) => {
      msg.success(res.message ?? '密码已更新，请重新登录')
      form.resetFields()
      // 改密后服务端已吊销全部会话，本地强制退出
      await logout()
      window.location.href = '/login'
    },
    onError: (err: Error) => msg.error(err.message || '密码更新失败'),
  })

  const onFinish = useCallback(
    (values: { oldPassword: string; newPassword: string }) => {
      changePwd.mutate(values)
    },
    [changePwd],
  )

  const handleRevokeAll = useCallback(() => {
    Modal.confirm({
      title: '强制下线全部设备？',
      content: '所有已登录设备（包括当前设备）将被强制退出，需要重新登录。',
      okText: '全部下线',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        await revokeAllSessions()
        msg.success('已强制下线全部设备')
        await logout()
        window.location.href = '/login'
      },
    })
  }, [msg])

  return (
    <div style={{ maxWidth: 720, margin: '0 auto', padding: '24px 16px' }}>
      {msgCtx}
      <Title level={4}>用户中心</Title>

      {/* 个人资料 */}
      <Card title="个人资料" style={{ marginBottom: 16 }}>
        <Descriptions column={1} size="small">
          <Descriptions.Item label="头像">
            <Avatar src={profile?.avatar} size={48} />
          </Descriptions.Item>
          <Descriptions.Item label="用户名">
            <Space>
              <span>{profile?.username}</span>
              {profile?.admin && <Tag color="gold">管理员</Tag>}
            </Space>
          </Descriptions.Item>
          <Descriptions.Item label="显示名">{profile?.displayName || '-'}</Descriptions.Item>
          <Descriptions.Item label="邮箱">{profile?.email || '-'}</Descriptions.Item>
          <Descriptions.Item label="组织">{profile?.organization || '-'}</Descriptions.Item>
          <Descriptions.Item label="角色">
            <Space wrap>
              {(profile?.roles ?? []).map((r) => (
                <Tag key={r}>{r}</Tag>
              ))}
            </Space>
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 密码由统一身份中心管理；本地密码表单仅在后端明确声明的开发模式显示。 */}
      {localPasswordManagement ? <Card title="修改密码" style={{ marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="修改成功后，所有已登录设备将被强制下线，请使用新密码重新登录。"
        />
        <Form form={form} layout="vertical" onFinish={onFinish} style={{ maxWidth: 420 }}>
          <Form.Item
            name="oldPassword"
            label="当前密码"
            rules={[{ required: true, message: '请输入当前密码' }]}
          >
            <Input.Password placeholder="当前密码" />
          </Form.Item>
          <Form.Item
            name="newPassword"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '密码长度至少 8 位' },
              {
                pattern: /^(?=.*[A-Za-z])(?=.*\d).+$/,
                message: '密码需同时包含字母和数字',
              },
            ]}
          >
            <Input.Password placeholder="8-72 位，含字母和数字" />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label="确认新密码"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('newPassword') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次输入的密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password placeholder="再次输入新密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<KeyOutlined />} loading={changePwd.isPending}>
              修改密码
            </Button>
          </Form.Item>
        </Form>
      </Card> : <Card title="账号安全" style={{ marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          message="账号安全由企业统一管理"
          description="如需修改登录凭据或安全设置，请联系系统管理员。"
        />
      </Card>}

      {/* 登录设备 */}
      <Card
        title="登录设备"
        extra={
          <Space>
            <Button size="small" onClick={() => setDeviceModalOpen(true)} icon={<LaptopOutlined />}>
              查看设备
            </Button>
            <Button size="small" danger onClick={handleRevokeAll} icon={<LogoutOutlined />}>
              全部下线
            </Button>
          </Space>
        }
      >
        <Text type="secondary">管理已登录的设备会话。可查看最近活跃时间与 IP，或强制下线可疑设备。</Text>
        <DeviceModal open={deviceModalOpen} onClose={() => setDeviceModalOpen(false)} />
      </Card>
    </div>
  )
}

/** 设备列表弹窗（含逐条下线）。 */
function DeviceModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const queryClient = useQueryClient()
  const [msg, msgCtx] = message.useMessage()
  const { data: devices, isLoading } = useQuery({
    queryKey: ['auth', 'sessions'],
    queryFn: listSessions,
    enabled: open,
  })

  const revoke = useMutation({
    mutationFn: (sessionId: string) => revokeSession(sessionId),
    onSuccess: async () => {
      msg.success('已下线该设备')
      await queryClient.invalidateQueries({ queryKey: ['auth', 'sessions'] })
    },
    onError: (err: Error) => msg.error(err.message || '下线失败'),
  })

  return (
    <Modal title="登录设备" open={open} onCancel={onClose} footer={null} width={560}>
      {msgCtx}
      <List
        loading={isLoading}
        dataSource={devices ?? []}
        locale={{ emptyText: '暂无设备记录' }}
        renderItem={(d: SessionDevice) => (
          <List.Item
            actions={[
              <Button
                key="revoke"
                size="small"
                danger
                disabled={d.current}
                loading={revoke.isPending && revoke.variables === d.sessionId}
                onClick={() => revoke.mutate(d.sessionId)}
              >
                下线
              </Button>,
            ]}
          >
            <List.Item.Meta
              avatar={<LaptopOutlined style={{ fontSize: 22 }} />}
              title={
                <Space>
                  <span>{d.userAgent ? d.userAgent.split(' ').slice(0, 2).join(' ') : '未知设备'}</span>
                  {d.current && <Tag color="blue">当前设备</Tag>}
                  {d.revokedAt && <Tag color="red">已下线</Tag>}
                </Space>
              }
              description={
                <Text type="secondary" style={{ fontSize: 12 }}>
                  IP {d.ip || '-'} · 最近活跃 {d.lastActiveAt ? new Date(d.lastActiveAt).toLocaleString() : '-'} ·
                  过期 {d.expiresAt ? new Date(d.expiresAt).toLocaleString() : '-'}
                </Text>
              }
            />
          </List.Item>
        )}
      />
      <Divider style={{ margin: '12px 0' }} />
      <Text type="secondary" style={{ fontSize: 12 }}>
        当前设备不可下线（请使用「退出登录」）。改密后所有设备将自动下线。
      </Text>
    </Modal>
  )
}
