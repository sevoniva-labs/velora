import type { ReactNode } from 'react'
import { Button, Input, Space, Typography } from 'antd'

export interface AdminListScopeOption {
  label: string
  value: string
  count?: number
}

interface AdminListScopeProps {
  options: AdminListScopeOption[]
  value: string
  onChange: (value: string) => void
}

interface AdminListSearchProps {
  value: string
  placeholder: string
  onChange: (value: string) => void
  onSearch: (value: string) => void
  children?: ReactNode
}

export function AdminListScope({ options, value, onChange }: AdminListScopeProps) {
  return (
    <div className="velora-admin-list-scope" role="tablist" aria-label="列表范围">
      {options.map((option) => {
        const active = option.value === value
        return (
          <Button
            key={option.value}
            type="text"
            role="tab"
            aria-selected={active}
            className={`velora-admin-list-scope-item${active ? ' is-active' : ''}`}
            onClick={() => onChange(option.value)}
          >
            <span>{option.label}</span>
            {option.count != null && <span className="velora-admin-list-scope-count">{option.count}</span>}
          </Button>
        )
      })}
    </div>
  )
}

export function AdminListSearch({ value, placeholder, onChange, onSearch, children }: AdminListSearchProps) {
  return (
    <Space className="velora-admin-list-tools" size={12} wrap>
      <Input.Search
        className="velora-admin-list-search"
        value={value}
        placeholder={placeholder}
        allowClear
        enterButton
        onChange={(event) => {
          const next = event.target.value
          onChange(next)
          if (!next) onSearch('')
        }}
        onSearch={(next) => onSearch(next.trim())}
      />
      {children}
    </Space>
  )
}

export function AdminListTitle({ children }: { children: ReactNode }) {
  return <Typography.Text className="velora-admin-list-title" strong>{children}</Typography.Text>
}
