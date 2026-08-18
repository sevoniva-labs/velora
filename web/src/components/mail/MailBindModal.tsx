import { useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntdApp, Form, Input, Modal, Select } from 'antd'
import { bindMailAccount, listMailProviders, queryKeys } from '../../api/api'
import type { MailAccount } from '../../types'

interface Props {
  open: boolean
  onClose: () => void
  onBound: (acc: MailAccount) => void
}

interface FormValues {
  provider: string
  email: string
  password: string
  imapHost?: string
  imapPort?: number
}

/**
 * 绑定企业邮箱。服务端在落库前会实测 IMAP 连接（最长约 10s），
 * 因此提交期间 confirmLoading + 不重复点击。凭证仅加密后落库。
 */
export default function MailBindModal({ open, onClose, onBound }: Props) {
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [form] = Form.useForm<FormValues>()
  const provider = Form.useWatch('provider', form)

  const providersQuery = useQuery({
    queryKey: queryKeys.mailProviders,
    queryFn: listMailProviders,
    enabled: open,
    staleTime: 5 * 60 * 1000,
  })
  const profiles = providersQuery.data?.profiles ?? []

  useEffect(() => {
    if (open) {
      form.setFieldsValue({ provider: form.getFieldValue('provider') || 'aliyun' })
    }
  }, [open, form])

  const bindMutation = useMutation({
    mutationFn: bindMailAccount,
    onSuccess: (acc) => {
      message.success('绑定成功，正在拉取邮件…')
      queryClient.invalidateQueries({ queryKey: queryKeys.mailAccounts })
      form.resetFields()
      onBound(acc)
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '绑定失败'),
  })

  const submit = async () => {
    const values = await form.validateFields()
    bindMutation.mutate({
      provider: values.provider,
      email: values.email.trim(),
      password: values.password.trim(),
      imapHost: values.imapHost?.trim(),
      imapPort: values.imapPort ? Number(values.imapPort) : undefined,
    })
  }

  return (
    <Modal
      title="绑定企业邮箱"
      open={open}
      onCancel={() => {
        form.resetFields()
        onClose()
      }}
      onOk={submit}
      okText="验证并绑定"
      cancelText="取消"
      confirmLoading={bindMutation.isPending}
      width={440}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" requiredMark={false} style={{ marginTop: 12 }}>
        <Form.Item name="provider" label="邮箱服务商" rules={[{ required: true, message: '请选择服务商' }]}>
          <Select
            loading={providersQuery.isLoading}
            options={profiles.map((p) => ({ value: p.provider, label: p.label }))}
            placeholder="选择服务商"
          />
        </Form.Item>
        <Form.Item
          name="email"
          label="邮箱地址"
          rules={[
            { required: true, message: '请输入邮箱地址' },
            { type: 'email', message: '邮箱格式不正确' },
          ]}
        >
          <Input placeholder="zhangsan@company.com" autoComplete="off" />
        </Form.Item>
        <Form.Item
          name="password"
          label="客户端授权码"
          tooltip="非登录密码。在邮箱「设置 → 账户 → IMAP/SMTP」中开启服务并生成授权码。"
          rules={[{ required: true, message: '请输入客户端授权码' }]}
        >
          <Input.Password placeholder="非登录密码，在邮箱设置中获取" autoComplete="new-password" />
        </Form.Item>
        {provider === 'custom' && (
          <>
            <Form.Item
              name="imapHost"
              label="IMAP 服务器"
              rules={[{ required: true, message: '请输入 IMAP 服务器地址' }]}
            >
              <Input placeholder="imap.example.com" />
            </Form.Item>
            <Form.Item name="imapPort" label="IMAP 端口（SSL）">
              <Input type="number" placeholder="993" />
            </Form.Item>
          </>
        )}
      </Form>
    </Modal>
  )
}
