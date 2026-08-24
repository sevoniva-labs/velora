import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { ModalForm, ProFormText } from '@ant-design/pro-components'
import { App as AntdApp, Avatar, Button, Drawer, Dropdown, Layout, Menu, type MenuProps } from 'antd'
import { HomeOutlined, LogoutOutlined, MenuFoldOutlined, MenuOutlined, MenuUnfoldOutlined, UserOutlined } from '@ant-design/icons'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { logout, stepUpAuthentication } from '../api/api'
import { ApiError, STEP_UP_REQUIRED_EVENT } from '../api/client'
import { useMe } from '../auth/useMe'
import { adminActiveKey, adminNavItems, type AdminNavItem } from './menu'
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

function groupedMenuItems(items: AdminNavItem[]): MenuProps['items'] {
  return items.map((item) => {
    const children = item.children ?? []
    const entries = children.length ? children : [item]
    return {
      type: 'group' as const,
      key: `group-${item.key}`,
      label: children.length ? item.label : '总览',
      children: entries.map((entry) => ({
        key: entry.key,
        icon: entry.icon ?? item.icon,
        label: entry.path ? <Link className="velora-side-link" to={entry.path}>{entry.label}</Link> : entry.label,
      })),
    }
  })
}

export function AdminLayout({ children }: AdminLayoutProps) {
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
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
  const activeKey = adminActiveKey(location.pathname)
  const visibleItems = useMemo(
    () => visibleNavigation(adminNavItems, me.data?.permissions ?? [], me.data?.roles ?? []),
    [me.data?.permissions, me.data?.roles],
  )
  const menuItems = useMemo(() => groupedMenuItems(visibleItems), [visibleItems])

  useEffect(() => {
    const requireStepUp = () => setStepUpOpen(true)
    window.addEventListener(STEP_UP_REQUIRED_EVENT, requireStepUp)
    return () => window.removeEventListener(STEP_UP_REQUIRED_EVENT, requireStepUp)
  }, [])

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: (result) => { queryClient.clear(); window.location.assign(result.federatedLogoutUrl || '/login') },
    onError: (error) => message.error(error instanceof Error ? error.message : '退出失败，请稍后重试'),
  })

  return (
    <>
      <Layout className={collapsed ? 'velora-layout is-collapsed' : 'velora-layout'}>
        <Layout.Header className="velora-header">
          <Button
            type="text"
            className="velora-header-trigger velora-mobile-menu-trigger"
            aria-label="打开导航"
            icon={<MenuOutlined />}
            onClick={() => setMobileOpen(true)}
          />
          <div className="velora-header-brand">
            <Link className="velora-brand" to="/admin" aria-label="管理后台">
              <span className="velora-brand-mark" aria-hidden="true">
                <img src="/sevoniva-mark.svg" alt="" width={19} height={19} />
              </span>
              <span className="velora-brand-text">
                <span className="velora-brand-name">{portalName}</span>
                <span className="velora-brand-sub">{portalConfig.welcome} · 管理后台</span>
              </span>
            </Link>
          </div>
          <Button
            type="text"
            className="velora-header-trigger velora-sider-trigger"
            aria-label={collapsed ? '展开菜单' : '收起菜单'}
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed((value) => !value)}
          />
          <div className="velora-header-toolbar">
            <Button className="velora-back-portal" icon={<HomeOutlined />} onClick={() => navigate('/home')}>返回门户</Button>
            <span className="velora-header-divider" aria-hidden="true" />
            <Dropdown
              trigger={['click']}
              menu={{ items: [
                { key: 'role', label: roleLabel, disabled: true },
                { type: 'divider' },
                { key: 'logout', label: '退出登录', icon: <LogoutOutlined />, danger: true, onClick: () => logoutMutation.mutate() },
              ] }}
            >
              <button type="button" className="velora-user-chip" aria-label={`当前用户：${displayName}`}>
                <Avatar size={28} icon={<UserOutlined />} />
                <span className="velora-user-chip-info">
                  <span className="velora-user-chip-name">{displayName}</span>
                  <span className="velora-user-chip-role">{roleLabel}</span>
                </span>
              </button>
            </Dropdown>
          </div>
        </Layout.Header>

        <Drawer
          placement="left"
          width={220}
          open={mobileOpen}
          title={`${portalName} 管理后台`}
          styles={{ body: { padding: 0 } }}
          onClose={() => setMobileOpen(false)}
        >
          <Menu
            mode="inline"
            selectedKeys={[activeKey]}
            items={menuItems}
            style={{ borderInlineEnd: 'none' }}
            onClick={() => setMobileOpen(false)}
          />
        </Drawer>

        <Layout hasSider className="velora-body">
          <Layout.Sider
            width={216}
            collapsedWidth={64}
            collapsible
            collapsed={collapsed}
            trigger={null}
            theme="light"
            className="velora-sider"
          >
            <div className="velora-sider-inner">
              <Menu
                mode="inline"
                inlineCollapsed={collapsed}
                selectedKeys={[activeKey]}
                items={menuItems}
                className="velora-side-menu"
              />
            </div>
          </Layout.Sider>
          <Layout.Content id="main" className="velora-main-content">
            <div className="velora-page-content velora-admin-content">{children}</div>
          </Layout.Content>
        </Layout>
      </Layout>

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
            if (error instanceof ApiError && error.code === '200026') message.warning('请先在个人中心启用多因素认证。')
            else message.error('密码或验证码不正确。')
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
