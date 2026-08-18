import { useEffect, useMemo, useState } from 'react'
import { usePageTitle } from '../../hooks/usePageTitle'
import AdminPageHead from '../../components/AdminPageHead'
import { App as AntdApp, Button, Card, Empty, Form, Input, Select, Table, Tag } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  adminListApplications,
  adminSetPolicies,
  queryKeys,
  type AdminApplicationInput,
} from '../../api/api'
import type { AccessPolicy, Application, PolicyType } from '../../types'
import { POLICY_TYPE_LABEL } from '../../labels'

const POLICY_OPTIONS: { value: PolicyType; label: string }[] = [
  { value: 'EVERYONE', label: '所有人（所有登录用户）' },
  { value: 'ORGANIZATION', label: '指定组织' },
  { value: 'ROLE', label: '指定角色' },
  { value: 'GROUP', label: '指定用户组' },
  { value: 'USER', label: '指定用户' },
]

interface PolicyRow {
  key: string
  policyType: PolicyType
  value: string
}

export default function AdminPolicies() {
  usePageTitle('访问策略')

  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [selectedAppId, setSelectedAppId] = useState<number>()
  const [rows, setRows] = useState<PolicyRow[]>([])
  const [form] = Form.useForm<{ policyType: PolicyType; value: string }>()

  const { data: pageData, isLoading: loadingApps } = useQuery({
    queryKey: queryKeys.adminApplications({ all: true }),
    queryFn: () => adminListApplications({ page: 1, pageSize: 100 }),
  })

  // 选中应用后加载其当前策略（管理员视图 DTO 含 policies）。
  const selectedApp = useMemo(
    () => pageData?.items.find((a) => a.id === selectedAppId),
    [pageData, selectedAppId],
  )

  useEffect(() => {
    if (selectedApp) {
      setRows(
        (selectedApp.policies ?? []).map((p, i) => ({
          key: `${i}`,
          policyType: p.policyType,
          value: p.value,
        })),
      )
    } else {
      setRows([])
    }
  }, [selectedApp])

  const saveMutation = useMutation({
    mutationFn: (policies: AccessPolicy[]) => adminSetPolicies(selectedAppId!, policies),
    onSuccess: () => {
      message.success('访问策略已保存，即时生效')
      void queryClient.invalidateQueries({ queryKey: queryKeys.applications() })
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '保存失败'),
  })

  const addRow = async () => {
    const values = await form.validateFields()
    setRows((prev) => [...prev, { key: `new-${Date.now()}`, ...values }])
    form.resetFields()
  }

  const save = () => {
    if (!selectedAppId) return
    saveMutation.mutate(
      rows.map((r) => ({ policyType: r.policyType, value: r.value })),
    )
  }

  return (
    <div>
      <AdminPageHead title="访问策略" />

      <Card>
        <Select
          style={{ width: '100%', maxWidth: 420 }}
          placeholder="选择要配置的应用"
          loading={loadingApps}
          showSearch
          optionFilterProp="label"
          value={selectedAppId}
          onChange={(v) => setSelectedAppId(v)}
          options={(pageData?.items ?? []).map((a) => ({ value: a.id, label: `${a.name}（${a.code}）` }))}
        />

        {!selectedApp ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="请先选择应用"
            style={{ padding: '48px 0' }}
          />
        ) : (
          <>
            <div style={{ marginTop: 20, display: 'flex', gap: 8, alignItems: 'flex-end' }}>
              <Form form={form} layout="vertical" style={{ flex: 1, display: 'flex', gap: 12 }} requiredMark={false}>
                <Form.Item label="策略类型" name="policyType" style={{ flex: 1.2 }} rules={[{ required: true, message: '请选择类型' }]}>
                  <Select placeholder="选择策略类型" options={POLICY_OPTIONS} />
                </Form.Item>
                <Form.Item
                  label="取值"
                  name="value"
                  style={{ flex: 1.6 }}
                  rules={[
                    ({ getFieldValue }) => ({
                      validator: (_, value) => {
                        const type = getFieldValue('policyType') as PolicyType | undefined
                        if (!type || type === 'EVERYONE') return Promise.resolve()
                        if (!value || !String(value).trim()) return Promise.reject(new Error('请输入取值'))
                        return Promise.resolve()
                      },
                    }),
                  ]}
                >
                  <Input placeholder="组织名 / 角色名 / 用户组名 / 用户 ID" />
                </Form.Item>
              </Form>
              <Button type="dashed" icon={<PlusOutlined />} onClick={() => void addRow()} style={{ marginBottom: 20 }}>
                添加规则
              </Button>
            </div>

            <Table<PolicyRow>
              rowKey="key"
              dataSource={rows}
              pagination={false}
              size="small"
              locale={{ emptyText: '暂无规则，所有登录用户可见' }}
              columns={[
                {
                  title: '策略类型',
                  dataIndex: 'policyType',
                  width: 200,
                  render: (t: PolicyType) => <Tag color="blue">{POLICY_TYPE_LABEL[t]}</Tag>,
                },
                { title: '取值', dataIndex: 'value', render: (v: string) => v || '—' },
                {
                  title: '操作',
                  key: 'actions',
                  width: 80,
                  render: (_, row) => (
                    <Button
                      type="link"
                      size="small"
                      danger
                      onClick={() => setRows((prev) => prev.filter((r) => r.key !== row.key))}
                    >
                      移除
                    </Button>
                  ),
                },
              ]}
            />

            <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
              <Button type="primary" loading={saveMutation.isPending} onClick={save}>
                保存策略
              </Button>
            </div>
          </>
        )}
      </Card>
    </div>
  )
}

// 保持类型引用（AdminApplicationInput 仅用于编译期契约）。
export type { AdminApplicationInput as _AdminAppInput }
export type { Application as _Application }
