import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { ModalForm, ProFormText, ProLayout, type MenuDataItem } from '@ant-design/pro-components'
import { App as AntdApp, Avatar, Button, Dropdown, Space } from 'antd'
import { HomeOutlined, LogoutOutlined, UserOutlined } from '@ant-design/icons'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { logout, stepUpAuthentication } from '../api/api'
import { ApiError, STEP_UP_REQUIRED_EVENT } from '../api/client'
import { useMe } from '../auth/useMe'
import { adminNavItems, type AdminNavItem } from './menu'
import { hasAnyPermission } from '../auth/permissions'
import { portalConfig } from '../config/portal'

export interface AdminLayoutProps { children: ReactNode }

const ROLE_LABELS: Record<string, string> = {
  system_admin: '系统管理员', application_admin: '应用管理员', iam_admin: '身份管理员', auditor: '审计员', user: '普通用户',
}

export function visibleNavigation(items: AdminNavItem[], permissions: string[], roles: string[]): AdminNavItem[] {
  return items.flatMap((item) => {
    const children = item.children ? visibleNavigation(item.children, permissions, roles) : undefined
    const ownVisible = !item.permissions?.length || hasAnyPermission(permissions, item.permissions, roles)
    if (!ownVisible && !children?.length) return []
    return [{ ...item, children }]
  })
}

function toMenuData(items: AdminNavItem[]): MenuDataItem[] {
  return items.map((item) => ({ key: item.key, path: item.path, name: item.label, icon: item.icon, children: item.children ? toMenuData(item.children) : undefined }))
}

export function AdminLayout({ children }: AdminLayoutProps) {
  const [collapsed, setCollapsed] = useState(false)
  const [stepUpOpen, setStepUpOpen] = useState(false)
  const [recoveryMode, setRecoveryMode] = useState(false)
  const { message } = AntdApp.useApp()
  const location = useLocation()
  const navigate = useNavigate()
  const me = useMe()
  const queryClient = useQueryClient()
  const portalName = portalConfig.name
  const displayName = me.data?.displayName || me.data?.username || '用户'
  const roleLabel = me.data?.roles.map((role) => ROLE_LABELS[role]).find(Boolean) || '管理成员'
  const menuData = useMemo(() => toMenuData(visibleNavigation(adminNavItems, me.data?.permissions ?? [], me.data?.roles ?? [])), [me.data?.permissions, me.data?.roles])

  useEffect(() => {
    const requireStepUp = () => setStepUpOpen(true)
    window.addEventListener(STEP_UP_REQUIRED_EVENT, requireStepUp)
    return () => window.removeEventListener(STEP_UP_REQUIRED_EVENT, requireStepUp)
  }, [])

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: (result) => { queryClient.clear(); window.location.assign(result.federatedLogoutUrl || '/login') },
  })

  return (
    <>
      <ProLayout
      className="velora-admin-layout"
      layout="side"
      navTheme="light"
      title={portalName}
      logo="/sevoniva-mark.svg"
      location={{ pathname: location.pathname }}
      menuDataRender={() => menuData}
      menuItemRender={(item, dom) => item.path ? <Link to={item.path}>{dom}</Link> : dom}
      collapsed={collapsed}
      onCollapse={setCollapsed}
      siderWidth={224}
      contentWidth="Fluid"
      fixedHeader
      fixSiderbar
      breakpoint="lg"
      menu={{ type: 'group', autoClose: false }}
      actionsRender={() => [<Button key="portal" type="text" icon={<HomeOutlined />} onClick={() => navigate('/home')}>返回门户</Button>]}
      avatarProps={{
        src: undefined,
        title: false,
        render: () => <Dropdown trigger={['click']} menu={{ items: [
          { key: 'role', label: roleLabel, disabled: true },
          { type: 'divider' },
          { key: 'logout', label: '退出登录', icon: <LogoutOutlined />, danger: true, onClick: () => logoutMutation.mutate() },
        ] }}><Space className="user-trigger"><Avatar size={28} icon={<UserOutlined />} /><span className="user-name">{displayName}</span></Space></Dropdown>,
      }}
    >
      <main id="main">{children}</main>
      </ProLayout>
      <ModalForm<{ currentPassword: string; mfaCode?: string; recoveryCode?: string }>
        title="确认本人操作"
        open={stepUpOpen}
        modalProps={{ destroyOnHidden: true, maskClosable: false, onCancel: () => setStepUpOpen(false) }}
        submitter={{ searchConfig: { submitText: '确认', resetText: '取消' } }}
        onFinish={async (values) => {
          try {
            await stepUpAuthentication(values.currentPassword, values.mfaCode, values.recoveryCode)
            setStepUpOpen(false)
            message.success('身份确认完成，请重新执行刚才的操作。')
            return true
          } catch (error) {
            if (error instanceof ApiError && error.code === '200026') {
              message.warning('请先在个人中心启用多因素认证。')
            } else {
              message.error('密码或验证码不正确。')
            }
            return false
          }
        }}
      >
        <ProFormText.Password name="currentPassword" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]} fieldProps={{ autoComplete: 'current-password' }} />
        {recoveryMode ? (
          <ProFormText name="recoveryCode" label="恢复码" rules={[{ required: true, message: '请输入恢复码' }]} fieldProps={{ autoComplete: 'one-time-code' }} />
        ) : (
          <ProFormText name="mfaCode" label="验证码" rules={[{ required: true, message: '请输入 6 位验证码' }, { pattern: /^\d{6}$/, message: '请输入 6 位数字验证码' }]} fieldProps={{ inputMode: 'numeric', maxLength: 6, autoComplete: 'one-time-code' }} />
        )}
        <Button type="link" size="small" style={{ paddingInline: 0 }} onClick={() => setRecoveryMode((value) => !value)}>{recoveryMode ? '使用验证码' : '使用恢复码'}</Button>
      </ModalForm>
    </>
  )
}
