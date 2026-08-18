import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntdApp, Button, Dropdown, Empty, Skeleton } from 'antd'
import {
  CheckCircleOutlined,
  DeleteOutlined,
  MailOutlined,
  MoreOutlined,
  ReloadOutlined,
  StarFilled,
  SyncOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import {
  listMailAccounts,
  listMailMessages,
  setMailRead,
  syncMailAccount,
  testMailAccount,
  unbindMailAccount,
  queryKeys,
} from '../../api/api'
import QueryErrorState from '../QueryErrorState'
import MailBindModal from './MailBindModal'
import { formatRelativeTime } from '../../utils/format'
import type { MailAccount, MailMessage } from '../../types'

interface Props {
  onOpenMail: (id: number) => void
  onConvert: (msg: MailMessage) => void
}

/** 邮件时间：今天显示时刻，否则显示日期 */
function formatMailTime(iso?: string | null): string {
  if (!iso) return ''
  const d = dayjs(iso)
  if (!d.isValid()) return ''
  return d.isSame(dayjs(), 'day') ? d.format('HH:mm') : d.format('MM-DD')
}

/**
 * 邮件 Tab：企业邮箱收件箱（Mail 独立领域）。
 * 第一阶段单账号体验（UI 预留多账号扩展）；同步 = 手动按钮 + 服务端定时补偿。
 */
export default function MailPanel({ onOpenMail, onConvert }: Props) {
  const { message: msgApi, modal } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [bindOpen, setBindOpen] = useState(false)

  const accountsQuery = useQuery({ queryKey: queryKeys.mailAccounts, queryFn: listMailAccounts })
  const accounts = accountsQuery.data ?? []
  const account: MailAccount | undefined = accounts[0]

  const messagesQuery = useQuery({
    queryKey: queryKeys.mailMessages({ pageSize: 8 }),
    queryFn: () => listMailMessages({ pageSize: 8 }),
    enabled: accounts.length > 0,
  })

  const invalidateMail = () => {
    queryClient.invalidateQueries({ queryKey: ['mail'] })
  }

  const syncMutation = useMutation({
    mutationFn: (id: number) => syncMailAccount(id),
    onSuccess: (acc) => {
      msgApi.success(`同步完成，未读 ${acc.unreadCount} 封`)
      invalidateMail()
    },
    onError: (err) => {
      msgApi.error(err instanceof Error ? err.message : '同步失败')
      invalidateMail()
    },
  })

  const readMutation = useMutation({
    mutationFn: ({ id, read }: { id: number; read: boolean }) => setMailRead(id, read),
    onSuccess: invalidateMail,
    onError: (err) => msgApi.error(err instanceof Error ? err.message : '操作失败'),
  })

  const testMutation = useMutation({
    mutationFn: (id: number) => testMailAccount(id),
    onSuccess: (res) => {
      if (res.ok) msgApi.success('连接正常')
      else msgApi.error(res.error || '连接失败')
    },
    onError: (err) => msgApi.error(err instanceof Error ? err.message : '测试失败'),
  })

  const unbindMutation = useMutation({
    mutationFn: (id: number) => unbindMailAccount(id),
    onSuccess: () => {
      msgApi.success('已解绑邮箱')
      invalidateMail()
    },
    onError: (err) => msgApi.error(err instanceof Error ? err.message : '解绑失败'),
  })

  const confirmUnbind = (acc: MailAccount) => {
    modal.confirm({
      title: '解绑邮箱',
      content: `解绑后将删除 ${acc.email} 在门户中的邮件缓存，且不再同步。确定解绑？`,
      okText: '解绑',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: () => unbindMutation.mutate(acc.id),
    })
  }

  if (accountsQuery.isLoading) {
    return <Skeleton active paragraph={{ rows: 3 }} style={{ padding: '4px 18px 16px' }} />
  }
  if (accountsQuery.isError) {
    return <QueryErrorState compact refetch={accountsQuery.refetch} />
  }

  // 未绑定：引导卡
  if (!account) {
    return (
      <div className="velora-myapps-state">
        <div className="velora-myapps-guide">
          <MailOutlined className="velora-mail-bind-icon" />
          <span className="velora-myapps-guide-text">绑定企业邮箱，新邮件在这里直接处理</span>
          <Button size="small" type="primary" onClick={() => setBindOpen(true)}>
            绑定邮箱
          </Button>
        </div>
        <MailBindModal
          open={bindOpen}
          onClose={() => setBindOpen(false)}
          onBound={(acc) => {
            setBindOpen(false)
            invalidateMail()
            syncMutation.mutate(acc.id)
          }}
        />
      </div>
    )
  }

  const items = messagesQuery.data?.items ?? []

  return (
    <div className="velora-mail-panel">
      {/* 账号工具栏 */}
      <div className="velora-mail-toolbar">
        <span className="velora-mail-account">
          <MailOutlined /> {account.email}
        </span>
        {account.status === 'error' ? (
          <span className="velora-mail-status is-error" title={account.lastError}>
            同步异常
          </span>
        ) : (
          <span className="velora-mail-sync-time">
            {account.lastSyncAt ? `${formatRelativeTime(account.lastSyncAt)}同步` : '尚未同步'}
          </span>
        )}
        <span className="velora-mail-toolbar-ops">
          <a onClick={() => syncMutation.mutate(account.id)}>
            {syncMutation.isPending ? <SyncOutlined spin /> : <ReloadOutlined />} 同步
          </a>
          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                { key: 'test', icon: <CheckCircleOutlined />, label: '测试连接' },
                { key: 'unbind', icon: <DeleteOutlined />, label: '解绑邮箱', danger: true },
              ],
              onClick: ({ key }) => {
                if (key === 'test') testMutation.mutate(account.id)
                if (key === 'unbind') confirmUnbind(account)
              },
            }}
          >
            <a onClick={(e) => e.preventDefault()}>
              <MoreOutlined />
            </a>
          </Dropdown>
        </span>
      </div>

      {/* 邮件列表 */}
      {messagesQuery.isLoading ? (
        <Skeleton active paragraph={{ rows: 3 }} style={{ padding: '4px 18px 8px' }} />
      ) : messagesQuery.isError ? (
        <QueryErrorState compact refetch={messagesQuery.refetch} />
      ) : items.length > 0 ? (
        <div className="velora-mail-list">
          {items.map((msg) => (
            <div
              key={msg.id}
              className={msg.isRead ? 'velora-mail-item is-read' : 'velora-mail-item'}
              role="button"
              tabIndex={0}
              onClick={() => onOpenMail(msg.id)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  onOpenMail(msg.id)
                }
              }}
            >
              {!msg.isRead && <span className="velora-mail-dot" />}
              <span className="velora-mail-main">
                <span className="velora-mail-line1">
                  <span className="velora-mail-from">{msg.fromName || msg.fromAddress || '未知发件人'}</span>
                  <span className="velora-mail-time">{formatMailTime(msg.receivedAt)}</span>
                </span>
                <span className="velora-mail-subject">
                  {msg.isStarred && <StarFilled className="velora-mail-star" />}
                  {msg.subject || '（无主题）'}
                </span>
              </span>
              <span className="velora-mail-ops">
                <a
                  onClick={(e) => {
                    e.stopPropagation()
                    onConvert(msg)
                  }}
                >
                  转待办
                </a>
                <a
                  onClick={(e) => {
                    e.stopPropagation()
                    readMutation.mutate({ id: msg.id, read: !msg.isRead })
                  }}
                >
                  {msg.isRead ? '置未读' : '置已读'}
                </a>
              </span>
            </div>
          ))}
        </div>
      ) : (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={account.lastSyncAt ? '收件箱为空' : '尚未同步，点击右上角「同步」拉取邮件'}
          style={{ padding: '24px 18px' }}
        />
      )}
    </div>
  )
}
