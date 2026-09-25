import { describe, expect, it } from 'vitest'
import { toApiRoot } from '../url'

describe('toApiRoot', () => {
  it.each([
    ['https://a.com', 'https://a.com'],
    ['https://a.com/', 'https://a.com'],
    ['https://a.com/v1', 'https://a.com'],
    ['https://a.com/v1/', 'https://a.com'],
    // 先去斜杠再去 /v1，否则会留下 /v1，客户端请求 /v1/v1/messages
    ['https://a.com/v1//', 'https://a.com'],
    ['https://a.com/proxy/v1', 'https://a.com/proxy'],
    ['https://a.com/v10', 'https://a.com/v10'],
  ])('%s → %s', (input, expected) => {
    expect(toApiRoot(input)).toBe(expected)
  })
})
