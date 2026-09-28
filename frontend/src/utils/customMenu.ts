/**
 * 自定义菜单项的打开方式判定：侧边栏与 CustomPageView 共用，保证两处口径一致。
 */
import type { CustomMenuItem, CustomMenuOpenMode } from '@/types'
import { sanitizeUrl } from '@/utils/url'

type MenuItemFields = Pick<CustomMenuItem, 'url' | 'page_slug' | 'open_mode'>

/** Markdown 页面的 slug（`page_slug` 优先，其次 `md:<slug>`），不是 Markdown 页面时为空串。 */
export function customMenuMarkdownSlug(item: MenuItemFields): string {
  if (item.page_slug) return item.page_slug
  if (item.url?.startsWith('md:')) return item.url.slice(3)
  return ''
}

/**
 * Markdown 页面返回 null；其余按 open_mode 解析。只有缺省（旧数据）和 embed 才附带用户参数，
 * 不认识的值（如回滚后遇到新版本写入的取值）一律按 embed_clean，宁可少传也不把 token 交给第三方。
 */
export function customMenuOpenMode(item: MenuItemFields): CustomMenuOpenMode | null {
  if (customMenuMarkdownSlug(item)) return null
  const mode: string = item.open_mode ?? ''
  if (mode === '' || mode === 'embed') return 'embed'
  return mode === 'new_tab' ? 'new_tab' : 'embed_clean'
}

/** new_tab 模式且 URL 是 http(s) 时返回原始 URL（不带任何用户参数），否则 null。 */
export function customMenuExternalUrl(item: MenuItemFields): string | null {
  if (customMenuOpenMode(item) !== 'new_tab') return null
  return sanitizeUrl(item.url ?? '') || null
}
