import { useEffect, useState } from 'react'
import { App as AntdApp, Button, Form, Input, Select } from 'antd'
import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Navigate } from 'react-router-dom'
import { useMe } from '../auth/useMe'
import { getPortalSettings, getSystemVersion, loginWithPassword, queryKeys } from '../api/api'

/**
 * Velora 登录页：复用 Spectra Web 的视觉体系（技术蓝分栏版式）。
 * 登录即 SSO：账号密码由 Velora 后端代理 Casdoor OAuth2 password 模式认证
 * （Velora 不实现密码认证，密码仅经 HTTPS 提交给 Casdoor，不落库）。
 */
export default function Login() {
  const me = useMe()
  // 已登录用户直接进入工作台。
  if (me.data) {
    return <Navigate to="/home" replace />
  }
  const [searchParams] = useSearchParams()
  const { message } = AntdApp.useApp()
  const [serverVersion, setServerVersion] = useState('')
  const [locale, setLocale] = useState('zh-CN')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    void getSystemVersion()
      .then((v) => setServerVersion(v.version || v.application))
      .catch(() => setServerVersion(''))
  }, [])

  // 门户展示配置：名称 / 欢迎语 / 页脚（未登录也可读，后端为公开只读接口）。
  const { data: settings } = useQuery({ queryKey: queryKeys.portalSettings, queryFn: getPortalSettings, retry: false })
  const valueOf = (key: string) => settings?.find((s) => s.key === key)?.value ?? ''
  const portalName = valueOf('portal_name') || 'Velora'
  const portalWelcome = valueOf('portal_welcome') || '企业应用门户'
  const portalFooter = valueOf('portal_footer') || ''

  // 未登录访问受保护页面时，携带 redirect 以便登录后跳回。
  const redirect = searchParams.get('redirect')

  const onFinish = async (values: { username: string; password: string }) => {
    setSubmitting(true)
    try {
      const res = await loginWithPassword(values.username, values.password, redirect ?? undefined)
      // 整页跳转：让应用重新加载会话（Cookie 已由后端种下）。
      window.location.assign(res.redirect || '/')
    } catch (err) {
      const msg = err instanceof Error ? err.message : '登录失败，请稍后再试'
      message.error(msg)
      setSubmitting(false)
    }
  }

  const highlights = [
    { name: '统一入口', desc: '一个门户，汇聚企业全部数字化应用', tone: '#1677FF' },
    { name: '一次登录', desc: 'Casdoor 统一身份认证（OIDC + PKCE）', tone: '#13C2C2' },
    { name: '智能分类', desc: '搜索 / 分类 / 收藏 / 最近使用', tone: '#FA8C16' },
    { name: '安全可控', desc: '应用级访问策略，前端隐藏 + 后端强制校验', tone: '#722ED1' },
  ]

  return (
    <div className="velora-login">
      <header className="velora-login-header">
        <span className="velora-login-lockup">
          <span className="velora-brand-mark" aria-hidden="true">
            <img src="/logo-mark.svg" alt="" width={28} height={28} />
          </span>
          {portalName} {portalWelcome}
        </span>
        <Select
          aria-label="语言"
          size="small"
          variant="borderless"
          className="velora-locale-select"
          value={locale}
          popupMatchSelectWidth={false}
          onChange={(value) => {
            if (value === 'zh-CN') setLocale('zh-CN')
            else {
              message.info('英文界面暂未提供，已保持中文')
              setLocale('zh-CN')
            }
          }}
          options={[
            { value: 'zh-CN', label: '中文' },
            { value: 'en-US', label: 'EN' },
          ]}
        />
      </header>

      <main className="velora-login-main">
        <div className="velora-login-card">
          {/* 左：产品介绍 */}
          <section className="velora-login-intro">
            <p className="velora-login-eyebrow">Enterprise Application Portal</p>
            <h1 className="velora-login-intro-title">
              {portalWelcome}
              <br />
              统一工作台
            </h1>
            <p className="velora-login-intro-copy">
              一个门户，一次登录，访问您有权使用的全部企业应用。Casdoor 统一身份认证，应用级访问控制。
            </p>
            <ol className="velora-login-workflow">
              {highlights.map((item) => (
                <li key={item.name} className="velora-login-workflow-item">
                  <span className="velora-login-workflow-dot" style={{ background: item.tone }} aria-hidden="true" />
                  <span className="velora-login-workflow-name">{item.name}</span>
                  <span className="velora-login-workflow-desc">{item.desc}</span>
                </li>
              ))}
            </ol>
          </section>

          {/* 右：账号密码登录 */}
          <section className="velora-login-form">
            <div className="velora-login-form-inner">
              <h2 className="velora-login-form-title">欢迎回来</h2>
              <p className="velora-login-form-desc">使用企业统一账号（Casdoor）登录 {portalName}</p>

              <Form<{ username: string; password: string }>
                name="login"
                size="large"
                onFinish={onFinish}
                requiredMark={false}
              >
                <Form.Item
                  name="username"
                  rules={[{ required: true, message: '请输入账号' }]}
                >
                  <Input
                    prefix={<UserOutlined style={{ color: '#98a2b3' }} />}
                    placeholder="账号 / 邮箱"
                    autoComplete="username"
                    maxLength={64}
                  />
                </Form.Item>
                <Form.Item
                  name="password"
                  rules={[{ required: true, message: '请输入密码' }]}
                >
                  <Input.Password
                    prefix={<LockOutlined style={{ color: '#98a2b3' }} />}
                    placeholder="密码"
                    autoComplete="current-password"
                    maxLength={128}
                  />
                </Form.Item>
                <Form.Item style={{ marginBottom: 12 }}>
                  <Button type="primary" htmlType="submit" block loading={submitting}>
                    登 录
                  </Button>
                </Form.Item>
              </Form>

              <div className="velora-login-note">
                登录即代表您同意企业的应用访问规范。遇到问题请联系系统管理员。
              </div>
            </div>
          </section>
        </div>
      </main>

      <footer className="velora-login-footer">
        {portalFooter || `${portalName} · ${portalWelcome}`}
        {serverVersion ? ` · v${serverVersion}` : ''}
      </footer>
    </div>
  )
}
