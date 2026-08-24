import { useEffect, useMemo, useRef, useState } from 'react'
import { App, Button, Modal, Popconfirm, Space, Statistic, Tag, Typography } from 'antd'
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'
import { EditableProTable, ModalForm, ProCard, ProForm, ProFormCheckbox, ProFormDateTimePicker, ProFormSelect, ProFormText, ProTable, type ProColumns, type ProFormInstance } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { adminListApplicationRoles, adminReplaceApplicationRoles, type ApplicationRole } from '../../api/api'
import { createApproval, listApprovals, listApplicationAccessGrants, listApplicationEffectiveAccess, listDepartments, listPlatformRoles, listUserGroups, previewApplicationAccessGrants, replaceApplicationAccessGrants } from '../../api/admin-platform'
import type { ApplicationAccessGrant, ApplicationAccessImpact, ApplicationEffectiveAccess, ApprovalRequest } from '../../types'
import { useMe } from '../../auth/useMe'
import AdminUserSelect from '../../components/AdminUserSelect'
import { APPROVAL_REQUEST_CREATE, APPROVAL_REQUEST_READ, hasPermission } from '../../auth/permissions'
import { useClientTableSearch } from '../../utils/tableSearch'

const SUBJECT_LABELS: Record<ApplicationAccessGrant['subjectType'], string> = { EVERYONE: '全体成员', DEPARTMENT: '部门', USER_GROUP: '用户组', PLATFORM_ROLE: '平台角色', USER: '指定人员' }
const RISK_LABELS = { NORMAL: '普通', PRIVILEGED: '高权限', CRITICAL: '关键权限' }

interface Props { applicationId: string; view: 'roles' | 'access' }

