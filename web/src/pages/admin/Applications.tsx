import { useState } from 'react'
import { usePageTitle } from '../../hooks/usePageTitle'
import QueryErrorState from '../../components/QueryErrorState'
import AdminPageHead from '../../components/AdminPageHead'
import { isSafeHttpUrl } from '../../utils/format'
import { App as AntdApp, Alert, Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Table, Tag, Tooltip } from 'antd'
import { ApiOutlined } from '@ant-design/icons'
import { CloudSyncOutlined } from '@ant-design/icons'
import { PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  adminCreateApplication,
  adminDeleteApplication,
  adminListApplications,
  adminSyncApplications,
  adminUpdateApplication,
  createOIDCClient,
  listCategories,
  listOIDCClients,
  listTags,
  queryKeys,
  revokeOIDCClient,
  type AdminApplicationInput,
  type OIDCClient,
} from '../../api/api'
import type { Application } from '../../types'
import { APP_STATUS_LABEL, SSO_TYPE_COLOR, SSO_TYPE_LABEL } from '../../labels'
import { AppIcon } from '../../components/AppCard'

const SSO_OPTIONS = ['URL', 'OIDC', 'VELORA_OIDC', 'SAML', 'CAS', 'FORWARD_AUTH']

export default function AdminApplications() {
  usePageTitle('应用管理')

  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Application | null>(null)
  const [oidcApp, setOidcApp] = useState<Application | null>(null)
  const [form] = Form.useForm<AdminApplicationInput>()
  const watchSsoType = Form.useWatch('ssoType', form)

  const { data: categories } = useQuery({ queryKey: queryKeys.categories, queryFn: listCategories })
  const { data: tags } = useQuery({ queryKey: queryKeys.tags, queryFn: listTags })

  const { data: pageData, isLoading, isError, refetch } = useQuery({
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

  const syncMutation = useMutation({
    mutationFn: adminSyncApplications,
    onSuccess: (r) => {
      message.success(`同步完成：新增 ${r.created} 个，更新 ${r.updated} 个（共 ${r.total} 个 Casdoor 应用）`)
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '同步失败'),
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
      tagIds: (app.tags ?? []).map((t) => t.id),
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

  const openOIDCClients = (app: Application) => setOidcApp(app)

  return (
    <div>
      <AdminPageHead
        title="应用管理"
        extra={
          <>
            <Tooltip title="将 Casdoor 中已接入统一登录的应用同步到门户（含图标与名称）">
              <Button icon={<CloudSyncOutlined />} loading={syncMutation.isPending} onClick={() => syncMutation.mutate()}>
                从 Casdoor 同步
              </Button>
            </Tooltip>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建应用
            </Button>
          </>
        }
      />

      <Input.Search
        allowClear
        placeholder="搜索名称 / 编码 / 描述"
        style={{ maxWidth: 360, marginBottom: 16 }}
        onSearch={(value) => {
          setKeyword(value.trim())
          setPage(1)
        }}
      />

      {isError ? (
        <QueryErrorState refetch={refetch} />
      ) : (
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
              <span className={`velora-status-pill ${v === 'ENABLED' ? 'is-enabled' : 'is-disabled'}`}>
                {APP_STATUS_LABEL[v]}
              </span>
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
            width: 210,
            render: (_, app) => (
              <Space>
                {app.ssoType === 'VELORA_OIDC' && (
                  <Button type="link" size="small" icon={<ApiOutlined />} onClick={() => openOIDCClients(app)}>
                    OIDC 客户端
                  </Button>
                )}
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
      )}

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
        <Form form={form} layout="vertical" requiredMark={false} style={{ marginTop: 4 }}>
          <div className="velora-form-grid">
            <Form.Item label="应用编码" name="code" rules={[{ required: true, message: '请输入应用编码' }]}>
              <Input placeholder="如 devops" />
            </Form.Item>
            <Form.Item label="应用名称" name="name" rules={[{ required: true, message: '请输入应用名称' }]}>
              <Input placeholder="如 DevOps 平台" />
            </Form.Item>
          </div>

          <Form.Item label="描述" name="description">
            <Input.TextArea rows={2} placeholder="应用用途说明" />
          </Form.Item>

          <Form.Item label="关键词" name="keywords">
            <Input placeholder="搜索用，多个用逗号分隔" />
          </Form.Item>

          <div className="velora-form-grid">
            <Form.Item label="图标" name="icon">
              <Input placeholder="图片 URL 或 Emoji" />
            </Form.Item>
            <Form.Item label="分类" name="categoryId">
              <Select
                allowClear
                placeholder="选择分类"
                options={(categories ?? []).map((c) => ({ value: c.id, label: c.name }))}
              />
            </Form.Item>
          </div>

          <div className="velora-form-grid">
            <Form.Item
              label="主页地址"
              name="homeUrl"
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
          </div>

          <div className="velora-form-grid">
            <Form.Item label="接入类型" name="ssoType" rules={[{ required: true }]}>
              <Select options={SSO_OPTIONS.map((v) => ({ value: v, label: SSO_TYPE_LABEL[v as Application['ssoType']] }))} />
            </Form.Item>
            <Form.Item
              noStyle
              shouldUpdate={(prev, cur) => prev.ssoType !== cur.ssoType}
            >
              {({ getFieldValue }) =>
                getFieldValue('ssoType') === 'OIDC' ? (
                  <Form.Item label="Casdoor Client ID" name="casdoorClientId" rules={[{ required: true, message: 'OIDC 应用需配置 Client ID' }]}>
                    <Input placeholder="Casdoor 中该应用的 Client ID" />
                  </Form.Item>
                ) : null
              }
            </Form.Item>
            {watchSsoType === 'OIDC' ? (
              <Form.Item label="Casdoor 应用名" name="casdoorApplicationName">
                <Input placeholder="可选" />
              </Form.Item>
            ) : null}
          </div>

          <div className="velora-form-grid">
            <Form.Item label="负责人" name="owner">
              <Input />
            </Form.Item>
            <Form.Item label="所属部门" name="department">
              <Input />
            </Form.Item>
          </div>

          <div className="velora-form-grid">
            <Form.Item label="标签" name="tagIds">
              <Select
                mode="multiple"
                allowClear
                placeholder="选择标签"
                options={(tags ?? []).map((t) => ({ value: t.id, label: t.name }))}
              />
            </Form.Item>
            <Form.Item label="排序" name="sort">
              <InputNumber min={0} placeholder="越小越靠前" style={{ width: '100%' }} />
            </Form.Item>
          </div>

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

      {/* OIDC 客户端管理（Phase B6）：仅 VELORA_OIDC 接入的应用 */}
      <OIDCClientModal app={oidcApp} onClose={() => setOidcApp(null)} />
    </div>
  )
}

/**
 * OIDC 客户端管理弹窗：查看 / 新建（返回一次 secret）/ 吊销客户端。
 * 展示对接参数（authorize/token/userinfo/jwks 端点）便于第三方接入。
 */
function OIDCClientModal({ app, onClose }: { app: Application | null; onClose: () => void }) {
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [newSecret, setNewSecret] = useState<{ clientId: string; secret: string } | null>(null)
  const [redirectInput, setRedirectInput] = useState('')

  const { data: clients, isLoading } = useQuery({
    queryKey: ['admin', 'oidc-clients', app?.id],
    queryFn: () => listOIDCClients(app!.id),
    enabled: !!app,
  })

  const createMutation = useMutation({
    mutationFn: (redirectUris: string[]) => createOIDCClient(app!.id, redirectUris),
    onSuccess: (r) => {
      message.success('客户端已创建（密钥仅显示一次，请立即保存）')
      setNewSecret({ clientId: r.client.clientId, secret: r.clientSecret })
      setRedirectInput('')
      void queryClient.invalidateQueries({ queryKey: ['admin', 'oidc-clients', app?.id] })
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '创建失败'),
  })

  const revokeMutation = useMutation({
    mutationFn: (clientId: string) => revokeOIDCClient(clientId),
    onSuccess: () => {
      message.success('客户端已吊销')
      void queryClient.invalidateQueries({ queryKey: ['admin', 'oidc-clients', app?.id] })
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '吊销失败'),
  })

  const endpointBase = `${window.location.origin}/oidc`

  return (
    <Modal
      title={`OIDC 客户端：${app?.name ?? ''}`}
      open={!!app}
      onCancel={onClose}
      footer={null}
      width={640}
      destroyOnHidden
    >
      {app && (
        <>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="第三方应用通过以下 Velora SSO 端点对接（PKCE S256 强制）"
            description={
              <div style={{ fontSize: 12 }}>
                authorize：<code>{endpointBase}/authorize</code>
                <br />
                token：<code>{endpointBase}/token</code> · userinfo：
                <code>{endpointBase}/userinfo</code> · jwks：<code>{endpointBase}/jwks</code>
                <br />
                发现文档：<code>{endpointBase}/.well-known/openid-configuration</code>
              </div>
            }
          />

          {newSecret && (
            <Alert
              type="success"
              showIcon
              style={{ marginBottom: 12 }}
              message={`客户端 ${newSecret.clientId} 创建成功`}
              description={
                <div>
                  <div>
                    Client ID：<code>{newSecret.clientId}</code>
                  </div>
                  <div>
                    Client Secret：<code>{newSecret.secret}</code>
                  </div>
                  <div style={{ marginTop: 4, color: '#fa8c16' }}>⚠ 密钥仅本次显示，请立即复制保存；遗失需吊销后重建。</div>
                </div>
              }
            />
          )}

          {/* 新建客户端 */}
          <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
            <Input
              placeholder="回调地址 redirect_uri（多个用逗号分隔）"
              value={redirectInput}
              onChange={(e) => setRedirectInput(e.target.value)}
              onPressEnter={() => {
                const uris = redirectInput.split(',').map((s) => s.trim()).filter(Boolean)
                if (uris.length) createMutation.mutate(uris)
              }}
            />
            <Button
              type="primary"
              loading={createMutation.isPending}
              disabled={!redirectInput.trim()}
              onClick={() => {
                const uris = redirectInput.split(',').map((s) => s.trim()).filter(Boolean)
                createMutation.mutate(uris)
              }}
            >
              新建客户端
            </Button>
          </Space.Compact>

          <Table<OIDCClient>
            rowKey="clientId"
            size="small"
            loading={isLoading}
            dataSource={clients ?? []}
            pagination={false}
            locale={{ emptyText: '暂无客户端，请先创建' }}
            columns={[
              {
                title: 'Client ID',
                dataIndex: 'clientId',
                ellipsis: true,
                render: (v: string) => <code style={{ fontSize: 12 }}>{v}</code>,
              },
              {
                title: '回调地址',
                dataIndex: 'redirectUris',
                render: (uris: string[]) => (
                  <Space size={4} wrap>
                    {uris.map((u) => (
                      <Tag key={u} style={{ fontSize: 11, maxWidth: 220 }}>
                        {u}
                      </Tag>
                    ))}
                  </Space>
                ),
              },
              {
                title: '创建时间',
                dataIndex: 'createdAt',
                width: 100,
                render: (v: string) => (v ? new Date(v).toLocaleDateString() : '-'),
              },
              {
                title: '操作',
                key: 'op',
                width: 70,
                render: (_, cl) => (
                  <Popconfirm
                    title="吊销客户端？"
                    description="吊销后该客户端所有 code/token 立即失效，第三方应用将无法登录。"
                    okText="吊销"
                    okButtonProps={{ danger: true }}
                    onConfirm={() => revokeMutation.mutate(cl.clientId)}
                  >
                    <Button type="link" size="small" danger loading={revokeMutation.isPending}>
                      吊销
                    </Button>
                  </Popconfirm>
                ),
              },
            ]}
          />
        </>
      )}
    </Modal>
  )
}
