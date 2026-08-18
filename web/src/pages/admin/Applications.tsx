import { useState } from 'react'
import { usePageTitle } from '../../hooks/usePageTitle'
import { isSafeHttpUrl } from '../../utils/format'
import { App as AntdApp, Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Table, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  adminCreateApplication,
  adminDeleteApplication,
  adminListApplications,
  adminUpdateApplication,
  listCategories,
  listTags,
  queryKeys,
  type AdminApplicationInput,
} from '../../api/api'
import type { Application } from '../../types'
import { APP_STATUS_LABEL, SSO_TYPE_COLOR, SSO_TYPE_LABEL } from '../../labels'
import { AppIcon } from '../../components/AppCard'

const SSO_OPTIONS = ['URL', 'OIDC', 'SAML', 'CAS', 'FORWARD_AUTH']

export default function AdminApplications() {
  usePageTitle('应用管理')

  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Application | null>(null)
  const [form] = Form.useForm<AdminApplicationInput>()
  const watchSsoType = Form.useWatch('ssoType', form)

  const { data: categories } = useQuery({ queryKey: queryKeys.categories, queryFn: listCategories })
  const { data: tags } = useQuery({ queryKey: queryKeys.tags, queryFn: listTags })

  const { data: pageData, isLoading } = useQuery({
    queryKey: queryKeys.adminApplications({ page, keyword }),
    queryFn: () => adminListApplications({ page, pageSize: 20, keyword: keyword || undefined }),
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['admin', 'applications'] })
    void queryClient.invalidateQueries({ queryKey: ['applications'] })
  }

  const saveMutation = useMutation({
    mutationFn: (input: AdminApplicationInput) =>
      editing ? adminUpdateApplication(editing.id, input) : adminCreateApplication(input),
    onSuccess: () => {
      message.success(editing ? '应用已更新' : '应用已创建')
      setModalOpen(false)
      setEditing(null)
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '保存失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => adminDeleteApplication(id),
    onSuccess: () => {
      message.success('应用已删除')
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '删除失败'),
  })

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      ssoType: 'URL',
      status: 'ENABLED',
      sort: 0,
      isFeatured: false,
      healthCheckEnabled: false,
      tagIds: [],
    })
    setModalOpen(true)
  }

  const openEdit = (app: Application) => {
    setEditing(app)
    form.setFieldsValue({
      code: app.code,
      name: app.name,
      description: app.description,
      keywords: app.keywords,
      icon: app.icon,
      categoryId: app.categoryId,
      homeUrl: app.homeUrl,
      launchUrl: app.launchUrl,
      ssoType: app.ssoType,
      casdoorApplicationName: app.casdoorApplicationName,
      casdoorClientId: app.casdoorClientId,
      owner: app.owner,
      department: app.department,
      status: app.status,
      sort: app.sort,
      isFeatured: app.isFeatured,
      healthCheckEnabled: app.healthCheckEnabled,
      healthCheckUrl: app.healthCheckUrl,
      tagIds: app.tags.map((t) => t.id),
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    saveMutation.mutate({
      ...values,
      categoryId: values.categoryId ?? null,
    })
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
        <div>
          <Typography.Title level={3} style={{ marginBottom: 4 }}>
            应用管理
          </Typography.Title>
          <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 0 }}>
            维护门户中的应用目录、接入类型与展示信息。
          </Typography.Paragraph>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新建应用
        </Button>
      </div>

      <Input.Search
        allowClear
        placeholder="搜索应用：名称 / 编码 / 描述"
        style={{ maxWidth: 360, marginBottom: 16 }}
        onSearch={(value) => {
          setKeyword(value.trim())
          setPage(1)
        }}
      />

      <Table<Application>
        rowKey="id"
        loading={isLoading}
        dataSource={pageData?.items ?? []}
        pagination={{
          current: page,
          total: pageData?.total ?? 0,
          pageSize: 20,
          showSizeChanger: false,
          onChange: setPage,
        }}
        onChange={(_, __, sort) => {
          void sort
        }}
        columns={[
          {
            title: '应用',
            key: 'app',
            width: 260,
            render: (_, app) => (
              <Space>
                <AppIcon app={app} size={36} />
                <div>
                  <div style={{ fontWeight: 600 }}>{app.name}</div>
                  <div style={{ fontSize: 12, color: 'var(--velora-secondary)' }}>{app.code}</div>
                </div>
              </Space>
            ),
          },
          {
            title: '分类',
            dataIndex: ['category', 'name'],
            width: 120,
            render: (v) => v || '-',
          },
          {
            title: '接入类型',
            dataIndex: 'ssoType',
            width: 110,
            render: (v: Application['ssoType']) => (
              <Tag color={SSO_TYPE_COLOR[v]}>{SSO_TYPE_LABEL[v]}</Tag>
            ),
          },
          {
            title: '标签',
            dataIndex: 'tags',
            width: 160,
            render: (tags: Application['tags']) => (
              <Space size={4} wrap>
                {tags.slice(0, 2).map((t) => (
                  <Tag key={t.id} style={{ marginInlineEnd: 0 }}>{t.name}</Tag>
                ))}
                {tags.length > 2 && <Tag>+{tags.length - 2}</Tag>}
              </Space>
            ),
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 90,
            render: (v: Application['status']) => (
              <Tag color={v === 'ENABLED' ? 'success' : 'default'}>{APP_STATUS_LABEL[v]}</Tag>
            ),
          },
          {
            title: '精选',
            dataIndex: 'isFeatured',
            width: 80,
            render: (v: boolean) => (v ? <Tag color="gold">精选</Tag> : '-'),
          },
          {
            title: '排序',
            dataIndex: 'sort',
            width: 70,
          },
          {
            title: '操作',
            key: 'actions',
            width: 150,
            render: (_, app) => (
              <Space>
                <Button type="link" size="small" onClick={() => openEdit(app)}>
                  编辑
                </Button>
                <Popconfirm
                  title="删除应用"
                  description={`确定删除「${app.name}」？其策略、收藏与访问记录将一并删除。`}
                  okText="删除"
                  okButtonProps={{ danger: true }}
                  onConfirm={() => deleteMutation.mutate(app.id)}
                >
                  <Button type="link" size="small" danger>
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? `编辑应用：${editing.name}` : '新建应用'}
        open={modalOpen}
        onCancel={() => {
          setModalOpen(false)
          setEditing(null)
        }}
        onOk={() => void handleSubmit()}
        confirmLoading={saveMutation.isPending}
        width={640}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" requiredMark={false} style={{ marginTop: 16 }}>
          <Space.Compact block>
            <Form.Item label="应用编码" name="code" style={{ flex: 1 }} rules={[{ required: true, message: '请输入应用编码' }]}>
              <Input placeholder="如 devops" />
            </Form.Item>
            <Form.Item label="应用名称" name="name" style={{ flex: 1.4, marginLeft: 12 }} rules={[{ required: true, message: '请输入应用名称' }]}>
              <Input placeholder="如 DevOps 平台" />
            </Form.Item>
          </Space.Compact>

          <Form.Item label="描述" name="description">
            <Input.TextArea rows={2} placeholder="应用用途说明" />
          </Form.Item>

          <Form.Item label="关键词（搜索用，逗号分隔）" name="keywords">
            <Input placeholder="如 流水线,发布,ci" />
          </Form.Item>

          <Space.Compact block>
            <Form.Item label="图标（URL 或 Emoji）" name="icon" style={{ flex: 1 }}>
              <Input placeholder="https://.../icon.png 或 🚀" />
            </Form.Item>
            <Form.Item label="分类" name="categoryId" style={{ flex: 1, marginLeft: 12 }}>
              <Select
                allowClear
                placeholder="选择分类"
                options={(categories ?? []).map((c) => ({ value: c.id, label: c.name }))}
              />
            </Form.Item>
          </Space.Compact>

          <Space.Compact block>
            <Form.Item
              label="主页地址"
              name="homeUrl"
              style={{ flex: 1 }}
              rules={[
                { required: true, message: '请输入主页地址' },
                {
                  validator: (_, value: string) => {
                    if (value && !isSafeHttpUrl(value)) return Promise.reject(new Error('仅支持 http/https 地址'))
                    return Promise.resolve()
                  },
                },
              ]}
            >
              <Input placeholder="https://app.example.internal" />
            </Form.Item>
            <Form.Item
              label="启动地址"
              name="launchUrl"
              style={{ flex: 1, marginLeft: 12 }}
              rules={[
                {
                  validator: (_, value: string) => {
                    if (!value || isSafeHttpUrl(value)) return Promise.resolve()
                    return Promise.reject(new Error('仅支持 http/https 地址'))
                  },
                },
              ]}
            >
              <Input placeholder="留空则使用主页地址" />
            </Form.Item>
          </Space.Compact>

          <Space.Compact block>
            <Form.Item label="接入类型" name="ssoType" style={{ flex: 1 }} rules={[{ required: true }]}>
              <Select options={SSO_OPTIONS.map((v) => ({ value: v, label: SSO_TYPE_LABEL[v as Application['ssoType']] }))} />
            </Form.Item>
            <Form.Item
              noStyle
              shouldUpdate={(prev, cur) => prev.ssoType !== cur.ssoType}
            >
              {({ getFieldValue }) =>
                getFieldValue('ssoType') === 'OIDC' ? (
                  <Form.Item label="Casdoor Client ID" name="casdoorClientId" style={{ flex: 1.4, marginLeft: 12 }} rules={[{ required: true, message: 'OIDC 应用需配置 Client ID' }]}>
                    <Input placeholder="Casdoor 中该应用的 Client ID" />
                  </Form.Item>
                ) : null
              }
            </Form.Item>
          </Space.Compact>

          {watchSsoType === 'OIDC' ? (
            <Form.Item label="Casdoor 应用名（可选）" name="casdoorApplicationName">
              <Input placeholder="Casdoor 中注册的应用名称" />
            </Form.Item>
          ) : null}

          <Space.Compact block>
            <Form.Item label="负责人" name="owner" style={{ flex: 1 }}>
              <Input />
            </Form.Item>
            <Form.Item label="所属部门" name="department" style={{ flex: 1, marginLeft: 12 }}>
              <Input />
            </Form.Item>
          </Space.Compact>

          <Space.Compact block>
            <Form.Item label="标签" name="tagIds" style={{ flex: 1 }}>
              <Select
                mode="multiple"
                allowClear
                placeholder="选择标签"
                options={(tags ?? []).map((t) => ({ value: t.id, label: t.name }))}
              />
            </Form.Item>
            <Form.Item label="排序（越小越靠前）" name="sort" style={{ flex: 0.6, marginLeft: 12 }}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
          </Space.Compact>

          <Space size={24} wrap>
            <Form.Item
              label="状态"
              style={{ marginBottom: 8 }}
              shouldUpdate={(prev, cur) => prev.status !== cur.status}
            >
              {({ getFieldValue, setFieldValue }) => (
                <Switch
                  checked={getFieldValue('status') === 'ENABLED'}
                  checkedChildren="启用"
                  unCheckedChildren="停用"
                  onChange={(checked) => setFieldValue('status', checked ? 'ENABLED' : 'DISABLED')}
                />
              )}
            </Form.Item>
            <Form.Item label="精选展示" name="isFeatured" valuePropName="checked" style={{ marginBottom: 8 }}>
              <Switch />
            </Form.Item>
            <Form.Item label="健康检查" name="healthCheckEnabled" valuePropName="checked" style={{ marginBottom: 8 }}>
              <Switch />
            </Form.Item>
          </Space>
          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.healthCheckEnabled !== cur.healthCheckEnabled}
          >
            {({ getFieldValue }) =>
              getFieldValue('healthCheckEnabled') ? (
                <Form.Item label="健康检查地址" name="healthCheckUrl" rules={[{ type: 'url', message: '请输入合法 URL' }]}>
                  <Input placeholder="https://app.example.internal/healthz" />
                </Form.Item>
              ) : null
            }
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
