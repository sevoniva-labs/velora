import { useEffect, useState } from 'react'
import { Button, Select } from 'antd'
import { SafetyCertificateOutlined } from '@ant-design/icons'
import { App as AntdApp } from 'antd'
import { useSearchParams } from 'react-router-dom'
import { getSystemVersion, oidcLoginUrl } from '../api/api'

/**
 * Velora 登录页：复用 Spectra Web 的视觉体系（技术蓝分栏版式）。
 * 认证统一由 Casdoor OIDC 完成 —— 页面仅保留「Sign in with SSO」入口。
 */
export default function Login() {
  const [searchParams] = useSearchParams()
  const { message } = AntdApp.useApp()
  const [serverVersion, setServerVersion] = useState('')
  const [locale, setLocale] = useState('zh-CN')

  useEffect(() => {
    void getSystemVersion()
      .then((v) => setServerVersion(v.version || v.application))
      .catch(() => setServerVersion(''))
  }, [])

  // 未登录访问受保护页面时，携带 redirect 以便登录后跳回。
  const redirect = searchParams.get('redirect')
  const loginHref = oidcLoginUrl(redirect && redirect.startsWith('/') ? redirect : undefined)

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

          {/* 右：SSO 登录 */}
          <section className="velora-login-form">
            <div className="velora-login-form-inner">
              <h2 className="velora-login-form-title">欢迎回来</h2>
              <p className="velora-login-form-desc">使用企业统一身份登录 Velora</p>

              <Button
                type="primary"
                size="large"
                block
                icon={<SafetyCertificateOutlined />}
                href={loginHref}
              >
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
