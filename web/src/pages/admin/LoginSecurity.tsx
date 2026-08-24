import { useState } from 'react'
import { App, Button } from 'antd'
import { ModalForm, PageContainer, ProDescriptions, ProForm, ProFormDigit, ProFormSwitch } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createApproval, getSecurityPolicy, listApprovals, updateSecurityPolicy } from '../../api/admin-platform'
import type { ApprovalRequest, SecurityPolicy } from '../../types'
import { useMe } from '../../auth/useMe'
import { usePageTitle } from '../../hooks/usePageTitle'
import AdminUserSelect from '../../components/AdminUserSelect'
import { SYSTEM_SECURITY_MANAGE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'

type PolicyForm = Omit<SecurityPolicy, 'loginLockDurationSeconds' | 'sessionTtlSeconds'> & { loginLockDurationMinutes: number; sessionTtlMinutes: number; approverId: string }

export default function LoginSecurity() {
  usePageTitle('登录安全')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const me = useMe()
  const canManage = useAdminPermission(SYSTEM_SECURITY_MANAGE)
  const [open, setOpen] = useState(false)
  const policy = useQuery({ queryKey: ['admin', 'security-policy'], queryFn: getSecurityPolicy })
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, enabled: canManage })
  const refresh = async () => Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'security-policy'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] })])
  const request = useMutation({ mutationFn: (values: PolicyForm) => { const { approverId, loginLockDurationMinutes, sessionTtlMinutes, ...rest } = values; const next: SecurityPolicy = { ...rest, loginLockDurationSeconds: loginLockDurationMinutes * 60, sessionTtlSeconds: sessionTtlMinutes * 60 }; return createApproval({ requestType: 'SECURITY_POLICY_CHANGE', action: 'security.config.update', resource: 'security', resourceId: 'policy', summary: '更新登录安全策略', payloadJson: JSON.stringify(policyPayload(next)), approverIds: [approverId] }) }, onSuccess: async () => { message.success('安全策略变更已提交审批'); setOpen(false); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '审批提交失败') })
  const approved = (approvals.data ?? []).find((item) => item.action === 'security.config.update' && item.resourceId === 'policy' && item.status === 'APPROVED')
  const execute = useMutation({ mutationFn: (approval: ApprovalRequest) => updateSecurityPolicy(policyFromPayload(JSON.parse(approval.payloadJson) as Record<string, unknown>), approval.id), onSuccess: async () => { message.success('登录安全策略已生效'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '策略执行失败') })
  const value = policy.data

  const formValue = value ? { ...value, loginLockDurationMinutes: Math.round(value.loginLockDurationSeconds / 60), sessionTtlMinutes: Math.round(value.sessionTtlSeconds / 60) } : undefined
  return <PageContainer title="登录安全" extra={!canManage ? [] : approved ? [<Button key="execute" type="primary" loading={execute.isPending} onClick={() => execute.mutate(approved)}>应用已批准设置</Button>] : [<Button key="edit" type="primary" onClick={() => setOpen(true)}>修改设置</Button>]}>
    <ProDescriptions column={2} loading={policy.isLoading} dataSource={value ?? {}} columns={[
      { title: '密码最小长度', render: () => value ? `${value.passwordMinLength} 位` : '—' },
      { title: '密码复杂度', render: () => value ? complexity(value) : '—' },
      { title: '不可重复使用', render: () => value ? `最近 ${value.passwordHistory} 个密码` : '—' },
      { title: '密码有效期', render: () => value ? `${value.passwordMaxAgeDays} 天` : '—' },
      { title: '登录失败锁定', render: () => value ? `${value.loginMaxFailures} 次后锁定 ${Math.round(value.loginLockDurationSeconds / 60)} 分钟` : '—' },
      { title: '会话有效期', render: () => value ? `${Math.round(value.sessionTtlSeconds / 3600)} 小时` : '—' },
      { title: '最多同时登录', render: () => value ? `${value.maxActiveSessions} 个设备` : '—' },
      { title: '多因素认证', render: () => '用户可在个人中心启用' },
    ]} />
    <ModalForm<PolicyForm> key={JSON.stringify(value)} title="修改登录安全设置" open={open} onOpenChange={setOpen} initialValues={formValue} width={680} grid submitter={{ searchConfig: { submitText: '提交审批', resetText: '取消' } }} onFinish={async (values) => { await request.mutateAsync(values); return true }}>
      <ProFormDigit name="passwordMinLength" label="密码最小长度" min={12} max={128} colProps={{ span: 12 }} rules={[{ required: true }]} />
      <ProFormDigit name="passwordHistory" label="记住最近密码数" min={5} max={50} colProps={{ span: 12 }} rules={[{ required: true }]} />
      <ProFormDigit name="passwordMaxAgeDays" label="密码有效期（天）" min={1} max={90} colProps={{ span: 12 }} rules={[{ required: true }]} />
      <ProFormDigit name="loginMaxFailures" label="连续失败次数" min={1} max={5} colProps={{ span: 12 }} rules={[{ required: true }]} />
      <ProFormDigit name="loginLockDurationMinutes" label="锁定时长（分钟）" min={15} max={1440} colProps={{ span: 12 }} rules={[{ required: true }]} />
      <ProFormDigit name="sessionTtlMinutes" label="登录有效期（分钟）" min={15} max={720} colProps={{ span: 12 }} rules={[{ required: true }]} />
      <ProFormDigit name="maxActiveSessions" label="最多同时登录设备数" min={1} max={20} colProps={{ span: 12 }} rules={[{ required: true }]} />
      <ProForm.Item name="approverId" label="审批人" colProps={{ span: 12 }} rules={[{ required: true, message: '请选择审批人' }]}><AdminUserSelect excludeIds={me.data?.id ? [me.data.id] : []} /></ProForm.Item>
      <ProForm.Group title="密码必须包含">
        <ProFormSwitch name="passwordRequireUpper" label="大写字母" />
        <ProFormSwitch name="passwordRequireLower" label="小写字母" />
        <ProFormSwitch name="passwordRequireDigit" label="数字" />
        <ProFormSwitch name="passwordRequireSymbol" label="特殊字符" />
      </ProForm.Group>
    </ModalForm>
  </PageContainer>
}

