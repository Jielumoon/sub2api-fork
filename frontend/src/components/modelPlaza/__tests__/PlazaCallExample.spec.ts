import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PlazaCallExample from '../PlazaCallExample.vue'
import { buildPlazaCatalog } from '@/utils/plazaCatalog'
import type { PlazaModel } from '@/api/modelPlaza'
import { plazaGroup, requestPricing, tokenModel } from '@/utils/__tests__/plazaFixtures'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const { copyToClipboard } = vi.hoisted(() => ({ copyToClipboard: vi.fn() }))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: ref(false), copyToClipboard })
}))

const ROOT = 'https://api.example.com'

/** 每个模型放进各自平台的分组，合并成一个目录条目。 */
function mountExample(...models: PlazaModel[]) {
  const groups = models.map((model, i) => plazaGroup({ id: i + 1, platform: model.platform, models: [model] }))
  const [entry] = buildPlazaCatalog(groups)
  return mount(PlazaCallExample, { props: { entry, baseUrl: ROOT } })
}

const code = (wrapper: ReturnType<typeof mountExample>) => wrapper.get('[data-example-code]').text()
const tabs = (wrapper: ReturnType<typeof mountExample>) => wrapper.findAll('[role="tab"]')

describe('PlazaCallExample 端点', () => {
  it('Anthropic 走 /v1/messages，模型名写进请求体', () => {
    const text = code(mountExample(tokenModel()))
    expect(text).toContain(`curl '${ROOT}/v1/messages'`)
    expect(text).toContain('x-api-key: sk-xxx')
    expect(text).toContain('anthropic-version: 2023-06-01')
    expect(text).toContain('"model": "claude-sonnet"')
  })

  it('同名跨平台时每个端点一个标签页，Antigravity 带 /antigravity 前缀', async () => {
    const wrapper = mountExample(tokenModel(), tokenModel({ platform: 'antigravity' }))
    expect(tabs(wrapper).map((t) => t.text())).toEqual(['Anthropic Messages', 'Antigravity · Messages'])
    expect(tabs(wrapper)[0].attributes('aria-selected')).toBe('true')

    await tabs(wrapper)[1].trigger('click')
    expect(tabs(wrapper)[1].attributes('aria-selected')).toBe('true')
    expect(code(wrapper)).toContain(`curl '${ROOT}/antigravity/v1/messages'`)
  })

  it('Gemini 走 v1beta generateContent；Antigravity 下的 gemini 模型走 /antigravity/v1beta', () => {
    const gemini = code(mountExample(tokenModel({ name: 'gemini-2.5-pro', platform: 'gemini' })))
    expect(gemini).toContain(`${ROOT}/v1beta/models/gemini-2.5-pro:generateContent`)
    expect(gemini).toContain('x-goog-api-key: sk-xxx')

    const ag = code(mountExample(tokenModel({ name: 'gemini-2.5-pro', platform: 'antigravity' })))
    expect(ag).toContain(`${ROOT}/antigravity/v1beta/models/gemini-2.5-pro:generateContent`)
  })

  it('OpenAI 兼容平台走 /v1/chat/completions，Bearer 鉴权', () => {
    for (const platform of ['openai', 'deepseek', 'kimi', 'opencode_go']) {
      const text = code(mountExample(tokenModel({ name: 'm', platform })))
      expect(text).toContain(`curl '${ROOT}/v1/chat/completions'`)
      expect(text).toContain('Authorization: Bearer sk-xxx')
    }
  })

  it('图片计费按平台选协议：Gemini / Antigravity 走 generateContent，其余非 OpenAI/Grok 平台不给示例', () => {
    const image = requestPricing({ billing_mode: 'image', per_request_price: 0.04 })
    const gemini = code(mountExample(tokenModel({ name: 'gemini-2.5-flash-image', platform: 'gemini', pricing: image })))
    expect(gemini).toContain(`${ROOT}/v1beta/models/gemini-2.5-flash-image:generateContent`)
    expect(gemini).not.toContain('/images/generations')

    const ag = code(mountExample(tokenModel({ name: 'nano-banana', platform: 'antigravity', pricing: image })))
    expect(ag).toContain(`${ROOT}/antigravity/v1beta/models/nano-banana:generateContent`)

    expect(code(mountExample(tokenModel({ name: 'grok-imagine-image', platform: 'grok', pricing: image })))).toContain(
      `${ROOT}/v1/images/generations`
    )
    for (const platform of ['anthropic', 'kimi', 'deepseek']) {
      expect(mountExample(tokenModel({ name: 'img', platform, pricing: image })).html()).toBe('<!--v-if-->')
    }
  })

  it('图片计费走 /v1/images/generations；按次计费的工具模型不给示例', () => {
    const image = code(
      mountExample(tokenModel({ name: 'gpt-image-2', platform: 'openai', pricing: requestPricing({ billing_mode: 'image', per_request_price: 0.04 }) }))
    )
    expect(image).toContain(`curl '${ROOT}/v1/images/generations'`)
    expect(image).toContain('"prompt"')

    const tool = mountExample(tokenModel({ name: 'search', platform: 'openai', pricing: requestPricing({ per_request_price: 0.01 }) }))
    expect(tool.html()).toBe('<!--v-if-->')
  })

  it('请求体是合法 JSON', () => {
    const text = code(mountExample(tokenModel()))
    const body = shellWords(text).at(-1)!
    expect(JSON.parse(body)).toEqual({ model: 'claude-sonnet', max_tokens: 1024, messages: [{ role: 'user', content: 'Hello' }] })
    // 每个字段一行，数组不展开
    expect(text).toContain('    "messages": [{"role":"user","content":"Hello"}]')
    expect(text).toContain(`curl '${ROOT}/v1/messages'`)
  })
})

