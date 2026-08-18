import { App as AntdApp, Button, Card, Form, Input, InputNumber } from 'antd'
import { usePageTitle } from '../../hooks/usePageTitle'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AdminPageHead from '../../components/AdminPageHead'
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
    <div className="velora-admin-settings">
      <AdminPageHead title="门户设置" desc="门户名称、欢迎语、公告等基础展示配置。" />

      <Card loading={isLoading}>
        {SETTING_FIELDS.map((field) => (
          <SettingRow
            key={`${field.key}:${valueOf(field.key)}`}
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
          className="velora-admin-setting-row"
          key={`ui_scale:${uiScalePercent}`}
          initialValues={{ scale: uiScalePercent }}
          onFinish={(v) => saveMutation.mutate({ key: 'ui_scale', value: String(v.scale / 100) })}
        >
          <div className="velora-admin-setting-label">界面缩放（%）</div>
          <div className="velora-admin-setting-control velora-admin-setting-control--fixed">
            <Form.Item name="scale" noStyle>
              <InputNumber min={80} max={140} step={5} addonAfter="%" style={{ width: 160 }} />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={saveMutation.isPending}>
              保存
            </Button>
          </div>
          <p className="velora-admin-setting-help">
            部署到服务器后，若访问端的显示大小与本地不一致（不同分辨率 / 浏览器缩放），可整体调整界面大小，
            效果等同浏览器 Ctrl + / -。默认 100%。
          </p>
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
      className="velora-admin-setting-row"
      initialValues={{ value: defaultValue }}
      onFinish={(v) => onSave(v.value)}
    >
      <div className="velora-admin-setting-label">{label}</div>
      <div className="velora-admin-setting-control">
        <Form.Item name="value" noStyle>
          {textarea ? (
            <Input.TextArea placeholder={placeholder} maxLength={500} autoSize={{ minRows: 2, maxRows: 4 }} />
          ) : (
            <Input placeholder={placeholder} maxLength={120} />
          )}
        </Form.Item>
        <Button type="primary" htmlType="submit" loading={saving}>
          保存
        </Button>
      </div>
    </Form>
  )
}
