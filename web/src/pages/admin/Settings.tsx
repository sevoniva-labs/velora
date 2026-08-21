import { Alert, Spin, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { usePageTitle } from '../../hooks/usePageTitle'
import AdminPageHead from '../../components/AdminPageHead'
import { getPortalSettings, queryKeys } from '../../api/api'

interface SettingField {
  key: string
  label: string
  help: string
}

const SETTING_SECTIONS: { key: string; title: string; fields: SettingField[] }[] = [
  {
    key: 'base',
    title: '基础信息',
    fields: [
      { key: 'portal_name', label: '门户名称', help: '顶栏、登录页与浏览器标签页标题。' },
      { key: 'portal_welcome', label: '欢迎语', help: '顶栏品牌名下方，以及登录页品牌区。' },
      { key: 'portal_footer', label: '页脚文案', help: '登录页底部；留空展示默认版权信息。' },
    ],
  },
  {
    key: 'notice',
    title: '公告',
    fields: [{ key: 'announcement', label: '工作台公告', help: '当前版本没有共享门户配置服务。' }],
  },
]

/**
 * 当前后端没有门户设置表和审计化配置服务，因此这里明确只读，避免
 * 展示一个必然返回 501/404 的“保存”按钮。生产修改应走配置发布流程。
 */
export default function AdminSettings() {
  usePageTitle('门户设置')
  const { data: settings, isLoading } = useQuery({
    queryKey: queryKeys.portalSettings,
    queryFn: getPortalSettings,
  })
  const valueOf = (key: string) => settings?.find((s) => s.key === key)?.value || '未配置'

  return (
    <div className="velora-admin-settings">
      <AdminPageHead title="门户设置" />
      <Alert
        type="info"
        showIcon
        message="当前版本为只读配置基线"
        description="门户设置服务尚未接入。生产环境请通过版本化配置与审批发布修改，不在浏览器中写入本地或未审计配置。"
        style={{ marginBottom: 16 }}
      />

      {isLoading ? (
        <div className="velora-admin-card velora-settings-loading"><Spin /></div>
      ) : (
        <>
          {SETTING_SECTIONS.map((section) => (
            <section key={section.key} className="velora-admin-card velora-settings-section">
              <header className="velora-settings-section-head">
                <h3 className="velora-settings-section-title">{section.title}</h3>
              </header>
              {section.fields.map((field) => (
                <ReadOnlyRow key={field.key} label={field.label} help={field.help} value={valueOf(field.key)} />
              ))}
            </section>
          ))}
          <section className="velora-admin-card velora-settings-section">
            <header className="velora-settings-section-head">
              <h3 className="velora-settings-section-title">界面显示</h3>
            </header>
            <ReadOnlyRow label="界面缩放（%）" help="当前版本使用固定基线。" value={`${Math.round(Number.parseFloat(valueOf('ui_scale')) * 100) || 100}%`} />
            <ReadOnlyRow label="新应用标识（天）" help="当前版本使用固定基线。" value={`${Number.parseInt(valueOf('new_badge_days'), 10) || 7} 天`} />
          </section>
        </>
      )}
    </div>
  )
}

function ReadOnlyRow({ label, help, value }: { label: string; help: string; value: string }) {
  return (
    <div className="velora-settings-row">
      <div className="velora-settings-row-text">
        <div className="velora-settings-row-label">{label}</div>
        <div className="velora-settings-row-help">{help}</div>
      </div>
      <Typography.Text type="secondary" className="velora-settings-row-control velora-settings-row-control--fixed">
        {value}
      </Typography.Text>
    </div>
  )
}