function complexity(value: SecurityPolicy): string { return [value.passwordRequireUpper && '大写字母', value.passwordRequireLower && '小写字母', value.passwordRequireDigit && '数字', value.passwordRequireSymbol && '特殊字符'].filter(Boolean).join('、') || '不限制' }
function policyPayload(value: SecurityPolicy): Record<string, unknown> { return { password_min_length: value.passwordMinLength, password_require_upper: value.passwordRequireUpper, password_require_lower: value.passwordRequireLower, password_require_digit: value.passwordRequireDigit, password_require_symbol: value.passwordRequireSymbol, password_history: value.passwordHistory, password_max_age_days: value.passwordMaxAgeDays, login_max_failures: value.loginMaxFailures, login_lock_duration_seconds: value.loginLockDurationSeconds, session_ttl_seconds: value.sessionTtlSeconds, max_active_sessions: value.maxActiveSessions } }
function policyFromPayload(value: Record<string, unknown>): SecurityPolicy { return { passwordMinLength: Number(value.password_min_length), passwordRequireUpper: Boolean(value.password_require_upper), passwordRequireLower: Boolean(value.password_require_lower), passwordRequireDigit: Boolean(value.password_require_digit), passwordRequireSymbol: Boolean(value.password_require_symbol), passwordHistory: Number(value.password_history), passwordMaxAgeDays: Number(value.password_max_age_days), loginMaxFailures: Number(value.login_max_failures), loginLockDurationSeconds: Number(value.login_lock_duration_seconds), sessionTtlSeconds: Number(value.session_ttl_seconds), maxActiveSessions: Number(value.max_active_sessions) } }
