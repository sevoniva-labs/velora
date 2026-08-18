import { useEffect, useState } from 'react'
import { App as AntdApp, Button, Form, Input, Select, Typography } from 'antd'
import { LockOutlined, SafetyCertificateOutlined, UserOutlined } from '@ant-design/icons'
import { useSearchParams } from 'react-router-dom'
import { getSystemVersion, loginWithPassword, oidcLoginUrl } from '../api/api'

/**
 * Velora 登录页：复用 Spectra Web 的视觉体系（技术蓝分栏版式）。
 * 身份认证由 Casdoor 统一完成 —— 页面提供两种入口：
 *   1. 账号密码直登：后端代理 Casdoor OAuth2 password 模式（推荐，无需跳转）；
 *   2. Sign in with SSO：标准 OIDC 授权码跳转（备选）。
 * Velora 不实现任何密码认证逻辑，密码仅经 HTTPS 提交给 Casdoor。
 */
export default function Login() {
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

  // 未登录访问受保护页面时，携带 redirect 以便登录后跳回。
  const redirect = searchParams.get('redirect')
  const loginHref = oidcLoginUrl(redirect && redirect.startsWith('/') ? redirect : undefined)

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
    <div className="velora-login" style={{ minHeight: '100dvh', background: '#f5f7fa' }}>
      <header className="velora-login-header">
        <span className="velora-login-lockup">Velora</span>
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

      <main
        style={{
          minHeight: 'calc(100dvh - 56px)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '40px 24px',
        }}
      >
        <div className="velora-login-card" style={{ width: 'min(100%, 960px)' }}>
          {/* 左：产品介绍 */}
          <section className="velora-login-intro">
            <p className="velora-login-eyebrow">Enterprise Application Portal</p>
            <h1 className="velora-login-intro-title">
              企业应用门户
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
              <p className="velora-login-form-desc">使用企业统一账号（Casdoor）登录 Velora</p>

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

              <div style={{ display: 'flex', alignItems: 'center', gap: 12, margin: '4px 0 16px' }}>
                <span style={{ flex: 1, height: 1, background: '#e5e8ee' }} aria-hidden="true" />
                <Typography.Text type="secondary" style={{ fontSize: 12.5 }}>
                  或使用单点登录
                </Typography.Text>
                <span style={{ flex: 1, height: 1, background: '#e5e8ee' }} aria-hidden="true" />
              </div>

              <Button size="large" block icon={<SafetyCertificateOutlined />} href={loginHref}>
                Sign in with SSO
              </Button>

              <div style={{ marginTop: 20, fontSize: 12.5, color: '#98a2b3' }}>
                登录即代表您同意企业的应用访问规范。遇到问题请联系系统管理员。
              </div>
            </div>
          </section>
        </div>
      </main>

      <footer className="velora-login-footer">
        Velora · 企业应用门户{serverVersion ? ` · v${serverVersion}` : ''}
      </footer>
    </div>
  )
}
