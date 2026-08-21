import { useState } from 'react'
import { usePageTitle } from '../../hooks/usePageTitle'
import QueryErrorState from '../../components/QueryErrorState'
import AdminPageHead from '../../components/AdminPageHead'
import { App as AntdApp, Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Table } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  adminCreateCategory,
  adminDeleteCategory,
  adminUpdateCategory,
  listCategories,
  queryKeys,
} from '../../api/api'
import type { Category } from '../../types'

export default function AdminCategories() {
  usePageTitle('分类管理')

  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Category | null>(null)
  const [form] = Form.useForm<Partial<Category>>()

  const { data: categories, isLoading, isError, refetch } = useQuery({ queryKey: queryKeys.categories, queryFn: listCategories })

  const invalidate = () => void queryClient.invalidateQueries({ queryKey: queryKeys.categories })

  const saveMutation = useMutation({
    mutationFn: (input: Partial<Category>) =>
      editing ? adminUpdateCategory(editing.id, input) : adminCreateCategory(input),
    onSuccess: () => {
      message.success(editing ? '分类已更新' : '分类已创建')
      setModalOpen(false)
      setEditing(null)
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '保存失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string | number) => adminDeleteCategory(id),
    onSuccess: () => {
      message.success('分类已删除')
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '删除失败'),
  })

  const openModal = (cat?: Category) => {
    setEditing(cat ?? null)
    form.resetFields()
    if (cat) form.setFieldsValue(cat)
    else form.setFieldsValue({ sort: 0 })
    setModalOpen(true)
  }

  return (
    <div>
      <AdminPageHead
        title="分类管理"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>
            新建分类
          </Button>
        }
      />

      {isError ? (
        <QueryErrorState refetch={refetch} />
      ) : (
        <Table<Category>
        rowKey="id"
        loading={isLoading}
        dataSource={categories ?? []}
        pagination={false}
        columns={[
          { title: '编码', dataIndex: 'code', width: 160 },
          { title: '名称', dataIndex: 'name', width: 180 },
          { title: '描述', dataIndex: 'description', render: (v: string) => v || '-' },
          { title: '排序', dataIndex: 'sort', width: 80 },
          {
            title: '操作',
            key: 'actions',
            width: 140,
            render: (_, cat) => (
              <Space>
                <Button type="link" size="small" onClick={() => openModal(cat)}>
                  编辑
                </Button>
                <Popconfirm
                  title="删除分类"
                  description="删除后该分类下的应用将变为未分类。"
                  okText="删除"
                  okButtonProps={{ danger: true }}
                  onConfirm={() => deleteMutation.mutate(cat.id)}
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
        title={editing ? '编辑分类' : '新建分类'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
      >
        <Form
          form={form}
          layout="vertical"
          requiredMark={false}
          style={{ marginTop: 16 }}
          onFinish={(values) => saveMutation.mutate(values)}
        >
          <Form.Item label="分类编码" name="code" rules={[{ required: true, message: '请输入编码' }]}>
            <Input placeholder="如 rd / ops / ai" />
          </Form.Item>
          <Form.Item label="分类名称" name="name" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如 研发工具" />
          </Form.Item>
          <Form.Item label="描述" name="description">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item label="排序" name="sort">
            <InputNumber min={0} placeholder="越小越靠前" style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
