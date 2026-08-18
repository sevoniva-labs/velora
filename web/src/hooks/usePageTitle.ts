// 页面标题：按页面设置 document.title，企业门户各页面有独立标签页标题。
import { useEffect } from 'react'

export function usePageTitle(title?: string) {
  useEffect(() => {
    document.title = title ? `${title} · Velora 企业应用门户` : 'Velora · 企业应用门户'
  }, [title])
}