export default function ApplicationAccess({ applicationId, view }: Props) {
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const me = useMe()
  const grantFormRef = useRef<ProFormInstance<ApplicationAccessGrant> | undefined>(undefined)
  const canReadApprovals = hasPermission(me.data?.permissions, APPROVAL_REQUEST_READ, me.data?.roles)
  const canRequestApproval = hasPermission(me.data?.permissions, APPROVAL_REQUEST_CREATE, me.data?.roles)
  const [drafts, setDrafts] = useState<ApplicationAccessGrant[]>([])
  const [dirty, setDirty] = useState(false)
  const [editing, setEditing] = useState<ApplicationAccessGrant>()
  const [grantOpen, setGrantOpen] = useState(false)
  const [subjectType, setSubjectType] = useState<ApplicationAccessGrant['subjectType']>('DEPARTMENT')
  const [effect, setEffect] = useState<ApplicationAccessGrant['effect']>('ALLOW')
  const [preview, setPreview] = useState<ApplicationAccessImpact>()
  const [roleDrafts, setRoleDrafts] = useState<ApplicationRole[]>([])
  const [roleDirty, setRoleDirty] = useState(false)
  const [approverId, setApproverId] = useState<string>()
  const [selectedSubjectName, setSelectedSubjectName] = useState('')
  const grants = useQuery({ queryKey: ['admin', 'applications', applicationId, 'access-grants'], queryFn: () => listApplicationAccessGrants(applicationId) })
  const effective = useQuery({ queryKey: ['admin', 'applications', applicationId, 'effective-access'], queryFn: () => listApplicationEffectiveAccess(applicationId) })
  const effectiveTable = useClientTableSearch(effective.data ?? [])
  const roles = useQuery({ queryKey: ['admin', 'applications', applicationId, 'roles'], queryFn: () => adminListApplicationRoles(applicationId) })
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments })
  const groups = useQuery({ queryKey: ['admin', 'user-groups'], queryFn: listUserGroups })
  const platformRoles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, enabled: canReadApprovals })
  useEffect(() => { if (grants.data && !dirty) setDrafts(grants.data) }, [grants.data, dirty])
  useEffect(() => { if (roles.data && !roleDirty) setRoleDrafts(roles.data) }, [roleDirty, roles.data])

  const subjectOptions = useMemo(() => {
    if (subjectType === 'DEPARTMENT') return (departments.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))
    if (subjectType === 'USER_GROUP') return (groups.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))
    if (subjectType === 'PLATFORM_ROLE') return (platformRoles.data ?? []).map((item) => ({ label: item.name, value: item.key }))
    return []
  }, [departments.data, groups.data, platformRoles.data, subjectType])
  const activeRoles = (roles.data ?? []).filter((item) => item.status === 'ACTIVE')
  const roleMutation = useMutation({ mutationFn: (values: readonly ApplicationRole[]) => adminReplaceApplicationRoles(applicationId, values.map(({ roleKey, name, description, riskLevel, status }) => ({ roleKey, name, description, riskLevel, status }))), onSuccess: async () => { message.success('应用角色已保存'); setRoleDirty(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'applications', applicationId, 'roles'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '应用角色保存失败') })
  const previewMutation = useMutation({ mutationFn: () => previewApplicationAccessGrants(applicationId, drafts), onSuccess: (data) => setPreview(data.impact), onError: (error) => message.error(error instanceof Error ? error.message : '影响预览失败') })
  const refreshAccess = async () => { await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'applications', applicationId, 'access-grants'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'applications', applicationId, 'effective-access'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] })]) }
  const saveMutation = useMutation({ mutationFn: (input?: { grants: ApplicationAccessGrant[]; approvalId?: string }) => replaceApplicationAccessGrants(applicationId, input?.grants ?? drafts, input?.approvalId), onSuccess: async () => { message.success('使用范围已生效'); setPreview(undefined); setDirty(false); await refreshAccess() }, onError: (error) => message.error(error instanceof Error ? error.message : '使用范围保存失败') })
  const requestApproval = useMutation({ mutationFn: () => createApproval({ requestType: 'APPLICATION_ACCESS_CHANGE', action: 'portal.application.access_grants.replace', resource: 'portal_application', resourceId: applicationId, summary: '变更应用使用范围', payloadJson: JSON.stringify(accessApprovalPayload(applicationId, drafts)), approverIds: [approverId!] }), onSuccess: async () => { message.success('使用范围变更已提交审批'); setPreview(undefined); setDirty(false); setApproverId(undefined); await queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '审批提交失败') })
  const approvedChanges = (approvals.data ?? []).filter((item) => item.action === 'portal.application.access_grants.replace' && item.resourceId === applicationId && item.status === 'APPROVED')
  const executeApproved = (approval: ApprovalRequest) => {
    try { saveMutation.mutate({ grants: accessGrantsFromApproval(approval.payloadJson), approvalId: approval.id }) } catch { message.error('审批内容无法解析，请重新提交变更') }
  }

  const saveGrant = (values: ApplicationAccessGrant) => {
    const name = values.subjectType === 'EVERYONE' ? '全体成员' : values.subjectType === 'USER' ? selectedSubjectName || editing?.subjectName || '' : subjectOptions.find((item) => item.value === values.subjectId)?.label ?? editing?.subjectName ?? ''
    const item = { ...values, id: editing?.id || crypto.randomUUID(), applicationId, subjectId: values.subjectType === 'EVERYONE' ? '' : values.subjectId, subjectName: name, includeDescendants: values.subjectType === 'DEPARTMENT' && Boolean(values.includeDescendants), roles: values.effect === 'EXCLUDE' ? [] : values.roles ?? [], validFrom: normalizeTimestamp(values.validFrom), validUntil: normalizeTimestamp(values.validUntil), status: values.status || 'ACTIVE', reason: values.reason || '', version: editing?.version ?? 0 }
    setDrafts((current) => editing ? current.map((row) => row.id === editing.id ? item : row) : [...current, item])
    setDirty(true)
    setGrantOpen(false)
    return true
  }
  const grantColumns: ProColumns<ApplicationAccessGrant>[] = [
    { title: '适用对象', dataIndex: 'subjectName', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.subjectName || SUBJECT_LABELS[row.subjectType]}</Typography.Text><Typography.Text type="secondary">{SUBJECT_LABELS[row.subjectType]}{row.includeDescendants ? ' · 含下级部门' : ''}</Typography.Text></Space> },
    { title: '应用角色', dataIndex: 'roles', render: (_, row) => row.effect === 'EXCLUDE' ? <Tag color="error">排除</Tag> : row.roles.length ? row.roles.map((key) => <Tag key={key}>{activeRoles.find((role) => role.roleKey === key)?.name ?? key}</Tag>) : <Tag>仅访问</Tag> },
    { title: '状态', dataIndex: 'status', render: (_, row) => row.status === 'ACTIVE' ? <Tag color="success">可使用</Tag> : <Tag>已停用</Tag> },
    { title: '操作', valueType: 'option', width: 120, render: (_, row) => <Space><Button type="text" icon={<EditOutlined />} aria-label="编辑" onClick={() => { setEditing(row); setSelectedSubjectName(row.subjectName); setSubjectType(row.subjectType); setEffect(row.effect); setGrantOpen(true) }} /><Popconfirm title="移除此使用范围？" onConfirm={() => { setDrafts((current) => current.filter((item) => item.id !== row.id)); setDirty(true) }}><Button type="text" danger icon={<DeleteOutlined />} aria-label="移除" /></Popconfirm></Space> },
  ]
  const effectiveColumns: ProColumns<ApplicationEffectiveAccess>[] = [
    { title: '用户', dataIndex: 'displayName', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.displayName || row.loginName}</Typography.Text><Typography.Text type="secondary">{row.loginName}</Typography.Text></Space> },
    { title: '应用角色', dataIndex: 'roles', render: (_, row) => row.roles.length ? row.roles.map((role) => <Tag key={role}>{activeRoles.find((item) => item.roleKey === role)?.name ?? role}</Tag>) : '仅访问' },
    { title: '获得方式', dataIndex: 'sourceGrantIds', render: (_, row) => row.sourceGrantIds.map((id) => drafts.find((grant) => grant.id === id)?.subjectName).filter(Boolean).join('、') || '—' },
  ]
  const roleColumns: ProColumns<ApplicationRole>[] = [
    { title: '角色编码', dataIndex: 'roleKey', formItemProps: { rules: [{ required: true, pattern: /^[a-z][a-z0-9_-]{1,99}$/, message: '使用小写字母、数字、短横线或下划线' }] }, editable: (_, row) => row.configVersion > 0 ? false : true },
    { title: '角色名称', dataIndex: 'name', formItemProps: { rules: [{ required: true, message: '请输入角色名称' }] } },
    { title: '权限风险', dataIndex: 'riskLevel', valueType: 'select', valueEnum: { NORMAL: { text: RISK_LABELS.NORMAL }, PRIVILEGED: { text: RISK_LABELS.PRIVILEGED }, CRITICAL: { text: RISK_LABELS.CRITICAL } } },
    { title: '说明', dataIndex: 'description' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '启用' }, DISABLED: { text: '停用' } } },
    { title: '操作', valueType: 'option' },
  ]

  return <Space direction="vertical" size={16} style={{ width: '100%' }}>
    {view === 'roles' && <ProCard className="velora-admin-section-card" title="应用角色" extra={<Button type="primary" disabled={!roleDirty} loading={roleMutation.isPending} onClick={() => roleMutation.mutate(roleDrafts)}>保存角色</Button>}>
      <EditableProTable<ApplicationRole> rowKey="id" columns={roleColumns} value={roleDrafts} loading={roles.isLoading} recordCreatorProps={{ newRecordType: 'dataSource', record: () => ({ id: crypto.randomUUID(), applicationId, roleKey: '', name: '', description: '', riskLevel: 'NORMAL', status: 'ACTIVE', configVersion: 0 }) }} editable={{ type: 'multiple' }} onChange={(values) => { setRoleDrafts([...values]); setRoleDirty(true) }} controlled={false} />
    </ProCard>}
    {view === 'access' && <>
    <ProTable<ApplicationAccessGrant> headerTitle="谁可以使用" rowKey="id" columns={grantColumns} dataSource={drafts} loading={grants.isLoading} search={false} pagination={false} options={false} toolBarRender={() => [dirty ? <Tag key="dirty" color="warning">待保存</Tag> : null, <Button key="add" icon={<PlusOutlined />} onClick={() => { setEditing(undefined); setSelectedSubjectName(''); setSubjectType('DEPARTMENT'); setEffect('ALLOW'); setGrantOpen(true) }}>添加使用范围</Button>, <Button key="save" type="primary" disabled={!dirty} loading={previewMutation.isPending} onClick={() => previewMutation.mutate()}>预览并保存</Button>].filter(Boolean)} />
    {approvedChanges.length > 0 && <ProTable<ApprovalRequest> headerTitle="待执行变更" rowKey="id" dataSource={approvedChanges} search={false} pagination={false} options={false} columns={[{ title: '事项', dataIndex: 'summary' }, { title: '操作', valueType: 'option', width: 110, render: (_, row) => <Button type="link" loading={saveMutation.isPending} onClick={() => executeApproved(row)}>执行变更</Button> }]} />}
    <ProTable<ApplicationEffectiveAccess> className="velora-admin-secondary-table" headerTitle="当前可使用人员" rowKey="userId" columns={effectiveColumns} {...effectiveTable} loading={effective.isLoading} search={{ labelWidth: 'auto' }} pagination={{ pageSize: 20 }} />

    <ModalForm<ApplicationAccessGrant> formRef={grantFormRef} key={editing?.id ?? 'new'} title={editing ? '编辑使用范围' : '添加使用范围'} open={grantOpen} onOpenChange={setGrantOpen} width={560} initialValues={editing ? { ...editing, validFrom: editing.validFrom ? dayjs(editing.validFrom) : undefined, validUntil: editing.validUntil ? dayjs(editing.validUntil) : undefined } : { subjectType: 'DEPARTMENT', effect: 'ALLOW', roles: [], status: 'ACTIVE', includeDescendants: true }} submitter={{ searchConfig: { submitText: '确定', resetText: '取消' } }} onFinish={async (values) => saveGrant(values)}>
      <ProFormSelect name="subjectType" label="按什么范围" options={Object.entries(SUBJECT_LABELS).map(([value, label]) => ({ value, label }))} fieldProps={{ onChange: (value) => { setSubjectType(value as ApplicationAccessGrant['subjectType']); grantFormRef.current?.setFieldValue('subjectId', undefined); setSelectedSubjectName('') } }} rules={[{ required: true }]} />
      {subjectType === 'USER' && <ProForm.Item name="subjectId" label="选择人员" rules={[{ required: true, message: '请选择人员' }]}><AdminUserSelect onUserSelect={(user) => setSelectedSubjectName(user.displayName || user.loginName)} /></ProForm.Item>}
      {subjectType !== 'EVERYONE' && subjectType !== 'USER' && <ProFormSelect name="subjectId" label="选择对象" showSearch fieldProps={{ optionFilterProp: 'label' }} options={subjectOptions} rules={[{ required: true, message: '请选择对象' }]} />}
      {subjectType === 'DEPARTMENT' && <ProFormCheckbox name="includeDescendants">包含下级部门</ProFormCheckbox>}
      <ProFormSelect name="effect" label="设置方式" options={[{ label: '允许使用', value: 'ALLOW' }, { label: '排除人员', value: 'EXCLUDE' }]} fieldProps={{ onChange: (value) => setEffect(value as ApplicationAccessGrant['effect']) }} rules={[{ required: true }]} />
      {effect === 'ALLOW' && <ProFormSelect name="roles" label="应用角色" mode="multiple" options={activeRoles.map((role) => ({ label: role.name, value: role.roleKey }))} />}
      <ProFormDateTimePicker name="validFrom" label="生效时间" />
      <ProFormDateTimePicker name="validUntil" label="失效时间" fieldProps={{ disabledDate: (date) => date.isBefore(dayjs(), 'day') }} />
      <ProFormSelect name="status" label="状态" options={[{ label: '启用', value: 'ACTIVE' }, { label: '停用', value: 'DISABLED' }]} rules={[{ required: true }]} />
      <ProFormText name="reason" label="变更原因" fieldProps={{ maxLength: 200 }} />
    </ModalForm>
    <Modal title="确认使用范围" open={Boolean(preview)} onCancel={() => { setPreview(undefined); setApproverId(undefined) }} okText={preview && requiresApproval(preview) ? '提交审批' : '确认生效'} cancelText="返回修改" confirmLoading={saveMutation.isPending || requestApproval.isPending} okButtonProps={{ disabled: Boolean(preview && requiresApproval(preview) && (!canRequestApproval || !approverId)) }} onOk={() => preview && requiresApproval(preview) ? requestApproval.mutate() : saveMutation.mutate(undefined)}>
      <ProCard ghost gutter={12} wrap>{preview && <><ProCard><Statistic title="可使用人数" value={preview.effectiveUsers} /></ProCard><ProCard><Statistic title="新增人员" value={preview.addedUsers} /></ProCard><ProCard><Statistic title="移除人员" value={preview.revokedUsers} /></ProCard><ProCard><Statistic title="角色变化" value={preview.roleChangedUsers} /></ProCard></>}</ProCard>
      {preview && requiresApproval(preview) && <Space direction="vertical" style={{ width: '100%', marginTop: 16 }}><Typography.Text type="warning">{canRequestApproval ? '该变更需要审批。' : '该变更需要审批，请联系有审批申请权限的管理员。'}</Typography.Text>{canRequestApproval && <AdminUserSelect value={approverId} onChange={(value) => setApproverId(Array.isArray(value) ? value[0] : value)} excludeIds={me.data?.id ? [me.data.id] : []} placeholder="选择审批人" />}</Space>}
    </Modal>
    </>}
  </Space>
}

