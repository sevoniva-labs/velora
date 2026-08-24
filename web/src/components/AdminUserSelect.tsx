import { useMemo, useState } from 'react'
import { Select } from 'antd'
import { useQueries, useQuery } from '@tanstack/react-query'
import { adminGetUser, adminPageUsers } from '../api/api'
import type { AdminUser } from '../types'

interface AdminUserSelectProps {
  value?: string | string[]
  onChange?: (value: string | string[]) => void
  mode?: 'multiple'
  excludeIds?: string[]
  placeholder?: string
  disabled?: boolean
  onUserSelect?: (user: AdminUser) => void
}

function userOption(user: AdminUser) {
  return { label: `${user.displayName || user.loginName}（${user.loginName}）`, value: user.id, user }
}

/** 面向大组织的人员选择器：服务端检索，且补取已选人员避免显示原始 ID。 */
export default function AdminUserSelect({ value, onChange, mode, excludeIds = [], placeholder = '搜索姓名或账号', disabled, onUserSelect }: AdminUserSelectProps) {
  const [keyword, setKeyword] = useState('')
  const selectedIds = useMemo(() => (Array.isArray(value) ? value : value ? [value] : []), [value])
  const directory = useQuery({
    queryKey: ['admin', 'user-options', keyword],
    queryFn: () => adminPageUsers({ page: 1, pageSize: 50, keyword: keyword || undefined, status: 'ACTIVE' }),
  })
  const selectedUsers = useQueries({
    queries: selectedIds.map((id) => ({ queryKey: ['admin', 'users', id], queryFn: () => adminGetUser(id), staleTime: 60_000 })),
  })
  const excluded = useMemo(() => new Set(excludeIds), [excludeIds])
  const options = useMemo(() => {
    const byId = new Map<string, ReturnType<typeof userOption>>()
    for (const user of directory.data?.items ?? []) if (!excluded.has(user.id)) byId.set(user.id, userOption(user))
    for (const query of selectedUsers) if (query.data && !excluded.has(query.data.id)) byId.set(query.data.id, userOption(query.data))
    return [...byId.values()]
  }, [directory.data?.items, excluded, selectedUsers])

  return (
    <Select
      value={value}
      mode={mode}
      disabled={disabled}
      allowClear
      showSearch
      filterOption={false}
      placeholder={placeholder}
      options={options}
      loading={directory.isFetching}
      onSearch={setKeyword}
      onOpenChange={(open: boolean) => { if (!open) setKeyword('') }}
      onChange={(next) => onChange?.(next)}
      onSelect={(id) => {
        const selected = options.find((option) => option.value === id)?.user
        if (selected) onUserSelect?.(selected)
      }}
      style={{ width: '100%' }}
    />
  )
}