/**
 * 按 POSIX shell 规则切分命令（只实现示例会用到的：空白分词、单引号、反斜杠续行 / 转义）。
 * 能正确切回原始参数、且没有多出的词，就说明任何配置内容都没法逃出引号插入命令。
 */
function shellWords(command: string): string[] {
  const words: string[] = []
  let word: string | null = null
  let quoted = false
  for (let i = 0; i < command.length; i++) {
    const ch = command[i]
    if (quoted) {
      if (ch === "'") quoted = false
      else word += ch
      continue
    }
    if (ch === "'") {
      quoted = true
      word ??= ''
    } else if (ch === '\\') {
      const next = command[++i]
      if (next !== '\n') word = (word ?? '') + next
    } else if (/\s/.test(ch)) {
      if (word != null) words.push(word)
      word = null
    } else {
      word = (word ?? '') + ch
    }
  }
  expect(quoted).toBe(false)
  if (word != null) words.push(word)
  return words
}

describe('PlazaCallExample shell 引用', () => {
  it('模型名和站点地址里的单引号、分号、$() 都留在各自参数里，不会变成额外命令', () => {
    const name = `it's'; rm -rf ~ #$(id)`
    const root = `https://api.example.com/o'k`
    const groups = [plazaGroup({ platform: 'anthropic', models: [tokenModel({ name })] })]
    const [entry] = buildPlazaCatalog(groups)
    const wrapper = mount(PlazaCallExample, { props: { entry, baseUrl: root } })
    const words = shellWords(code(wrapper))
    expect(words).toEqual([
      'curl',
      `${root}/v1/messages`,
      '-H',
      'x-api-key: sk-xxx',
      '-H',
      'anthropic-version: 2023-06-01',
      '-H',
      'Content-Type: application/json',
      '-d',
      expect.any(String)
    ])
    expect(JSON.parse(words[9])).toMatchObject({ model: name })
  })

  it('Gemini 路径里的模型名做 URL 编码', () => {
    const [entry] = buildPlazaCatalog([plazaGroup({ platform: 'gemini', models: [tokenModel({ name: "g'e mini", platform: 'gemini' })] })])
    const words = shellWords(code(mount(PlazaCallExample, { props: { entry, baseUrl: ROOT } })))
    expect(words[1]).toBe(`${ROOT}/v1beta/models/g'e%20mini:generateContent`)
  })
})

describe('PlazaCallExample 交互', () => {
  it('复制的内容与展示一致', async () => {
    copyToClipboard.mockClear()
    const wrapper = mountExample(tokenModel())
    await wrapper.findAll('button').find((b) => b.text().includes('modelPlaza.example.copy'))!.trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(code(wrapper), 'modelPlaza.example.copySuccess')
  })

  it('方向键在标签页间循环切换', async () => {
    const wrapper = mountExample(tokenModel(), tokenModel({ platform: 'antigravity' }))
    await tabs(wrapper)[0].trigger('keydown', { key: 'ArrowLeft' })
    expect(tabs(wrapper)[1].attributes('aria-selected')).toBe('true')
    await tabs(wrapper)[1].trigger('keydown', { key: 'ArrowRight' })
    expect(tabs(wrapper)[0].attributes('aria-selected')).toBe('true')
  })
})
