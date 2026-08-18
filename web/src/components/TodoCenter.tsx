import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntdApp, Empty, Skeleton } from 'antd'
import { CheckOutlined, MailOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { listMailAccounts, listTodos, markTodoDone, queryKeys } from '../api/api'
import QueryErrorState from './QueryErrorState'
import MailPanel from './mail/MailPanel'
import MailDetailDrawer from './mail/MailDetailDrawer'
import ConvertTodoModal from './mail/ConvertTodoModal'
import type { MailMessage, TodoItem, TodoKind, TodoPriority } from '../types'

/** 待办优先级 → 中文标签（色条/标签样式类名与优先级同名） */
const TODO_PRI: Record<TodoPriority, string> = { urgent: '紧急', high: '高', mid: '中', low: '低' }

/** 待办类型 Tab：固定顺序，仅展示有待办的类型（邮件 Tab 常驻，承载企业邮箱） */
const KIND_TABS: { kind: TodoKind; label: string }[] = [
  { kind: 'approval', label: '审批' },
  { kind: 'devops', label: '研发' },
  { kind: 'ops', label: '运维' },
  { kind: 'project', label: '项目' },
  { kind: 'hr', label: '人事' },
  { kind: 'other', label: '其他' },
]

type TabKey = 'all' | 'mail' | TodoKind

/** 待办到期展示：逾期/今天/明天红色警示，其余显示日期 */
function formatTodoDue(dueAt?: string | null): { text: string; warn: boolean } | null {
  if (!dueAt) return null
  const d = dayjs(dueAt)
  if (!d.isValid()) return null
  const today = dayjs().startOf('day')
  const day = d.startOf('day')
  if (day.isBefore(today)) return { text: '已逾期', warn: true }
  const diff = day.diff(today, 'day')
  if (diff === 0) return { text: '今天到期', warn: true }
  if (diff === 1) return { text: '明天到期', warn: true }
  return { text: d.format('MM-DD'), warn: false }
}

/**
 * 待办中心：多 Tab 工作中心。
 * - 全部 / 审批 / 研发 / 运维 …：Todo 领域（外部系统经 POST /todos 集成）
 * - 邮件：Mail 领域（企业邮箱收件箱），邮件默认不进待办，手动"转待办"建立关联
 */
export default function TodoCenter() {
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<TabKey>('all')
  const [mailDetailId, setMailDetailId] = useState<number | null>(null)
  const [convertTarget, setConvertTarget] = useState<MailMessage | null>(null)

  const todosQuery = useQuery({
    queryKey: queryKeys.todos,
    queryFn: () => listTodos({ status: 'open', limit: 50 }),
  })
  const accountsQuery = useQuery({ queryKey: queryKeys.mailAccounts, queryFn: listMailAccounts })

  const todos = useMemo(() => todosQuery.data?.items ?? [], [todosQuery.data])
  const kindCounts = useMemo(() => {
    const m = new Map<TodoKind, number>()
    todos.forEach((t) => m.set(t.kind, (m.get(t.kind) ?? 0) + 1))
    return m
  }, [todos])
  const mailUnread = useMemo(
    () => (accountsQuery.data ?? []).reduce((s, a) => s + (a.unreadCount || 0), 0),
    [accountsQuery.data],
  )

  const doneMutation = useMutation({
    mutationFn: markTodoDone,
    onSuccess: () => {
      message.success('已完成')
      queryClient.invalidateQueries({ queryKey: queryKeys.todos })
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '操作失败'),
  })

  // 点击待办：邮件来源的打开邮件详情；其余按 URL 跳转来源系统
  const openTodo = (todo: TodoItem) => {
    if (todo.sourceSystem === 'mail' && todo.sourceId) {
      setMailDetailId(Number(todo.sourceId))
      return
    }
    if (todo.url) window.open(todo.url, '_blank', 'noopener,noreferrer')
  }

  const filteredTodos = tab === 'all' || tab === 'mail' ? todos : todos.filter((t) => t.kind === tab)

  const tabs: { key: TabKey; label: string; count: number; isMail?: boolean }[] = [
    { key: 'all', label: '全部', count: todosQuery.data?.openCount ?? todos.length },
    { key: 'mail', label: '邮件', count: mailUnread, isMail: true },
    ...KIND_TABS.filter((k) => (kindCounts.get(k.kind) ?? 0) > 0).map((k) => ({
      key: k.kind as TabKey,
      label: k.label,
      count: kindCounts.get(k.kind) ?? 0,
    })),
  ]

  const headerMeta =
    tab === 'mail'
      ? mailUnread > 0
        ? `${mailUnread} 封未读`
        : '收件箱'
      : `${todosQuery.data?.openCount ?? todos.length} 项待处理`

  return (
    <section className="velora-panel">
      <div className="velora-panel-head">
        <h2 className="velora-panel-title">待办中心</h2>
        <div className="velora-panel-more">
          <span className="velora-todo-total">{headerMeta}</span>
        </div>
      </div>

      <div className="velora-todo-tabs" role="tablist" aria-label="待办类型">
        {tabs.map((t) => (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={tab === t.key}
            className={tab === t.key ? 'velora-todo-tab is-active' : 'velora-todo-tab'}
            onClick={() => setTab(t.key)}
          >
            {t.isMail && <MailOutlined />}
            {t.label}
            {t.count > 0 && (
              <span className={t.isMail ? 'velora-todo-tab-count is-mail' : 'velora-todo-tab-count'}>{t.count}</span>
            )}
          </button>
        ))}
      </div>

      {tab === 'mail' ? (
        <MailPanel onOpenMail={setMailDetailId} onConvert={setConvertTarget} />
      ) : todosQuery.isLoading ? (
        <Skeleton active paragraph={{ rows: 4 }} style={{ padding: '4px 18px 16px' }} />
      ) : todosQuery.isError ? (
        <QueryErrorState compact refetch={todosQuery.refetch} />
      ) : filteredTodos.length > 0 ? (
        <div className="velora-todo-list">
          {filteredTodos.map((todo) => {
            const pri = TODO_PRI[todo.priority] ? todo.priority : 'mid'
            const due = formatTodoDue(todo.dueAt)
            const clickable = todo.sourceSystem === 'mail' || !!todo.url
            return (
              <div
                key={todo.id}
                className="velora-todo-item"
                role={clickable ? 'button' : undefined}
                tabIndex={clickable ? 0 : undefined}
                onClick={() => openTodo(todo)}
                onKeyDown={(e) => {
                  if (clickable && (e.key === 'Enter' || e.key === ' ')) {
                    e.preventDefault()
                    openTodo(todo)
                  }
                }}
              >
                <span className={`velora-todo-pri velora-todo-pri--${pri}`} />
                <span className={`velora-todo-lv velora-todo-lv--${pri}`}>{TODO_PRI[pri]}</span>
                <span className="velora-todo-main">
                  <span className="velora-todo-title">{todo.title}</span>
                  <span className="velora-todo-from">{todo.sourceLabel || todo.sourceSystem}</span>
                </span>
                {due && <span className={due.warn ? 'velora-todo-due is-warn' : 'velora-todo-due'}>{due.text}</span>}
                <button
                  type="button"
                  className="velora-todo-done"
                  title="标记完成"
                  aria-label="标记完成"
                  onClick={(e) => {
                    e.stopPropagation()
                    doneMutation.mutate(todo.id)
                  }}
                >
                  <CheckOutlined />
                </button>
              </div>
            )
          })}
        </div>
      ) : (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={tab === 'all' ? '暂无待办事项' : '该类型暂无待办'}
          style={{ padding: '28px 18px' }}
        />
      )}

      <MailDetailDrawer mailId={mailDetailId} onClose={() => setMailDetailId(null)} onConvert={setConvertTarget} />
      <ConvertTodoModal mail={convertTarget} onClose={() => setConvertTarget(null)} />
    </section>
  )
}
