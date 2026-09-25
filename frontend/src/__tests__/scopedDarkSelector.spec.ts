import { describe, expect, it } from 'vitest'

// Vue scoped 样式会把 `:global(.dark) .foo` 整条替换成 `:global()` 的内容，
// 编译结果是裸的 `.dark { … }`，样式直接落到 <html class="dark"> 上、污染整页。
// 暗色覆盖应写成 `.dark .foo`（scoped 属性只加在最后一段）。
const sources = import.meta.glob('/src/**/*.vue', { query: '?raw', import: 'default', eager: true }) as Record<
  string,
  string
>

const SCOPED_STYLE = /<style\b[^>]*\bscoped\b[^>]*>([\s\S]*?)<\/style>/g
const GLOBAL_DARK_COMPOUND = /:global\(\s*\.dark\s*\)\s*[^\s{,]/

describe('scoped 样式里的暗色选择器', () => {
  it('不使用 :global(.dark) 加后代选择器的写法', () => {
    const offenders = Object.entries(sources).flatMap(([file, source]) =>
      [...source.matchAll(SCOPED_STYLE)]
        .flatMap((block) => block[1].split('\n'))
        .filter((line) => GLOBAL_DARK_COMPOUND.test(line) && !line.trim().startsWith('/*'))
        .map((line) => `${file}: ${line.trim()}`)
    )
    expect(offenders).toEqual([])
  })
})
