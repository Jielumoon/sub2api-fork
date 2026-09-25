import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, it } from 'vitest'
import postcss from 'postcss'
import tailwindcss from 'tailwindcss'
import config from '../../tailwind.config.js'

const luminance = (color) => color.match(/[\d.]+/g).slice(0, 3)
  .map(Number)
  .map((value) => value / 255)
  .map((value) => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4)
  .reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0)

it('主按钮编译后的默认与悬停颜色满足白字 4.5:1 对比度', async () => {
  const source = resolve('src/style.css')
  const result = await postcss([tailwindcss({
    ...config, content: [{ raw: '<button class="btn-primary">提交</button>', extension: 'html' }]
  })]).process(readFileSync(source, 'utf8'), { from: source })
  const colors = {}
  result.root.walkRules((rule) => {
    if (!['.btn-primary', '.btn-primary:hover'].includes(rule.selector)) return
    rule.walkDecls((decl) => {
      if (['color', 'background-color'].includes(decl.prop)) {
        (colors[rule.selector] ??= {})[decl.prop] = decl.value
      }
    })
  })
  const foreground = luminance(colors['.btn-primary'].color)
  for (const selector of ['.btn-primary', '.btn-primary:hover']) {
    const background = luminance(colors[selector]['background-color'])
    expect((Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05)).toBeGreaterThanOrEqual(4.5)
  }
})
