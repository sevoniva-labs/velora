import { useCallback, useState } from 'react'
import { Alert, Avatar, Button, Card, Descriptions, Divider, Form, Input, List, Modal, QRCode, Space, Tag, Typography, message } from 'antd'
import { KeyOutlined, LaptopOutlined, LogoutOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { ModalForm, ProForm, ProFormSelect, ProFormText } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  changePassword,
  beginMFAEnrollment,
  confirmMFAEnrollment,
  disableMFA,
  getAuthCapabilities,
  getMFAStatus,
  getUserProfile,
  updateUserProfile,
  type UserProfileInput,
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
  const queryClient = useQueryClient()
  const [deviceModalOpen, setDeviceModalOpen] = useState(false)
  const [beginMFAOpen, setBeginMFAOpen] = useState(false)
  const [disableMFAOpen, setDisableMFAOpen] = useState(false)
  const [profileOpen, setProfileOpen] = useState(false)
  const [enrollment, setEnrollment] = useState<{ secret: string; provisioningUri: string }>()
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])

  const { data: profile } = useQuery({ queryKey: ['user-center', 'profile'], queryFn: getUserProfile })
  const { data: authCapabilities } = useQuery({ queryKey: ['auth', 'capabilities'], queryFn: getAuthCapabilities })
  const mfaStatus = useQuery({ queryKey: ['auth', 'mfa'], queryFn: getMFAStatus })
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
  const beginMFA = useMutation({ mutationFn: beginMFAEnrollment, onSuccess: (data) => { setEnrollment(data); setBeginMFAOpen(false) }, onError: (error: Error) => msg.error(error.message || '无法开始设置多因素认证') })
  const confirmMFA = useMutation({ mutationFn: confirmMFAEnrollment, onSuccess: async (data) => { setEnrollment(undefined); setRecoveryCodes(data.recoveryCodes ?? []); await queryClient.invalidateQueries({ queryKey: ['auth', 'mfa'] }); msg.success('多因素认证已启用') }, onError: (error: Error) => msg.error(error.message || '验证码不正确') })
  const disableMFAMutation = useMutation({ mutationFn: (values: { currentPassword: string; code?: string; recoveryCode?: string }) => disableMFA(values.currentPassword, values.code, values.recoveryCode), onSuccess: async () => { setDisableMFAOpen(false); await queryClient.invalidateQueries({ queryKey: ['auth', 'mfa'] }); msg.success('多因素认证已关闭') }, onError: (error: Error) => msg.error(error.message || '无法关闭多因素认证') })
  const updateProfileMutation = useMutation({ mutationFn: updateUserProfile, onSuccess: async () => { setProfileOpen(false); await Promise.all([queryClient.invalidateQueries({ queryKey: ['user-center', 'profile'] }), queryClient.invalidateQueries({ queryKey: ['me'] })]); msg.success('个人资料已更新') }, onError: (error: Error) => msg.error(error.message || '个人资料保存失败') })

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
      <Card title="个人资料" extra={<Button onClick={() => setProfileOpen(true)}>编辑资料</Button>} style={{ marginBottom: 16 }}>
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
          <Descriptions.Item label="显示名称">{profile?.displayName || '-'}</Descriptions.Item>
          <Descriptions.Item label="真实姓名">{profile?.realName || '-'}</Descriptions.Item>
          <Descriptions.Item label="性别">{{ MALE: '男', FEMALE: '女', UNSPECIFIED: '未设置' }[profile?.gender ?? 'UNSPECIFIED']}</Descriptions.Item>
          <Descriptions.Item label="手机号">{profile?.phone ? <Space>{profile.phoneCountryCode} {profile.phone}<Tag color={profile.phoneVerifiedAt ? 'success' : 'default'}>{profile.phoneVerifiedAt ? '已验证' : '未验证'}</Tag></Space> : '-'}</Descriptions.Item>
          <Descriptions.Item label="邮箱">{profile?.email ? <Space>{profile.email}<Tag color={profile.emailVerifiedAt ? 'success' : 'default'}>{profile.emailVerifiedAt ? '已验证' : '未验证'}</Tag></Space> : '-'}</Descriptions.Item>
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

      <ModalForm<UserProfileInput> title="编辑个人资料" open={profileOpen} onOpenChange={setProfileOpen} initialValues={{ displayName: profile?.displayName ?? '', realName: profile?.realName ?? '', gender: profile?.gender ?? 'UNSPECIFIED', phoneCountryCode: profile?.phoneCountryCode || '+86', phone: profile?.phone ?? '', email: profile?.email ?? '', avatarUrl: profile?.avatar ?? '', expectedVersion: profile?.profileVersion ?? 0 }} submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }} onFinish={async (values) => { await updateProfileMutation.mutateAsync({ ...values, expectedVersion: profile?.profileVersion ?? 0 }); return true }}>
        <ProFormText name="displayName" label="显示名称" rules={[{ required: true, message: '请输入显示名称' }, { max: 200 }]} />
        <ProFormText name="realName" label="真实姓名" rules={[{ max: 200 }]} />
        <ProFormSelect name="gender" label="性别" options={[{ label: '未设置', value: 'UNSPECIFIED' }, { label: '男', value: 'MALE' }, { label: '女', value: 'FEMALE' }]} />
        <ProForm.Group>
          <ProFormSelect name="phoneCountryCode" label="国家/地区代码" width="xs" options={[{ label: '中国大陆 +86', value: '+86' }]} />
          <ProFormText name="phone" label="手机号" width="md" fieldProps={{ inputMode: 'tel', maxLength: 20 }} rules={[{ pattern: /^\d{6,20}$/, message: '请输入6–20位数字' }]} />
        </ProForm.Group>
        <ProFormText name="email" label="邮箱" fieldProps={{ type: 'email' }} rules={[{ type: 'email', message: '请输入有效邮箱' }, { max: 320 }]} />
        <ProFormText name="avatarUrl" label="头像地址" fieldProps={{ type: 'url' }} rules={[{ type: 'url', message: '请输入 HTTPS 地址' }]} />
      </ModalForm>

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

      <Card title="多因素认证" style={{ marginBottom: 16 }} extra={mfaStatus.data ? <Tag color="success">已启用</Tag> : <Tag>未启用</Tag>}>
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Text type="secondary">使用身份验证器生成的动态验证码保护账号。启用后登录和敏感操作需要验证码。</Text>
          {mfaStatus.data ? <Button danger onClick={() => setDisableMFAOpen(true)}>关闭多因素认证</Button> : <Button type="primary" icon={<SafetyCertificateOutlined />} loading={mfaStatus.isLoading} onClick={() => setBeginMFAOpen(true)}>启用多因素认证</Button>}
        </Space>
      </Card>

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

      <Modal title="启用多因素认证" open={beginMFAOpen} onCancel={() => setBeginMFAOpen(false)} footer={null} destroyOnHidden>
        <Form layout="vertical" onFinish={(values: { currentPassword: string }) => beginMFA.mutate(values.currentPassword)}>
          <Form.Item name="currentPassword" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}><Input.Password autoComplete="current-password" /></Form.Item>
          <Button type="primary" htmlType="submit" block loading={beginMFA.isPending}>继续</Button>
        </Form>
      </Modal>
      <Modal title="绑定身份验证器" open={Boolean(enrollment)} onCancel={() => setEnrollment(undefined)} footer={null} destroyOnHidden maskClosable={false}>
        {enrollment && <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Text>使用身份验证器扫描二维码，然后输入生成的 6 位验证码。</Text>
          <div style={{ display: 'flex', justifyContent: 'center' }}><QRCode value={enrollment.provisioningUri} /></div>
          <Typography.Text copyable code>{enrollment.secret}</Typography.Text>
          <Form layout="vertical" onFinish={(values: { code: string }) => confirmMFA.mutate(values.code)}>
            <Form.Item name="code" label="验证码" rules={[{ required: true, pattern: /^\d{6}$/, message: '请输入 6 位数字验证码' }]}><Input inputMode="numeric" autoComplete="one-time-code" maxLength={6} /></Form.Item>
            <Button type="primary" htmlType="submit" block loading={confirmMFA.isPending}>确认启用</Button>
          </Form>
        </Space>}
      </Modal>
      <Modal title="保存恢复码" open={recoveryCodes.length > 0} onCancel={() => undefined} footer={<Button type="primary" onClick={() => setRecoveryCodes([])}>我已妥善保存</Button>} closable={false} maskClosable={false}>
        <Alert type="warning" showIcon message="恢复码只显示一次" description="请保存在安全位置。每个恢复码只能使用一次。" style={{ marginBottom: 16 }} />
        <Typography.Paragraph copyable={{ text: recoveryCodes.join('\n') }}><pre style={{ whiteSpace: 'pre-wrap' }}>{recoveryCodes.join('\n')}</pre></Typography.Paragraph>
      </Modal>
      <Modal title="关闭多因素认证" open={disableMFAOpen} onCancel={() => setDisableMFAOpen(false)} footer={null} destroyOnHidden>
        <Form layout="vertical" onFinish={(values: { currentPassword: string; code?: string; recoveryCode?: string }) => { if (!values.code && !values.recoveryCode) { msg.warning('请输入验证码或恢复码'); return }; disableMFAMutation.mutate(values) }}>
          <Form.Item name="currentPassword" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}><Input.Password autoComplete="current-password" /></Form.Item>
          <Form.Item name="code" label="验证码"><Input inputMode="numeric" autoComplete="one-time-code" maxLength={6} /></Form.Item>
          <Divider plain>或</Divider>
          <Form.Item name="recoveryCode" label="恢复码"><Input autoComplete="one-time-code" /></Form.Item>
          <Button danger type="primary" htmlType="submit" block loading={disableMFAMutation.isPending}>确认关闭</Button>
        </Form>
      </Modal>
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
