import { useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { App as AntdApp, DatePicker, Form, Input, Modal, Select } from 'antd'
import dayjs from 'dayjs'
import { convertMailToTodo, queryKeys } from '../../api/api'
import type { MailMessage, TodoKind, TodoPriority } from '../../types'

interface Props {
  mail: MailMessage | null
  onClose: () => void
}

interface FormValues {
  title: string
  priority: TodoPriority
  kind: TodoKind
  dueAt?: dayjs.Dayjs
}

const PRIORITY_OPTIONS: { value: TodoPriority; label: string }[] = [
  { value: 'urgent', label: '紧急' },
  { value: 'high', label: '高' },
  { value: 'mid', label: '中' },
  { value: 'low', label: '低' },
]

// 待办类型（不含 mail——邮件本身已在邮件 Tab，转出的待办归属于业务类型）
const KIND_OPTIONS: { value: TodoKind; label: string }[] = [
  { value: 'approval', label: '审批' },
  { value: 'devops', label: '研发' },
  { value: 'ops', label: '运维' },
  { value: 'project', label: '项目' },
  { value: 'hr', label: '人事' },
  { value: 'other', label: '其他' },
]

/**
 * 邮件 → 待办转换。幂等：同一邮件重复转换只更新原待办（服务端按
 * source_system='mail' + source_id=邮件ID 去重），不会刷出重复卡片。
 */
export default function ConvertTodoModal({ mail, onClose }: Props) {
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [form] = Form.useForm<FormValues>()

  useEffect(() => {
    if (mail) {
      form.setFieldsValue({
        title: mail.subject || '（无主题邮件）',
        priority: 'mid',
        kind: 'approval',
        dueAt: undefined,
      })
    }
  }, [mail, form])

  const convertMutation = useMutation({
    mutationFn: (values: FormValues) =>
      convertMailToTodo(mail?.id ?? 0, {
        title: values.title.trim(),
        priority: values.priority,
        kind: values.kind,
        dueAt: values.dueAt ? values.dueAt.endOf('day').toISOString() : null,
      }),
    onSuccess: () => {
      message.success('已转为待办')
      queryClient.invalidateQueries({ queryKey: queryKeys.todos })
      form.resetFields()
      onClose()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '转换失败'),
  })

  const submit = async () => {
    const values = await form.validateFields()
    convertMutation.mutate(values)
  }

  return (
    <Modal
      title="转为待办"
      open={mail != null}
      onCancel={() => {
        form.resetFields()
        onClose()
      }}
      onOk={submit}
      okText="创建待办"
      cancelText="取消"
      confirmLoading={convertMutation.isPending}
      width={440}
      destroyOnHidden
    >
      {mail && (
        <div className="velora-convert-source">
          来源：企业邮箱 · {mail.fromName || mail.fromAddress}
        </div>
      )}
      <Form form={form} layout="vertical" requiredMark={false} style={{ marginTop: 12 }}>
        <Form.Item
          name="title"
          label="待办标题"
          rules={[
            { required: true, message: '请输入待办标题' },
            { max: 256, message: '标题过长' },
          ]}
        >
          <Input placeholder="待办标题" maxLength={256} />
        </Form.Item>
        <Form.Item name="priority" label="优先级" rules={[{ required: true }]}>
          <Select options={PRIORITY_OPTIONS} />
        </Form.Item>
        <Form.Item name="kind" label="待办类型" rules={[{ required: true }]}>
          <Select options={KIND_OPTIONS} />
        </Form.Item>
        <Form.Item name="dueAt" label="截止日期（可选）">
          <DatePicker style={{ width: '100%' }} placeholder="选择日期" />
        </Form.Item>
      </Form>
    </Modal>
  )
}
