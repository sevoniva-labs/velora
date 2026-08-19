import { App as AntdApp, Button, Form, Input, InputNumber, Spin } from 'antd'
import { usePageTitle } from '../../hooks/usePageTitle'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AdminPageHead from '../../components/AdminPageHead'
import { getPortalSettings, queryKeys, updatePortalSetting } from '../../api/api'

interface SettingField {
  key: string
  label: string
  help: string
  placeholder: string
  textarea?: boolean
}

const SETTING_SECTIONS: { key: string; title: string; fields: SettingField[] }[] = [
  {
    key: 'base',
    title: '基础信息',
    fields: [
      { key: 'portal_name', label: '门户名称', help: '顶栏、登录页与浏览器标签页标题。', placeholder: '如 Velora' },
      { key: 'portal_welcome', label: '欢迎语', help: '顶栏品牌名下方，以及登录页品牌区。', placeholder: '如 企业应用门户' },
      { key: 'portal_footer', label: '页脚文案', help: '登录页底部；留空展示默认版权信息。', placeholder: '如 © Velora · 企业应用门户' },
    ],
  },
  {
    key: 'notice',
    title: '公告',
    fields: [
      {
        key: 'announcement',
        label: '工作台公告',
        help: '首页顶部跑马灯滚动展示，多条公告用 | 分隔。',
        placeholder: '如 系统将于本周六 22:00-24:00 升级维护，请提前保存工作',
        textarea: true,
      },
    ],
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
      void queryClient.invalidateQueries({ queryKey: ['applications'] })
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '保存失败'),
  })

  const valueOf = (key: string) => settings?.find((s) => s.key === key)?.value ?? ''
  const uiScalePercent = valueOf('ui_scale') ? Math.round(Number.parseFloat(valueOf('ui_scale')) * 100) : 100
  const parsedNewDays = Number.parseInt(valueOf('new_badge_days'), 10)
  const newBadgeDays = valueOf('new_badge_days') === '' || Number.isNaN(parsedNewDays) ? 7 : parsedNewDays
  const saving = saveMutation.isPending

  return (
    <div className="velora-admin-settings">
      <AdminPageHead title="门户设置" />

      {isLoading ? (
        <div className="velora-admin-card velora-settings-loading">
          <Spin />
        </div>
      ) : (
        <>
          {SETTING_SECTIONS.map((section) => (
            <section key={section.key} className="velora-admin-card velora-settings-section">
              <header className="velora-settings-section-head">
                <h3 className="velora-settings-section-title">{section.title}</h3>
              </header>
              {section.fields.map((field) => (
                <SettingRow
                  key={`${field.key}:${valueOf(field.key)}`}
                  label={field.label}
                  help={field.help}
                  placeholder={field.placeholder}
                  textarea={field.textarea}
                  defaultValue={valueOf(field.key)}
                  saving={saving}
                  onSave={(value) => saveMutation.mutate({ key: field.key, value })}
                />
              ))}
            </section>
          ))}

          <section className="velora-admin-card velora-settings-section">
            <header className="velora-settings-section-head">
              <h3 className="velora-settings-section-title">界面显示</h3>
            </header>
            <Form
              key={`ui_scale:${uiScalePercent}`}
              initialValues={{ scale: uiScalePercent }}
              onFinish={(v) => saveMutation.mutate({ key: 'ui_scale', value: String(v.scale / 100) })}
            >
              <div className="velora-settings-row">
                <div className="velora-settings-row-text">
                  <div className="velora-settings-row-label">界面缩放（%）</div>
                  <div className="velora-settings-row-help">
                    访问端显示大小与预期不一致时整体调整，效果等同浏览器 Ctrl + / -。默认 100%。
                  </div>
                </div>
                <div className="velora-settings-row-control velora-settings-row-control--fixed">
                  <Form.Item name="scale" noStyle>
                    <InputNumber min={80} max={140} step={5} addonAfter="%" style={{ width: 150 }} />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" loading={saving}>
                    保存
                  </Button>
                </div>
              </div>
            </Form>
            <Form
              key={`new_badge_days:${newBadgeDays}`}
              initialValues={{ days: newBadgeDays }}
              onFinish={(v) => saveMutation.mutate({ key: 'new_badge_days', value: String(v.days) })}
            >
              <div className="velora-settings-row">
                <div className="velora-settings-row-text">
                  <div className="velora-settings-row-label">新应用标识（天）</div>
                  <div className="velora-settings-row-help">上架 N 天内的应用显示「新」角标，填 0 关闭。默认 7 天。</div>
                </div>
                <div className="velora-settings-row-control velora-settings-row-control--fixed">
                  <Form.Item name="days" noStyle>
                    <InputNumber min={0} max={90} addonAfter="天" style={{ width: 150 }} />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" loading={saving}>
                    保存
                  </Button>
                </div>
              </div>
            </Form>
          </section>
        </>
      )}
    </div>
  )
}

function SettingRow({
  label,
  help,
  placeholder,
  textarea,
  defaultValue,
  saving,
  onSave,
}: {
  label: string
  help: string
  placeholder: string
  textarea?: boolean
  defaultValue: string
  saving: boolean
  onSave: (value: string) => void
}) {
  const [form] = Form.useForm<{ value: string }>()
  return (
    <Form form={form} initialValues={{ value: defaultValue }} onFinish={(v) => onSave(v.value)}>
      <div className="velora-settings-row">
        <div className="velora-settings-row-text">
          <div className="velora-settings-row-label">{label}</div>
          <div className="velora-settings-row-help">{help}</div>
        </div>
        <div className="velora-settings-row-control">
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
      </div>
    </Form>
  )
}
