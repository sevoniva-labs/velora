import { App as AntdApp, Button, Card, Form, Input, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getPortalSettings, queryKeys, updatePortalSetting } from '../../api/api'

const SETTING_FIELDS = [
  { key: 'portal_name', label: '门户名称', placeholder: '如 Velora' },
  { key: 'portal_welcome', label: '欢迎语', placeholder: '如 企业应用门户' },
  { key: 'portal_footer', label: '页脚文案', placeholder: '如 © Velora · 企业应用门户' },
]

export default function AdminSettings() {
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()

  const { data: settings, isLoading } = useQuery({
    queryKey: queryKeys.portalSettings,
    queryFn: getPortalSettings,
  })

  const saveMutation = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) => updatePortalSetting(key, value),
    onSuccess: () => {
      message.success('设置已保存')
      void queryClient.invalidateQueries({ queryKey: queryKeys.portalSettings })
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '保存失败'),
  })

  const valueOf = (key: string) => settings?.find((s) => s.key === key)?.value ?? ''

  return (
    <div style={{ maxWidth: 640 }}>
      <div style={{ marginBottom: 16 }}>
        <Typography.Title level={3} style={{ marginBottom: 4 }}>
          门户设置
        </Typography.Title>
        <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 0 }}>
          门户的基础展示信息（第一阶段的轻量配置入口）。
        </Typography.Paragraph>
      </div>

      <Card loading={isLoading}>
        {SETTING_FIELDS.map((field) => (
          <SettingRow
            key={field.key}
            label={field.label}
            placeholder={field.placeholder}
            defaultValue={valueOf(field.key)}
            saving={saveMutation.isPending}
            onSave={(value) => saveMutation.mutate({ key: field.key, value })}
          />
        ))}
      </Card>
    </div>
  )
}

function SettingRow({
  label,
  placeholder,
  defaultValue,
  saving,
  onSave,
}: {
  label: string
  placeholder: string
  defaultValue: string
  saving: boolean
  onSave: (value: string) => void
}) {
  const [form] = Form.useForm<{ value: string }>()
  return (
    <Form
      form={form}
      layout="vertical"
      style={{ marginBottom: 20 }}
      initialValues={{ value: defaultValue }}
      onFinish={(v) => onSave(v.value)}
    >
      <Form.Item label={label} name="value" style={{ marginBottom: 8 }}>
        <Input placeholder={placeholder} maxLength={120} />
      </Form.Item>
      <Button type="primary" htmlType="submit" size="small" loading={saving}>
        保存
      </Button>
    </Form>
  )
}
