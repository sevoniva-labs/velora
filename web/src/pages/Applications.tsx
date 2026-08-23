import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Empty, Input, Pagination, Skeleton, Tag } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import { useSearchParams } from 'react-router-dom'
import { listApplications, listCategories, listTags, queryKeys } from '../api/api'
import { AppCard } from '../components/AppCard'
import QueryErrorState from '../components/QueryErrorState'
import { usePageTitle } from '../hooks/usePageTitle'

const PAGE_SIZE = 24

export function categoryFilterValue(value: string): string | undefined {
  const categoryId = value.trim()
  return categoryId || undefined
}

/** 应用中心：关键词搜索 + 分类筛选 + 标签筛选 + 分页网格。 */
export default function Applications() {
  usePageTitle('应用中心')

  const [searchParams, setSearchParams] = useSearchParams()
  const keyword = searchParams.get('keyword') ?? ''
  const categoryId = searchParams.get('categoryId') ?? ''
  const tagId = searchParams.get('tagId') ?? ''
  const page = Math.max(1, Number(searchParams.get('page') ?? 1) || 1)

  const [searchInput, setSearchInput] = useState(keyword)

  const { data: categories } = useQuery({ queryKey: queryKeys.categories, queryFn: listCategories })
  const { data: tags } = useQuery({ queryKey: queryKeys.tags, queryFn: listTags })

  // 全量列表（与首页共享缓存）：分类计数
  const { data: allAppsList } = useQuery({
    queryKey: queryKeys.applications({ pageSize: 500 }),
    queryFn: () => listApplications({ pageSize: 500 }),
  })
  const catCounts = useMemo(() => {
    const m = new Map<string | number, number>()
    allAppsList?.items.forEach((a) => {
      if (a.categoryId != null) m.set(a.categoryId, (m.get(a.categoryId) ?? 0) + 1)
    })
    return m
  }, [allAppsList])

  const { data: pageData, isLoading, isError, refetch } = useQuery({
    queryKey: queryKeys.applications({ keyword, categoryId, tagId, page }),
    queryFn: () =>
      listApplications({
        keyword: keyword || undefined,
        categoryId: categoryFilterValue(categoryId),
        tagIds: tagId ? [tagId] : undefined,
        page,
        pageSize: PAGE_SIZE,
      }),
  })

  const updateParam = (patch: Record<string, string>) => {
    const next = new URLSearchParams(searchParams)
    for (const [k, v] of Object.entries(patch)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    next.delete('page')
    setSearchParams(next, { replace: true })
  }

  const selectedCategory = categories?.find((c) => String(c.id) === categoryId)
  const selectedTag = tags?.find((t) => String(t.id) === tagId)

  const activeFilters = useMemo(() => {
    const list: { key: string; label: string; onClose: () => void }[] = []
    if (keyword) list.push({ key: 'kw', label: `搜索：${keyword}`, onClose: () => updateParam({ keyword: '' }) })
    if (selectedCategory) {
      list.push({
        key: 'cat',
        label: `分类：${selectedCategory.name}`,
        onClose: () => updateParam({ categoryId: '' }),
      })
    }
    if (selectedTag) {
      list.push({ key: 'tag', label: `标签：${selectedTag.name}`, onClose: () => updateParam({ tagId: '' }) })
    }
    return list
  }, [keyword, selectedCategory, selectedTag]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div>
      <section className="velora-panel velora-apps-filter">
        <div className="velora-panel-body" style={{ paddingTop: 10, paddingBottom: 10 }}>
          {/* 搜索 + 分类（同一工具栏行，紧凑） */}
          <div className="velora-filter-row velora-filter-row--search">
            <div className="velora-apps-search">
              <Input
                allowClear
                prefix={<SearchOutlined style={{ color: 'var(--velora-secondary)' }} />}
                placeholder="搜索应用：名称 / 编码 / 描述"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                onPressEnter={() => updateParam({ keyword: searchInput.trim() })}
                onClear={() => updateParam({ keyword: '' })}
              />
            </div>
            <span className="velora-filter-sep" />
            <span className="velora-filter-label">分类</span>
            <button
              type="button"
              className={!categoryId ? 'velora-filter-chip is-active' : 'velora-filter-chip'}
              onClick={() => updateParam({ categoryId: '' })}
            >
              全部
            </button>
            {(categories ?? []).map((c) => {
              const count = catCounts.get(c.id) ?? 0
              return (
                <button
                  key={c.id}
                  type="button"
                  className={String(c.id) === categoryId ? 'velora-filter-chip is-active' : 'velora-filter-chip'}
                  onClick={() => updateParam({ categoryId: String(c.id) })}
                >
                  {c.name}
                  {count > 0 && <span className="velora-chip-count">{count}</span>}
                </button>
              )
            })}
          </div>

          {/* 标签筛选 */}
          {(tags ?? []).length > 0 && (
            <div className="velora-filter-row">
              <span className="velora-filter-label">标签</span>
              {(tags ?? []).map((t) => (
                <button
                  key={t.id}
                  type="button"
                  className={String(t.id) === tagId ? 'velora-filter-chip is-active' : 'velora-filter-chip'}
                  onClick={() => updateParam({ tagId: String(t.id) })}
                >
                  {t.name}
                </button>
              ))}
            </div>
          )}

          {/* 已选筛选条件 */}
          {activeFilters.length > 0 && (
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', margin: '12px 0 2px' }}>
              {activeFilters.map((f) => (
                <Tag key={f.key} closable onClose={f.onClose} style={{ padding: '2px 10px' }}>
                  {f.label}
                </Tag>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* 应用网格：玻璃卡片直接悬浮于深蓝背景之上 */}
      {isLoading ? (
        <div className="velora-panel" style={{ marginTop: 14 }}>
          <Skeleton active paragraph={{ rows: 6 }} style={{ padding: '24px 18px' }} />
        </div>
      ) : isError ? (
        <div className="velora-panel" style={{ marginTop: 14 }}>
          <QueryErrorState refetch={refetch} />
        </div>
      ) : pageData && pageData.items.length > 0 ? (
        <>
          <div className="velora-app-grid" style={{ marginTop: 14 }}>
            {pageData.items.map((app) => (
              <div key={app.id} className="velora-app-grid-item">
                <AppCard app={app} />
              </div>
            ))}
          </div>
          {pageData.total > PAGE_SIZE && (
            <div className="velora-apps-pager">
              <Pagination
                current={page}
                total={pageData.total}
                pageSize={PAGE_SIZE}
                showSizeChanger={false}
                showTotal={(t) => `共 ${t} 个应用`}
                onChange={(p) => updateParam({ page: String(p) })}
              />
            </div>
          )}
        </>
      ) : (
        <div className="velora-panel" style={{ marginTop: 14 }}>
          <Empty description="没有找到匹配的应用，试试更换关键词或清除筛选条件" style={{ padding: '48px 0' }} />
        </div>
      )}
    </div>
  )
}
