import { App as AntdApp, Button, Card, Form, Input, InputNumber, Typography } from 'antd'
import { usePageTitle } from '../../hooks/usePageTitle'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getPortalSettings, queryKeys, updatePortalSetting } from '../../api/api'

const SETTING_FIELDS = [
  { key: 'portal_name', label: '门户名称', placeholder: '如 Velora' },
  { key: 'portal_welcome', label: '欢迎语', placeholder: '如 企业应用门户' },
  { key: 'portal_footer', label: '页脚文案', placeholder: '如 © Velora · 企业应用门户' },
  {
    key: 'announcement',
    label: '工作台公告（跑马灯）',
    placeholder: '如 系统将于本周六 22:00-24:00 升级维护，请提前保存工作。多条公告用 | 分隔',
    textarea: true,
  },
]

export default function AdminSettings() {
  usePageTitle('门户设置')

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
  const uiScalePercent = valueOf('ui_scale') ? Math.round(Number.parseFloat(valueOf('ui_scale')) * 100) : 100

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
            textarea={field.textarea}
            defaultValue={valueOf(field.key)}
            saving={saveMutation.isPending}
            onSave={(value) => saveMutation.mutate({ key: field.key, value })}
          />
        ))}

        <Form
          layout="vertical"
          style={{ marginTop: 8, paddingTop: 16, borderTop: '1px solid var(--velora-border)' }}
          initialValues={{ scale: uiScalePercent }}
          onFinish={(v) => saveMutation.mutate({ key: 'ui_scale', value: String(v.scale / 100) })}
        >
          <Form.Item label="界面缩放（%）" name="scale" style={{ marginBottom: 8 }}>
            <InputNumber min={80} max={140} step={5} addonAfter="%" style={{ width: 160 }} />
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 10 }}>
            部署到服务器后，若访问端的显示大小与本地不一致（不同分辨率 / 浏览器缩放），可在此整体调整界面大小，
            效果等同浏览器 Ctrl + / -，对布局、文字、组件统一生效。默认 100%。
          </Typography.Paragraph>
          <Button type="primary" htmlType="submit" size="small" loading={saveMutation.isPending}>
            保存
          </Button>
        </Form>
      </Card>
    </div>
  )
}

function SettingRow({
  label,
  placeholder,
  textarea,
  defaultValue,
  saving,
  onSave,
}: {
  label: string
  placeholder: string
  textarea?: boolean
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
        {textarea ? (
          <Input.TextArea placeholder={placeholder} maxLength={500} autoSize={{ minRows: 2, maxRows: 4 }} />
        ) : (
          <Input placeholder={placeholder} maxLength={120} />
        )}
      </Form.Item>
      <Button type="primary" htmlType="submit" size="small" loading={saving}>
        保存
      </Button>
    </Form>
  )
}
