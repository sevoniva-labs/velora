import { useState } from 'react'
import { App, Button, Popconfirm, Space } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormText,
  ProFormTextArea,
  ProTable,
  type ProColumns,
} from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  adminCreateCategory,
  adminCreateTag,
  adminDeleteCategory,
  adminDeleteTag,
  adminUpdateCategory,
  adminUpdateTag,
  listCategories,
  listTags,
  queryKeys,
} from '../../api/api'
import QueryErrorState from '../../components/QueryErrorState'
import { usePageTitle } from '../../hooks/usePageTitle'
import type { Category, Tag } from '../../types'

type TaxonomyTab = 'categories' | 'tags'
type TaxonomyRecord = Category | Tag

interface TaxonomyForm {
  code: string
  name: string
  description?: string
  sort: number
}

export default function Taxonomy() {
  usePageTitle('分类与标签')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<TaxonomyTab>('categories')
  const [editing, setEditing] = useState<TaxonomyRecord>()
  const [formOpen, setFormOpen] = useState(false)
  const categories = useQuery({ queryKey: queryKeys.categories, queryFn: listCategories })
  const tags = useQuery({ queryKey: queryKeys.tags, queryFn: listTags })
  const activeQuery = tab === 'categories' ? categories : tags

  const refresh = async (target: TaxonomyTab) => {
    await queryClient.invalidateQueries({ queryKey: target === 'categories' ? queryKeys.categories : queryKeys.tags })
  }

  const save = useMutation({
    mutationFn: async (values: TaxonomyForm) => {
      if (tab === 'categories') {
        return editing
          ? adminUpdateCategory(editing.id, values)
          : adminCreateCategory(values)
      }
      return editing
        ? adminUpdateTag(editing.id, values)
        : adminCreateTag(values)
    },
    onSuccess: async () => {
      message.success(editing ? '已保存' : tab === 'categories' ? '分类已创建' : '标签已创建')
      setFormOpen(false)
      setEditing(undefined)
      await refresh(tab)
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  const remove = useMutation({
    mutationFn: async (record: TaxonomyRecord) => {
      if (tab === 'categories') await adminDeleteCategory(record.id)
      else await adminDeleteTag(record.id)
    },
    onSuccess: async () => {
      message.success(tab === 'categories' ? '分类已删除' : '标签已删除')
      await refresh(tab)
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const openForm = (record?: TaxonomyRecord) => {
    setEditing(record)
    setFormOpen(true)
  }

  const commonColumns: ProColumns<TaxonomyRecord>[] = [
    { title: '编码', dataIndex: 'code', width: 180, copyable: true },
    { title: '名称', dataIndex: 'name', width: 220 },
    { title: '排序', dataIndex: 'sort', width: 100, search: false },
    {
      title: '操作',
      valueType: 'option',
      width: 140,
      render: (_, record) => (
        <Space size={4}>
          <Button type="link" onClick={() => openForm(record)}>编辑</Button>
          <Popconfirm
            title={tab === 'categories' ? '删除分类？' : '删除标签？'}
            description={tab === 'categories' ? '相关应用将变为未分类。' : '相关应用将移除该标签。'}
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
            onConfirm={() => remove.mutate(record)}
          >
            <Button type="link" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]
  const columns: ProColumns<TaxonomyRecord>[] = tab === 'categories'
    ? [commonColumns[0], commonColumns[1], { title: '说明', dataIndex: 'description', search: false, ellipsis: true, renderText: (value) => value || '—' }, commonColumns[2], commonColumns[3]]
    : commonColumns

  return (
    <PageContainer
      title="分类与标签"
      tabList={[{ key: 'categories', tab: '分类' }, { key: 'tags', tab: '标签' }]}
      tabActiveKey={tab}
      onTabChange={(key) => setTab(key as TaxonomyTab)}
    >
      {activeQuery.isError ? (
        <QueryErrorState refetch={activeQuery.refetch} />
      ) : (
        <ProTable<TaxonomyRecord>
          key={tab}
          rowKey="id"
          columns={columns}
          dataSource={(activeQuery.data ?? []) as TaxonomyRecord[]}
          loading={activeQuery.isLoading}
          search={{ labelWidth: 'auto' }}
          pagination={false}
          toolBarRender={() => [
            <Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => openForm()}>
              {tab === 'categories' ? '新建分类' : '新建标签'}
            </Button>,
          ]}
        />
      )}

      <ModalForm<TaxonomyForm>
        key={`${tab}-${editing?.id ?? 'new'}`}
        title={editing ? (tab === 'categories' ? '编辑分类' : '编辑标签') : (tab === 'categories' ? '新建分类' : '新建标签')}
        open={formOpen}
        onOpenChange={(open) => { setFormOpen(open); if (!open) setEditing(undefined) }}
        initialValues={editing ? { ...editing } : { sort: 0 }}
        submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }}
        onFinish={async (values) => { await save.mutateAsync(values); return true }}
      >
        <ProFormText
          name="code"
          label="编码"
          disabled={Boolean(editing)}
          rules={[{ required: true, message: '请输入编码' }, { pattern: /^[a-z][a-z0-9_-]{1,63}$/, message: '使用小写字母、数字、短横线或下划线' }]}
        />
        <ProFormText name="name" label="名称" fieldProps={{ maxLength: 64 }} rules={[{ required: true, message: '请输入名称' }]} />
        {tab === 'categories' && <ProFormTextArea name="description" label="说明" fieldProps={{ maxLength: 200, showCount: true }} />}
        <ProFormDigit name="sort" label="排序" min={0} max={9999} rules={[{ required: true }]} />
      </ModalForm>
    </PageContainer>
  )
}
