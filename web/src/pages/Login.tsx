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
  getSystemVersion,
  getTurnstileConfig,
  loginWithPassword,
  queryKeys,
} from '../api/api'

/** 品牌区三大能力点（与门户真实能力对应，不写空话） */
const FEATURES = [
  { icon: <SafetyCertificateOutlined />, name: '统一身份认证', desc: '一次登录，访问全部授权应用' },
  { icon: <AppstoreOutlined />, name: '应用汇聚', desc: '分类、收藏与最近使用' },
  { icon: <CheckSquareOutlined />, name: '待办与邮件', desc: '审批、工单与企业邮件' },
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
 * Velora 登录页：全屏分栏（品牌叙事 / 登录表单），无顶栏。
 * 登录即 SSO：账号密码由 Velora 后端代理 Casdoor OAuth2 password 模式认证
 * （Velora 不实现密码认证，密码仅经 HTTPS 提交给 Casdoor，不落库）。
 */
export default function Login() {
  const me = useMe()
  const [searchParams] = useSearchParams()
  const { message } = AntdApp.useApp()
  const [serverVersion, setServerVersion] = useState('')
  const [submitting, setSubmitting] = useState(false)
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
    // 人机验证启用时强制校验 token（组件回调填充；未通过则按钮禁用）。
    if (turnstileEnabled && !turnstileToken) {
      message.warning('请先完成人机验证')
      return
    }
    setSubmitting(true)
    try {
      const res = await loginWithPassword(values.username, values.password, redirect ?? undefined, turnstileToken || undefined)
      // 整页跳转：让应用重新加载会话（Cookie 已由后端种下）。
      window.location.assign(res.redirect || '/')
    } catch (err) {
      const msg = err instanceof Error ? err.message : '登录失败，请稍后再试'
      message.error(msg)
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
          <p className="velora-login-eyebrow">ENTERPRISE APPLICATION PORTAL</p>
          <h1 className="velora-login-headline">
            一个门户
            <br />
            接入企业全部应用
          </h1>
          <p className="velora-login-sub">统一认证，直达应用、待办与企业邮件。</p>
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
          {portalFooter || `© ${new Date().getFullYear()} ${portalName} · ${portalWelcome}`}
          {serverVersion ? ` · v${serverVersion}` : ''}
        </footer>
      </div>

      {/* 右：登录表单区 */}
      <div className="velora-login-panel">
        <div className="velora-login-panel-inner">
          <h2 className="velora-login-title">登录</h2>
          <p className="velora-login-desc">使用企业统一账号登录</p>

          <Form<{ username: string; password: string }>
            name="login"
            size="large"
            onFinish={onFinish}
            requiredMark={false}
          >
            <Form.Item name="username" rules={[{ required: true, message: '请输入账号' }]}>
              <Input
                prefix={<UserOutlined style={{ color: '#98a2b3' }} />}
                placeholder="账号 / 邮箱"
                autoComplete="username"
                maxLength={64}
              />
            </Form.Item>
            <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password
                prefix={<LockOutlined style={{ color: '#98a2b3' }} />}
                placeholder="密码"
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

          <p className="velora-login-note">无法登录？请联系系统管理员。</p>
        </div>
      </div>
    </div>
  )
}
