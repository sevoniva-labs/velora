import { useEffect, useMemo, useState } from 'react'
import { App, Button, Modal, Popconfirm, Space, Statistic, Tag, Typography } from 'antd'
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import { EditableProTable, ModalForm, ProCard, ProFormCheckbox, ProFormSelect, ProFormText, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminListApplicationRoles, adminListUsers, adminReplaceApplicationRoles, type ApplicationRole } from '../../api/api'
import { listApplicationAccessGrants, listApplicationEffectiveAccess, listDepartments, listPlatformRoles, listUserGroups, previewApplicationAccessGrants, replaceApplicationAccessGrants } from '../../api/admin-platform'
import type { ApplicationAccessGrant, ApplicationAccessImpact, ApplicationEffectiveAccess } from '../../types'

const SUBJECT_LABELS: Record<ApplicationAccessGrant['subjectType'], string> = { EVERYONE: '全体成员', DEPARTMENT: '部门', USER_GROUP: '用户组', PLATFORM_ROLE: '平台角色', USER: '指定人员' }
const RISK_LABELS = { NORMAL: '普通', PRIVILEGED: '高权限', CRITICAL: '关键权限' }

interface Props { applicationId: string }

export default function ApplicationAccess({ applicationId }: Props) {
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [drafts, setDrafts] = useState<ApplicationAccessGrant[]>([])
  const [dirty, setDirty] = useState(false)
  const [editing, setEditing] = useState<ApplicationAccessGrant>()
  const [grantOpen, setGrantOpen] = useState(false)
  const [subjectType, setSubjectType] = useState<ApplicationAccessGrant['subjectType']>('DEPARTMENT')
  const [effect, setEffect] = useState<ApplicationAccessGrant['effect']>('ALLOW')
  const [preview, setPreview] = useState<ApplicationAccessImpact>()
  const [roleDrafts, setRoleDrafts] = useState<ApplicationRole[]>([])
  const [roleDirty, setRoleDirty] = useState(false)
  const grants = useQuery({ queryKey: ['admin', 'applications', applicationId, 'access-grants'], queryFn: () => listApplicationAccessGrants(applicationId) })
  const effective = useQuery({ queryKey: ['admin', 'applications', applicationId, 'effective-access'], queryFn: () => listApplicationEffectiveAccess(applicationId) })
  const roles = useQuery({ queryKey: ['admin', 'applications', applicationId, 'roles'], queryFn: () => adminListApplicationRoles(applicationId) })
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments })
  const groups = useQuery({ queryKey: ['admin', 'user-groups'], queryFn: listUserGroups })
  const platformRoles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const users = useQuery({ queryKey: ['admin', 'users'], queryFn: adminListUsers })
  useEffect(() => { if (grants.data && !dirty) setDrafts(grants.data) }, [grants.data, dirty])
  useEffect(() => { if (roles.data && !roleDirty) setRoleDrafts(roles.data) }, [roleDirty, roles.data])

  const subjectOptions = useMemo(() => {
    if (subjectType === 'DEPARTMENT') return (departments.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))
    if (subjectType === 'USER_GROUP') return (groups.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))
    if (subjectType === 'PLATFORM_ROLE') return (platformRoles.data ?? []).map((item) => ({ label: item.name, value: item.key }))
    if (subjectType === 'USER') return (users.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: `${item.displayName || item.loginName}（${item.loginName}）`, value: item.id }))
    return []
  }, [departments.data, groups.data, platformRoles.data, subjectType, users.data])
  const activeRoles = (roles.data ?? []).filter((item) => item.status === 'ACTIVE')
  const roleMutation = useMutation({ mutationFn: (values: readonly ApplicationRole[]) => adminReplaceApplicationRoles(applicationId, values.map(({ roleKey, name, description, riskLevel, status }) => ({ roleKey, name, description, riskLevel, status }))), onSuccess: async () => { message.success('应用角色已保存'); setRoleDirty(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'applications', applicationId, 'roles'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '应用角色保存失败') })
  const previewMutation = useMutation({ mutationFn: () => previewApplicationAccessGrants(applicationId, drafts), onSuccess: (data) => setPreview(data.impact), onError: (error) => message.error(error instanceof Error ? error.message : '影响预览失败') })
  const saveMutation = useMutation({ mutationFn: () => replaceApplicationAccessGrants(applicationId, drafts), onSuccess: async () => { message.success('访问范围已生效'); setPreview(undefined); setDirty(false); await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'applications', applicationId, 'access-grants'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'applications', applicationId, 'effective-access'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })]) }, onError: (error) => message.error(error instanceof Error ? error.message : '访问范围保存失败') })

  const saveGrant = (values: ApplicationAccessGrant) => {
    const name = values.subjectType === 'EVERYONE' ? '全体成员' : subjectOptions.find((item) => item.value === values.subjectId)?.label ?? editing?.subjectName ?? ''
    const item = { ...values, id: editing?.id || crypto.randomUUID(), applicationId, subjectId: values.subjectType === 'EVERYONE' ? '' : values.subjectId, subjectName: name, includeDescendants: values.subjectType === 'DEPARTMENT' && Boolean(values.includeDescendants), roles: values.effect === 'EXCLUDE' ? [] : values.roles ?? [], status: values.status || 'ACTIVE', reason: values.reason || '', version: editing?.version ?? 0 }
    setDrafts((current) => editing ? current.map((row) => row.id === editing.id ? item : row) : [...current, item])
    setDirty(true)
    setGrantOpen(false)
    return true
  }
  const grantColumns: ProColumns<ApplicationAccessGrant>[] = [
    { title: '访问对象', dataIndex: 'subjectName', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.subjectName || SUBJECT_LABELS[row.subjectType]}</Typography.Text><Typography.Text type="secondary">{SUBJECT_LABELS[row.subjectType]}{row.includeDescendants ? ' · 含下级部门' : ''}</Typography.Text></Space> },
    { title: '应用角色', dataIndex: 'roles', render: (_, row) => row.effect === 'EXCLUDE' ? <Tag color="error">排除</Tag> : row.roles.length ? row.roles.map((key) => <Tag key={key}>{activeRoles.find((role) => role.roleKey === key)?.name ?? key}</Tag>) : <Tag>仅访问</Tag> },
    { title: '状态', dataIndex: 'status', render: (_, row) => row.status === 'ACTIVE' ? <Tag color="success">生效中</Tag> : <Tag>已停用</Tag> },
    { title: '操作', valueType: 'option', width: 120, render: (_, row) => <Space><Button type="text" icon={<EditOutlined />} aria-label="编辑" onClick={() => { setEditing(row); setSubjectType(row.subjectType); setEffect(row.effect); setGrantOpen(true) }} /><Popconfirm title="移除此访问规则？" onConfirm={() => { setDrafts((current) => current.filter((item) => item.id !== row.id)); setDirty(true) }}><Button type="text" danger icon={<DeleteOutlined />} aria-label="移除" /></Popconfirm></Space> },
  ]
  const effectiveColumns: ProColumns<ApplicationEffectiveAccess>[] = [
    { title: '用户', dataIndex: 'displayName', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.displayName || row.loginName}</Typography.Text><Typography.Text type="secondary">{row.loginName}</Typography.Text></Space> },
    { title: '应用角色', dataIndex: 'roles', render: (_, row) => row.roles.length ? row.roles.map((role) => <Tag key={role}>{activeRoles.find((item) => item.roleKey === role)?.name ?? role}</Tag>) : '仅访问' },
    { title: '权限来源', dataIndex: 'sourceGrantIds', render: (_, row) => row.sourceGrantIds.map((id) => drafts.find((grant) => grant.id === id)?.subjectName).filter(Boolean).join('、') || '—' },
  ]
  const roleColumns: ProColumns<ApplicationRole>[] = [
    { title: '角色编码', dataIndex: 'roleKey', formItemProps: { rules: [{ required: true, pattern: /^[a-z][a-z0-9_-]{1,99}$/, message: '使用小写字母、数字、短横线或下划线' }] }, editable: (_, row) => row.configVersion > 0 ? false : true },
    { title: '角色名称', dataIndex: 'name', formItemProps: { rules: [{ required: true, message: '请输入角色名称' }] } },
    { title: '权限级别', dataIndex: 'riskLevel', valueType: 'select', valueEnum: { NORMAL: { text: RISK_LABELS.NORMAL }, PRIVILEGED: { text: RISK_LABELS.PRIVILEGED }, CRITICAL: { text: RISK_LABELS.CRITICAL } } },
    { title: '说明', dataIndex: 'description' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '启用' }, DISABLED: { text: '停用' } } },
    { title: '操作', valueType: 'option' },
  ]

  return <Space direction="vertical" size={16} style={{ width: '100%' }}>
    <ProCard title="应用角色" extra={<Button type="primary" disabled={!roleDirty} loading={roleMutation.isPending} onClick={() => roleMutation.mutate(roleDrafts)}>保存角色</Button>}>
      <EditableProTable<ApplicationRole> rowKey="id" columns={roleColumns} value={roleDrafts} loading={roles.isLoading} recordCreatorProps={{ newRecordType: 'dataSource', record: () => ({ id: crypto.randomUUID(), applicationId, roleKey: '', name: '', description: '', riskLevel: 'NORMAL', status: 'ACTIVE', configVersion: 0 }) }} editable={{ type: 'multiple' }} onChange={(values) => { setRoleDrafts([...values]); setRoleDirty(true) }} controlled={false} />
    </ProCard>
    <ProTable<ApplicationAccessGrant> headerTitle="访问范围" rowKey="id" columns={grantColumns} dataSource={drafts} loading={grants.isLoading} search={false} pagination={false} options={false} toolBarRender={() => [dirty ? <Tag key="dirty" color="warning">待保存</Tag> : null, <Button key="add" icon={<PlusOutlined />} onClick={() => { setEditing(undefined); setSubjectType('DEPARTMENT'); setEffect('ALLOW'); setGrantOpen(true) }}>添加规则</Button>, <Button key="save" type="primary" disabled={!dirty} loading={previewMutation.isPending} onClick={() => previewMutation.mutate()}>预览并保存</Button>].filter(Boolean)} />
    <ProTable<ApplicationEffectiveAccess> headerTitle="有效权限" rowKey="userId" columns={effectiveColumns} dataSource={effective.data ?? []} loading={effective.isLoading} search={{ labelWidth: 'auto' }} pagination={{ pageSize: 20 }} />

    <ModalForm<ApplicationAccessGrant> key={editing?.id ?? 'new'} title={editing ? '编辑访问规则' : '添加访问规则'} open={grantOpen} onOpenChange={setGrantOpen} width={560} initialValues={editing ?? { subjectType: 'DEPARTMENT', effect: 'ALLOW', roles: [], status: 'ACTIVE', includeDescendants: true }} submitter={{ searchConfig: { submitText: '确定', resetText: '取消' } }} onFinish={async (values) => saveGrant(values)}>
      <ProFormSelect name="subjectType" label="访问对象类型" options={Object.entries(SUBJECT_LABELS).map(([value, label]) => ({ value, label }))} fieldProps={{ onChange: (value) => setSubjectType(value as ApplicationAccessGrant['subjectType']) }} rules={[{ required: true }]} />
      {subjectType !== 'EVERYONE' && <ProFormSelect name="subjectId" label="访问对象" showSearch fieldProps={{ optionFilterProp: 'label' }} options={subjectOptions} rules={[{ required: true, message: '请选择访问对象' }]} />}
      {subjectType === 'DEPARTMENT' && <ProFormCheckbox name="includeDescendants">包含下级部门</ProFormCheckbox>}
      <ProFormSelect name="effect" label="授权方式" options={[{ label: '允许访问', value: 'ALLOW' }, { label: '排除人员', value: 'EXCLUDE' }]} fieldProps={{ onChange: (value) => setEffect(value as ApplicationAccessGrant['effect']) }} rules={[{ required: true }]} />
      {effect === 'ALLOW' && <ProFormSelect name="roles" label="应用角色" mode="multiple" options={activeRoles.map((role) => ({ label: role.name, value: role.roleKey }))} />}
      <ProFormSelect name="status" label="状态" options={[{ label: '启用', value: 'ACTIVE' }, { label: '停用', value: 'DISABLED' }]} rules={[{ required: true }]} />
      <ProFormText name="reason" label="变更原因" fieldProps={{ maxLength: 200 }} />
    </ModalForm>
    <Modal title="访问变更" open={Boolean(preview)} onCancel={() => setPreview(undefined)} okText="确认生效" cancelText="返回修改" confirmLoading={saveMutation.isPending} onOk={() => saveMutation.mutate()}>
      <ProCard ghost gutter={12} wrap>{preview && <><ProCard><Statistic title="生效用户" value={preview.effectiveUsers} /></ProCard><ProCard><Statistic title="新增" value={preview.addedUsers} /></ProCard><ProCard><Statistic title="撤销" value={preview.revokedUsers} /></ProCard><ProCard><Statistic title="角色变化" value={preview.roleChangedUsers} /></ProCard></>}</ProCard>
      {preview?.privilegedUsers ? <Typography.Text type="warning">涉及 {preview.privilegedUsers} 名高权限用户，需要审批并完成多因素认证。</Typography.Text> : null}
    </Modal>
  </Space>
}
