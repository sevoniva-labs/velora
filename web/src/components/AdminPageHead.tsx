// 管理后台统一页头：标题 + 描述 + 右侧操作区，保证各管理页结构一致。
import type { ReactNode } from 'react'

export interface AdminPageHeadProps {
  title: string
  desc?: string
  extra?: ReactNode
}

export default function AdminPageHead({ title, desc, extra }: AdminPageHeadProps) {
  return (
    <div className="velora-admin-page-head">
      <div>
        <h2 className="velora-admin-page-title">{title}</h2>
        {desc ? <p className="velora-admin-page-desc">{desc}</p> : null}
      </div>
      {extra ? <div className="velora-admin-page-extra">{extra}</div> : null}
    </div>
  )
}