function requiresApproval(impact: ApplicationAccessImpact): boolean { return impact.privilegedUsers > 0 || impact.provisioningTasks >= 50 }

function accessApprovalPayload(applicationId: string, grants: ApplicationAccessGrant[]) {
  return { application_id: applicationId, grants: grants.map((item) => ({ id: item.id, application_id: applicationId, subject_type: item.subjectType, subject_id: item.subjectId, include_descendants: item.includeDescendants, effect: item.effect, roles: item.roles, ...(normalizeTimestamp(item.validFrom) ? { valid_from: normalizeTimestamp(item.validFrom) } : {}), ...(normalizeTimestamp(item.validUntil) ? { valid_until: normalizeTimestamp(item.validUntil) } : {}), status: item.status, reason: item.reason, version: item.version })) }
}

function accessGrantsFromApproval(payloadJson: string): ApplicationAccessGrant[] {
  const payload = JSON.parse(payloadJson) as { grants: Array<Record<string, unknown>> }
  return payload.grants.map((item) => ({ id: String(item.id ?? ''), applicationId: String(item.application_id ?? ''), subjectType: String(item.subject_type) as ApplicationAccessGrant['subjectType'], subjectId: String(item.subject_id ?? ''), subjectName: '', includeDescendants: Boolean(item.include_descendants), effect: String(item.effect) as ApplicationAccessGrant['effect'], roles: Array.isArray(item.roles) ? item.roles.map(String) : [], validFrom: item.valid_from ? String(item.valid_from) : undefined, validUntil: item.valid_until ? String(item.valid_until) : undefined, status: String(item.status || 'ACTIVE') as ApplicationAccessGrant['status'], reason: String(item.reason ?? ''), version: Number(item.version ?? 0) }))
}

function normalizeTimestamp(value: unknown): string | undefined {
  if (!value) return undefined
  const date = dayjs.isDayjs(value) ? value.toDate() : new Date(String(value))
  if (Number.isNaN(date.getTime())) return undefined
  return date.toISOString().replace(/\.000Z$/, 'Z').replace(/(\.\d*?[1-9])0+Z$/, '$1Z')
}
