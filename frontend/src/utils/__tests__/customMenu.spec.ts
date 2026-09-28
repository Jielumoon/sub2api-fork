import { describe, expect, it } from 'vitest'
import { customMenuExternalUrl, customMenuMarkdownSlug, customMenuOpenMode } from '../customMenu'

const url = 'https://shop.example.com/s/abc'

describe('customMenu', () => {
  it('treats markdown items as markdown regardless of open_mode', () => {
    expect(customMenuMarkdownSlug({ url: 'md:guide' })).toBe('guide')
    expect(customMenuMarkdownSlug({ url, page_slug: 'faq' })).toBe('faq')
    expect(customMenuOpenMode({ url: 'md:guide', open_mode: 'new_tab' })).toBeNull()
    expect(customMenuExternalUrl({ url: 'md:guide', open_mode: 'new_tab' })).toBeNull()
  })

  it('keeps legacy items on embed but never sends user params for unknown open_mode', () => {
    expect(customMenuOpenMode({ url })).toBe('embed')
    expect(customMenuOpenMode({ url, open_mode: '' as never })).toBe('embed')
    expect(customMenuOpenMode({ url, open_mode: 'popup' as never })).toBe('embed_clean')
  })

  it.each(['embed', 'embed_clean', 'new_tab'] as const)('keeps open_mode %s', (mode) => {
    expect(customMenuOpenMode({ url, open_mode: mode })).toBe(mode)
  })

  it('returns the raw URL only for new_tab http(s) items', () => {
    expect(customMenuExternalUrl({ url, open_mode: 'new_tab' })).toBe(url)
    expect(customMenuExternalUrl({ url, open_mode: 'embed_clean' })).toBeNull()
    expect(customMenuExternalUrl({ url })).toBeNull()
    expect(customMenuExternalUrl({ url: 'javascript:alert(1)', open_mode: 'new_tab' })).toBeNull()
    expect(customMenuExternalUrl({ url: 'data:text/html,hi', open_mode: 'new_tab' })).toBeNull()
  })
})
