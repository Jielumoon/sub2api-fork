import { describe, expect, it, vi } from 'vitest'
import { baseCompile } from '@intlify/message-compiler'
import { createI18n } from 'vue-i18n'

import en from '../locales/en'
import zh from '../locales/zh'

// vue-i18n 在运行时才编译消息：文案里未转义的花括号（如内嵌 JSON 示例
// "{\"user-agent\": ...}"）会在渲染时抛 "Invalid token in placeholder"，
// 直接炸掉整个组件树，且构建期完全无感。本测试把全部文案预编译一遍，
// 将该类问题固化为显式失败。字面量花括号请用 {'{'} / {'}'} 转义，
// 或将语言中立的示例文本（如 JSON）移出 i18n。
function collectCompileErrors(node: unknown, path: string, out: string[]): void {
  if (typeof node === 'string') {
    baseCompile(node, {
      onError: (err) => {
        out.push(`${path}: ${err.message}`)
      }
    })
    return
  }
  if (Array.isArray(node)) {
    node.forEach((item, index) => collectCompileErrors(item, `${path}[${index}]`, out))
    return
  }
  if (node && typeof node === 'object') {
    for (const [key, value] of Object.entries(node as Record<string, unknown>)) {
      collectCompileErrors(value, path ? `${path}.${key}` : key, out)
    }
  }
}

describe('locale messages compile', () => {
  it('严格 CSP 禁止动态编译时仍能插值翻译消息', () => {
    const functionConstructor = vi.spyOn(globalThis, 'Function').mockImplementation(() => {
      throw new EvalError('CSP 禁止动态代码编译')
    })
    try {
      const i18n = createI18n({
        legacy: false,
        locale: 'zh',
        messages: { zh: { greeting: '严格 CSP 下你好，{name}' } }
      })
      expect(i18n.global.t('greeting', { name: '宝宝' })).toBe('严格 CSP 下你好，宝宝')
    } finally {
      functionConstructor.mockRestore()
    }
  })

  it.each([
    ['zh', zh],
    ['en', en]
  ] as const)('%s messages all compile without placeholder errors', (locale, messages) => {
    const errors: string[] = []
    collectCompileErrors(messages, locale, errors)
    expect(errors).toEqual([])
  })
})
