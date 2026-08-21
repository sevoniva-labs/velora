import { useEffect, useState } from 'react'
import { App as AntdApp, Button, Form, Input } from 'antd'
import {
  AppstoreOutlined,
  CheckSquareOutlined,
  LockOutlined,
  MailOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { useSearchParams, Navigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useMe } from '../auth/useMe'
import TurnstileWidget from '../components/TurnstileWidget'
import {
  getPortalSettings,
  getAuthCapabilities,
  getTurnstileConfig,
  beginOIDCLogin,
  loginWithPassword,
  queryKeys,
} from '../api/api'
import type { AuthCapabilities } from '../api/api'

/** 品牌区能力说明：只描述用户能实际使用的功能，不使用宣传式表述。 */
const FEATURES = [
  { icon: <SafetyCertificateOutlined />, name: '统一登录', desc: '使用企业账号访问授权应用' },
  { icon: <AppstoreOutlined />, name: '集中管理', desc: '按分类、收藏和最近使用查找应用' },
  { icon: <SafetyCertificateOutlined />, name: '权限控制', desc: '仅展示当前账号已获授权的应用' },
]

/** 品牌区装饰磁贴：呼应门户应用宫格，纯视觉（色调同应用图标体系） */
const DECO_TILES = [
  { icon: <AppstoreOutlined />, x: '60%', y: '14%', s: 56, tone: '#1677FF', d: '0s', dur: '7.5s', o: 0.9 },
  { icon: <MailOutlined />, x: '76%', y: '24%', s: 68, tone: '#13C2C2', d: '1.2s', dur: '8.5s', o: 0.95 },
  { icon: <CheckSquareOutlined />, x: '63%', y: '40%', s: 48, tone: '#FA8C16', d: '0.6s', dur: '7s', o: 0.85 },
  { icon: <SafetyCertificateOutlined />, x: '82%', y: '48%', s: 52, tone: '#722ED1', d: '1.8s', dur: '9s', o: 0.8 },
  { icon: <UserOutlined />, x: '70%', y: '64%', s: 44, tone: '#2d7cf0', d: '2.4s', dur: '8s', o: 0.7 },
]

/**
 * Velora 登录页：全屏分栏（品牌叙事 / 登录入口），无顶栏。
 * 登录方式由后端公开能力动态决定。认证服务的实现细节不在用户界面暴露。
 */
export default function Login() {
  const me = useMe()
  const [searchParams] = useSearchParams()
  const { message } = AntdApp.useApp()
  const [submitting, setSubmitting] = useState(false)
  const [authCapabilities, setAuthCapabilities] = useState<AuthCapabilities | null>(null)
  // Cloudflare Turnstile 人机验证（登录防 bot；后端配置启用后必须通过验证才能登录）
  const [turnstileToken, setTurnstileToken] = useState('')
  // Turnstile token 一次性有效：登录失败/过期后递增 key 强制重挂载 widget 获取新 token。
  const [turnstileAttempt, setTurnstileAttempt] = useState(0)
  const { data: turnstile } = useQuery({
    queryKey: ['turnstile-config'],
    queryFn: getTurnstileConfig,
    retry: false,
    // 不设 staleTime：登录页每次刷新实时读取，配置变更（启用/停用人机验证）立即生效。
  })
  const turnstileEnabled = turnstile?.enabled && !!turnstile.siteKey

  useEffect(() => {
    void getAuthCapabilities().then(setAuthCapabilities).catch(() => {
      // 能力发现失败时默认使用统一身份登录，不在页面展示密码表单。
      setAuthCapabilities({ authMode: 'oidc', passwordLoginEnabled: false, casdoorAccountUrl: '' })
    })
  }, [])

  // 门户展示配置：名称 / 欢迎语 / 页脚（未登录也可读，后端为公开只读接口）。
  const { data: settings } = useQuery({ queryKey: queryKeys.portalSettings, queryFn: getPortalSettings, retry: false })
  const valueOf = (key: string) => settings?.find((s) => s.key === key)?.value ?? ''
  const portalName = valueOf('portal_name') || 'Velora'
  const portalWelcome = valueOf('portal_welcome') || '企业应用门户'
  const portalFooter = valueOf('portal_footer') || ''

  // 未登录访问受保护页面时，携带 redirect 以便登录后跳回。
  const redirect = searchParams.get('redirect')
  const oidcOnly = !authCapabilities || !authCapabilities.passwordLoginEnabled

  const loginErrorMessage = (error: unknown, fallback: string): string => {
    const raw = error instanceof Error ? error.message.trim() : ''
    const normalized = raw.toLowerCase()
    if (!raw) return fallback
    if (normalized.includes('oidc') || normalized.includes('provider') || normalized.includes('identity')) {
      return '统一登录暂不可用，请联系系统管理员。'
    }
    if (
      normalized.includes('credential') ||
      normalized.includes('password') ||
      normalized.includes('unauthorized') ||
      normalized.includes('invalid') ||
      normalized.includes('账号') ||
      normalized.includes('密码')
    ) {
      return '账号或密码错误，请重试。'
    }
    return fallback
  }

  const onOIDCLogin = async () => {
    setSubmitting(true)
    try {
      const target = await beginOIDCLogin(redirect ?? undefined)
      window.location.assign(target)
    } catch (err) {
      message.error(loginErrorMessage(err, '统一登录暂不可用，请稍后重试。'))
      setSubmitting(false)
    }
  }

  const onFinish = async (values: { username: string; password: string }) => {
    // 人机验证启用时强制校验 token（组件回调填充；未通过则按钮禁用）。
    if (turnstileEnabled && !turnstileToken) {
      message.warning('请完成人机验证后继续。')
      return
    }
    setSubmitting(true)
    try {
      const res = await loginWithPassword(values.username, values.password, redirect ?? undefined, turnstileToken || undefined)
      // 整页跳转：让应用重新加载会话（Cookie 已由后端种下）。
      window.location.assign(res.redirect || '/')
    } catch (err) {
      message.error(loginErrorMessage(err, '登录失败，请稍后重试。'))
      setSubmitting(false)
      // 人机验证 token 已被消费：重置 widget，下次提交需重新验证。
      setTurnstileToken('')
      setTurnstileAttempt((n) => n + 1)
    }
  }

  // 已登录用户直接进入工作台（置于所有 hooks 之后）。
  if (me.data) {
    return <Navigate to="/home" replace />
  }

  return (
    <div className="velora-login">
      {/* 左：品牌叙事区 */}
      <div className="velora-login-brand">
        {/* 装饰层：色晕 + 浮动应用磁贴 */}
        <div className="velora-login-deco" aria-hidden="true">
          <span className="velora-login-orb velora-login-orb--1" />
          <span className="velora-login-orb velora-login-orb--2" />
          <span className="velora-login-orb velora-login-orb--3" />
          {DECO_TILES.map((t, i) => (
            <span
              key={i}
              className="velora-login-tile"
              style={{
                left: t.x,
                top: t.y,
                width: t.s,
                height: t.s,
                color: t.tone,
                opacity: t.o,
                fontSize: Math.round(t.s * 0.42),
                animationDelay: t.d,
                animationDuration: t.dur,
              }}
            >
              {t.icon}
            </span>
          ))}
        </div>

        <header className="velora-login-brand-head">
          <span className="velora-brand-mark" aria-hidden="true">
            <img src="/sevoniva-mark.svg" alt="" width={30} height={30} />
          </span>
          <span className="velora-login-brand-name">{portalName}</span>
          <span className="velora-login-brand-sep" />
          <span className="velora-login-brand-sub">{portalWelcome}</span>
        </header>

        <div className="velora-login-brand-body">
          <p className="velora-login-eyebrow">企业应用门户</p>
          <h1 className="velora-login-headline">
            统一访问
            <br />
            企业应用
          </h1>
          <p className="velora-login-sub">登录后访问当前账号已获授权的应用与服务。</p>
          <ul className="velora-login-features">
            {FEATURES.map((f) => (
              <li key={f.name} className="velora-login-feature">
                <span className="velora-login-feature-icon">{f.icon}</span>
                <span className="velora-login-feature-name">{f.name}</span>
                <span className="velora-login-feature-desc">{f.desc}</span>
              </li>
            ))}
          </ul>
        </div>

        <footer className="velora-login-brand-foot">
          {portalFooter || `© ${new Date().getFullYear()} ${portalName}`}
        </footer>
      </div>

      {/* 右：登录入口区 */}
      <div className="velora-login-panel">
        <div className="velora-login-panel-inner">
          <h2 className="velora-login-title">登录</h2>
          <p className="velora-login-desc">使用企业账号登录</p>

          {oidcOnly ? (
            <>
              <Button type="primary" block loading={submitting} onClick={() => void onOIDCLogin()} className="velora-login-submit">
                使用统一身份登录
              </Button>
              <p className="velora-login-note">登录验证由企业身份中心完成。</p>
            </>
          ) : (
            <Form<{ username: string; password: string }>
              name="login"
              size="large"
              onFinish={onFinish}
              requiredMark={false}
            >
              <Form.Item label="账号" name="username" rules={[{ required: true, message: '请输入账号或邮箱' }]}>
                <Input
                  prefix={<UserOutlined style={{ color: '#98a2b3' }} />}
                  placeholder="请输入账号或邮箱"
                  autoComplete="username"
                  maxLength={64}
                />
              </Form.Item>
              <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
                <Input.Password
                  prefix={<LockOutlined style={{ color: '#98a2b3' }} />}
                  placeholder="请输入密码"
                  autoComplete="current-password"
                  maxLength={128}
                />
              </Form.Item>
              <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
                <Button
                  type="primary"
                  htmlType="submit"
                  block
                  loading={submitting}
                  disabled={turnstileEnabled && !turnstileToken}
                  className="velora-login-submit"
                >
                  登录
                </Button>
              </Form.Item>
              {turnstileEnabled && (
                <div style={{ marginTop: 16, marginBottom: 4 }}>
                  <TurnstileWidget
                    key={turnstileAttempt}
                    siteKey={turnstile.siteKey}
                    onVerify={setTurnstileToken}
                    onExpire={() => setTurnstileToken('')}
                  />
                </div>
              )}
            </Form>
          )}

          {!oidcOnly && <p className="velora-login-note">如无法登录，请联系系统管理员。</p>}
        </div>
      </div>
    </div>
  )
}
