import { useMemo, useState } from 'react'
import { App, Button, Modal, Popconfirm, Tag, Typography } from 'antd'
import { KeyOutlined, PlusOutlined } from '@ant-design/icons'
import { ModalForm, PageContainer, ProFormDigit, ProFormSelect, ProFormText, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createIntegrationToken, listIntegrationTokens, revokeIntegrationToken, type IntegrationToken } from '../../api/api'
import { useMe } from '../../auth/useMe'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'

interface CreateForm { name: string; scopes: string[]; expiresInDays: number }

export default function AdminIntegrationTokens() {
  usePageTitle('服务账号')
  const { message } = App.useApp()
  const me = useMe()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [secret, setSecret] = useState<string>()
  const tokens = useQuery({ queryKey: ['admin', 'integration-tokens'], queryFn: listIntegrationTokens })
  const scopeOptions = useMemo(() => {
    const permissions = (me.data?.permissions ?? []).filter((item) => item !== '*').sort()
    const options = permissions.map((value) => ({ value, label: value }))
    return me.data?.roles?.includes('system_admin') ? [{ value: '*', label: '全部权限' }, ...options] : options
  }, [me.data?.permissions, me.data?.roles])
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin', 'integration-tokens'] })
  const create = useMutation({ mutationFn: createIntegrationToken, onSuccess: async (result) => { setSecret(result.token); setOpen(false); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '服务账号创建失败') })
  const revoke = useMutation({ mutationFn: revokeIntegrationToken, onSuccess: async () => { message.success('服务账号已吊销'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '服务账号吊销失败') })
  const columns: ProColumns<IntegrationToken>[] = [
    { title: '名称', dataIndex: 'name' },
    { title: '权限范围', dataIndex: 'scopes', search: false, render: (_, row) => row.scopes.map((scope) => <Tag key={scope}>{scope === '*' ? '全部权限' : scope}</Tag>) },
    { title: '过期时间', dataIndex: 'expiresAt', valueType: 'dateTime', search: false, render: (_, row) => row.expiresAt ? formatDateTime(row.expiresAt) : '永不过期' },
    { title: '最近使用', dataIndex: 'lastUsedAt', valueType: 'dateTime', search: false, render: (_, row) => row.lastUsedAt ? formatDateTime(row.lastUsedAt) : '未使用' },
    { title: '状态', dataIndex: 'revoked', valueType: 'select', valueEnum: { false: { text: '有效' }, true: { text: '已吊销' } }, render: (_, row) => row.revoked ? <Tag>已吊销</Tag> : <Tag color="success">有效</Tag> },
    { title: '操作', valueType: 'option', width: 90, render: (_, row) => row.revoked ? null : <Popconfirm title="吊销此服务账号？" okText="吊销" okButtonProps={{ danger: true }} onConfirm={() => revoke.mutate(row.id)}><Button type="link" danger>吊销</Button></Popconfirm> },
  ]
  return <PageContainer title="服务账号">
    <ProTable<IntegrationToken> rowKey="id" columns={columns} dataSource={tokens.data ?? []} loading={tokens.isLoading} search={{ labelWidth: 'auto' }} pagination={false} toolBarRender={() => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>新建服务账号</Button>]} />
    <ModalForm<CreateForm> title="新建服务账号" open={open} onOpenChange={setOpen} initialValues={{ expiresInDays: 90 }} submitter={{ searchConfig: { submitText: '创建', resetText: '取消' } }} onFinish={async (values) => { await create.mutateAsync(values); return true }}>
      <ProFormText name="name" label="名称" fieldProps={{ prefix: <KeyOutlined />, maxLength: 100 }} rules={[{ required: true, message: '请输入名称' }]} />
      <ProFormSelect name="scopes" label="权限范围" mode="multiple" options={scopeOptions} rules={[{ required: true, message: '请选择权限范围' }]} />
      <ProFormDigit name="expiresInDays" label="有效期（天）" min={1} max={365} rules={[{ required: true, message: '请输入有效期' }]} />
    </ModalForm>
    <Modal title="保存密钥" open={Boolean(secret)} onCancel={() => setSecret(undefined)} onOk={() => setSecret(undefined)} cancelButtonProps={{ style: { display: 'none' } }} okText="已保存" maskClosable={false} closable={false}>
      <Typography.Text code copyable>{secret}</Typography.Text>
    </Modal>
  </PageContainer>
}
