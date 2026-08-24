import { useState } from 'react'
import { App, Button, Dropdown, Input, Modal, Space, Tag, Typography } from 'antd'
import { MoreOutlined } from '@ant-design/icons'
import { DrawerForm, ProDescriptions, ProFormText } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminGetApplicationProvisioningTarget, adminRetryApplicationProvisioning, adminUpsertApplicationProvisioningTarget, queryKeys } from '../../api/api'
import type { Application } from '../../types'
import { formatDateTime } from '../../utils/format'

interface Props { application: Application; canManage: boolean }
interface ProvisioningForm { endpointUrl: string }

const DELIVERY_LABELS: Record<string, string> = { ACTIVE: '正常', HEALTHY: '正常', DEGRADED: '同步异常', FAILED: '同步异常', DISABLED: '未启用', PENDING: '等待同步' }

export default function ApplicationProvisioning({ application, canManage }: Props) {
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const target = useQuery({ queryKey: queryKeys.applicationProvisioningTarget(application.id), queryFn: () => adminGetApplicationProvisioningTarget(application.id) })
  const showSecret = (secret?: string) => { if (secret) Modal.info({ title: '账号同步密钥', content: <Space direction="vertical" style={{ width: '100%' }}><Input.Password value={secret} readOnly visibilityToggle /><Typography.Text type="secondary">密钥仅显示一次</Typography.Text></Space>, okText: '完成', maskClosable: false }) }
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.applicationProvisioningTarget(application.id) })
  const save = useMutation({ mutationFn: (values: ProvisioningForm) => adminUpsertApplicationProvisioningTarget(application.id, { endpointUrl: values.endpointUrl, expectedConfigVersion: target.data?.configVersion, credentialDeliveryMode: 'BROWSER' }), onSuccess: async (result) => { showSecret(result.oneTimeProvisioningSecret); message.success('账号同步已配置'); setOpen(false); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '账号同步配置失败') })
  const rotate = useMutation({ mutationFn: () => adminUpsertApplicationProvisioningTarget(application.id, { endpointUrl: target.data!.endpointUrl, rotateSecret: true, expectedConfigVersion: target.data?.configVersion, credentialDeliveryMode: 'BROWSER' }), onSuccess: async (result) => { showSecret(result.oneTimeProvisioningSecret); message.success('同步密钥已轮换'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '密钥轮换失败') })
  const retry = useMutation({ mutationFn: () => adminRetryApplicationProvisioning(application.id), onSuccess: async (result) => { message.success(result.retriedMessages > 0 ? `已重新提交 ${result.retriedMessages} 项同步任务` : '当前没有需要重试的任务'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '重新同步失败') })
  const rotateConfirm = () => Modal.confirm({ title: '轮换同步密钥？', content: '目标应用需要更新密钥。', okText: '确认轮换', okButtonProps: { danger: true }, onOk: () => rotate.mutateAsync() })
  return <>
    <ProDescriptions column={2} loading={target.isLoading} dataSource={target.data ?? {}} extra={canManage ? <Space>{['DEGRADED', 'FAILED'].includes(target.data?.deliveryStatus ?? '') && <Button type="primary" loading={retry.isPending} onClick={() => retry.mutate()}>重新同步</Button>}<Button type={target.data ? 'default' : 'primary'} onClick={() => setOpen(true)}>{target.data ? '编辑配置' : '配置账号同步'}</Button>{target.data && <Dropdown menu={{ items: [{ key: 'rotate', label: '轮换密钥', danger: true, onClick: rotateConfirm }] }}><Button icon={<MoreOutlined />} aria-label="更多操作" /></Dropdown>}</Space> : undefined} columns={[
      { title: '同步状态', render: () => <Tag color={['DEGRADED', 'FAILED'].includes(target.data?.deliveryStatus ?? '') ? 'error' : target.data?.deliveryStatus === 'PENDING' ? 'processing' : target.data ? 'success' : 'default'}>{DELIVERY_LABELS[target.data?.deliveryStatus ?? 'DISABLED'] ?? '等待同步'}</Tag> },
      { title: '同步地址', render: () => target.data?.endpointUrl || '—' },
      { title: '最近成功', render: () => formatDateTime(target.data?.lastSuccessAt) },
      { title: '最近失败', render: () => formatDateTime(target.data?.lastFailureAt) },
      { title: '失败原因', render: () => target.data?.lastErrorCode ? provisioningErrorLabel(target.data.lastErrorCode) : '—' },
      { title: '密钥版本', render: () => target.data?.activeKeyVersion ? `v${target.data.activeKeyVersion}` : '—' },
      { title: '密钥指纹', render: () => target.data?.secretFingerprint || '—' },
    ]} />
    <DrawerForm<ProvisioningForm> key={`${application.id}-${target.data?.configVersion ?? 0}`} title="账号同步" open={open} onOpenChange={setOpen} width={600} initialValues={{ endpointUrl: target.data?.endpointUrl ?? '' }} submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }} onFinish={async (values) => { await save.mutateAsync(values); return true }}>
      <ProFormText name="endpointUrl" label="同步地址" rules={[{ required: true, message: '请输入同步地址' }, { type: 'url', message: '请输入完整 HTTPS 地址' }, { pattern: /^https:\/\//i, message: '生产环境只允许 HTTPS' }]} />
    </DrawerForm>
  </>
}

function provisioningErrorLabel(code: string): string {
  if (code.startsWith('HTTP_')) return '目标应用返回异常响应'
  if (code === 'NETWORK_ERROR') return '无法连接目标应用'
  if (code === 'INVALID_RESPONSE') return '目标应用响应格式不正确'
  return '账号同步未成功，请检查目标应用后重试'
}
