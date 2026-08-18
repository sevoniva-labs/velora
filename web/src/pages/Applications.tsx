import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Empty, Input, Pagination, Skeleton, Tag, Typography } from 'antd'
import { useSearchParams } from 'react-router-dom'
import { listApplications, listCategories, listTags, queryKeys } from '../api/api'
import { AppCard } from '../components/AppCard'

const PAGE_SIZE = 24

/** 应用中心：关键词搜索 + 分类筛选 + 标签筛选 + 分页网格。 */
export default function Applications() {
  const [searchParams, setSearchParams] = useSearchParams()
  const keyword = searchParams.get('keyword') ?? ''
  const categoryId = searchParams.get('categoryId') ?? ''
  const tagId = searchParams.get('tagId') ?? ''
  const page = Math.max(1, Number(searchParams.get('page') ?? 1) || 1)

  const [searchInput, setSearchInput] = useState(keyword)

  const { data: categories } = useQuery({ queryKey: queryKeys.categories, queryFn: listCategories })
  const { data: tags } = useQuery({ queryKey: queryKeys.tags, queryFn: listTags })

  const { data: pageData, isLoading } = useQuery({
    queryKey: queryKeys.applications({ keyword, categoryId, tagId, page }),
    queryFn: () =>
      listApplications({
        keyword: keyword || undefined,
        categoryId: categoryId ? Number(categoryId) : undefined,
        tagIds: tagId ? [Number(tagId)] : undefined,
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
      <div style={{ padding: '20px 0 8px' }}>
        <Typography.Title level={3} style={{ marginBottom: 4 }}>
          应用中心
        </Typography.Title>
        <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 0 }}>
          企业全部数字化应用的统一入口，共 {pageData?.total ?? 0} 个可用应用。
        </Typography.Paragraph>
      </div>

      {/* 搜索 */}
      <div style={{ margin: '16px 0' }}>
        <Input.Search
          size="large"
          allowClear
          placeholder="搜索应用：名称 / 编码 / 描述 / 标签 / 关键词"
          enterButton="搜索"
          defaultValue={keyword}
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          onSearch={(value) => updateParam({ keyword: value.trim() })}
          style={{ maxWidth: 560 }}
        />
      </div>

      {/* 分类筛选 */}
      <div className="velora-filter-row">
        <span className="velora-filter-label">全部 /</span>
        <button
          type="button"
          className={!categoryId ? 'velora-filter-chip is-active' : 'velora-filter-chip'}
          onClick={() => updateParam({ categoryId: '' })}
        >
          全部
        </button>
        {(categories ?? []).map((c) => (
          <button
            key={c.id}
            type="button"
            className={String(c.id) === categoryId ? 'velora-filter-chip is-active' : 'velora-filter-chip'}
            onClick={() => updateParam({ categoryId: String(c.id) })}
          >
            {c.name}
          </button>
        ))}
      </div>

      {/* 标签筛选 */}
      <div className="velora-filter-row">
        <span className="velora-filter-label">标签 /</span>
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

      {/* 已选筛选条件 */}
      {activeFilters.length > 0 && (
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', margin: '12px 0' }}>
          {activeFilters.map((f) => (
            <Tag key={f.key} closable onClose={f.onClose} style={{ padding: '2px 10px' }}>
              {f.label}
            </Tag>
          ))}
        </div>
      )}

      {/* 应用网格 */}
      {isLoading ? (
        <Skeleton active paragraph={{ rows: 6 }} />
      ) : pageData && pageData.items.length > 0 ? (
        <>
          <div className="velora-app-grid" style={{ marginTop: 8 }}>
            {pageData.items.map((app) => (
              <div key={app.id} className="velora-app-grid-item">
                <AppCard app={app} />
              </div>
            ))}
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 24 }}>
            <Pagination
              current={page}
              total={pageData.total}
              pageSize={PAGE_SIZE}
              showSizeChanger={false}
              showTotal={(t) => `共 ${t} 个应用`}
              onChange={(p) => updateParam({ page: String(p) })}
            />
          </div>
        </>
      ) : (
        <Empty
          description="没有找到匹配的应用，试试更换关键词或清除筛选条件"
          style={{ background: 'var(--velora-bg-container)', borderRadius: 12, padding: '64px 0' }}
        />
      )}
    </div>
  )
}
