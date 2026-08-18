import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntdApp, Alert, Button, Drawer, Skeleton, Tooltip } from 'antd'
import { MailOutlined, StarFilled, StarOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import DOMPurify from 'dompurify'
import { getMailMessage, setMailRead, setMailStar, queryKeys } from '../../api/api'
import type { MailMessage } from '../../types'

interface Props {
  mailId: number | null
  onClose: () => void
  onConvert: (msg: MailMessage) => void
}

/** DOMPurify 基础配置：过滤 script/iframe/object/embed/form 等危险载体 */
const SANITIZE_BASE = {
  FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input', 'button', 'link', 'meta', 'base'],
  FORBID_ATTR: ['onerror', 'onclick', 'onload', 'onmouseover', 'onfocus', 'srcset'],
}

/**
 * 邮件详情抽屉。
 * 安全策略：HTML 经 DOMPurify 消毒；远程图片默认不加载（防追踪像素），用户手动放行。
 */
export default function MailDetailDrawer({ mailId, onClose, onConvert }: Props) {
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [showImages, setShowImages] = useState(false)

  const detailQuery = useQuery({
    queryKey: queryKeys.mailMessage(mailId ?? 0),
    queryFn: () => getMailMessage(mailId ?? 0),
    enabled: mailId != null,
  })
  const msg = detailQuery.data?.message
  const bodyError = detailQuery.data?.bodyError

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.mailAccounts })
    queryClient.invalidateQueries({ queryKey: ['mail', 'messages'] })
  }

  const readMutation = useMutation({
    mutationFn: (read: boolean) => setMailRead(mailId ?? 0, read),
    onSuccess: invalidate,
    onError: (err) => message.error(err instanceof Error ? err.message : '操作失败'),
  })
  const starMutation = useMutation({
    mutationFn: (starred: boolean) => setMailStar(mailId ?? 0, starred),
    onSuccess: invalidate,
    onError: (err) => message.error(err instanceof Error ? err.message : '操作失败'),
  })

  const hasRemoteImages = !!msg?.bodyHtml && /<img[\s>]/i.test(msg.bodyHtml)
  const cleanHtml = useMemo(() => {
    if (!msg?.bodyHtml) return ''
    const cfg = showImages
      ? SANITIZE_BASE
      : { ...SANITIZE_BASE, FORBID_TAGS: [...SANITIZE_BASE.FORBID_TAGS, 'img'] }
    return DOMPurify.sanitize(msg.bodyHtml, cfg)
  }, [msg?.bodyHtml, showImages])

  const close = () => {
    setShowImages(false)
    onClose()
  }

  return (
    <Drawer
      open={mailId != null}
      onClose={close}
      width={560}
      title={
        <span className="velora-mail-detail-title">
          <MailOutlined /> {msg?.subject || '邮件详情'}
        </span>
      }
      footer={
        msg && (
          <div className="velora-mail-detail-footer">
            <Button onClick={() => readMutation.mutate(!msg.isRead)} loading={readMutation.isPending}>
              {msg.isRead ? '标记未读' : '标记已读'}
            </Button>
            <Button
              icon={msg.isStarred ? <StarFilled style={{ color: '#faad14' }} /> : <StarOutlined />}
              onClick={() => starMutation.mutate(!msg.isStarred)}
              loading={starMutation.isPending}
            >
              {msg.isStarred ? '取消星标' : '星标'}
            </Button>
            <Button type="primary" onClick={() => onConvert(msg)}>
              转为待办
            </Button>
          </div>
        )
      }
    >
      {detailQuery.isLoading ? (
        <Skeleton active paragraph={{ rows: 6 }} />
      ) : detailQuery.isError ? (
        <Alert type="error" message="邮件加载失败" description="邮件可能已被删除，请刷新列表" showIcon />
      ) : msg ? (
        <div className="velora-mail-detail">
          <div className="velora-mail-detail-meta">
            <div className="velora-mail-detail-row">
              <span className="velora-mail-detail-label">发件人</span>
              <span>
                {msg.fromName ? `${msg.fromName} ` : ''}
                <span className="velora-mail-detail-addr">&lt;{msg.fromAddress}&gt;</span>
              </span>
            </div>
            {msg.toAddresses && (
              <div className="velora-mail-detail-row">
                <span className="velora-mail-detail-label">收件人</span>
                <span>{msg.toAddresses}</span>
              </div>
            )}
            <div className="velora-mail-detail-row">
              <span className="velora-mail-detail-label">时间</span>
              <span>{msg.receivedAt ? dayjs(msg.receivedAt).format('YYYY-MM-DD HH:mm') : '—'}</span>
              {msg.hasAttachment && (
                <Tooltip title="含附件（附件预览/下载将在后续版本支持）">
                  <span className="velora-mail-attach">含附件</span>
                </Tooltip>
              )}
            </div>
          </div>

          {bodyError && (
            <Alert
              type="warning"
              message="正文加载失败"
              description={bodyError}
              showIcon
              style={{ marginBottom: 12 }}
              action={
                <Button size="small" onClick={() => detailQuery.refetch()}>
                  重试
                </Button>
              }
            />
          )}

          {hasRemoteImages && !showImages && (
            <div className="velora-mail-imgnotice">
            此邮件包含外部图片，已默认拦截
              <a onClick={() => setShowImages(true)}>显示图片</a>
            </div>
          )}

          {cleanHtml ? (
            <div
              className="velora-mail-body"
              // eslint-disable-next-line react/no-danger -- 已经 DOMPurify 消毒
              dangerouslySetInnerHTML={{ __html: cleanHtml }}
            />
          ) : msg.bodyText ? (
            <div className="velora-mail-body is-text">{msg.bodyText}</div>
          ) : (
            !bodyError && <div className="velora-mail-body is-text">{msg.snippet || '（无正文）'}</div>
          )}
        </div>
      ) : null}
    </Drawer>
  )
}
