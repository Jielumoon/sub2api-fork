vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

// 模拟 useClipboard：复制后 copied 置 true，与真实实现一致。
const { copyToClipboard } = vi.hoisted(() => ({ copyToClipboard: vi.fn() }))
vi.mock('@/composables/useClipboard', async () => {
  const { ref } = await import('vue')
  const copied = ref(false)
  return {
    useClipboard: () => ({
      copied,
      copyToClipboard: (text: string, message?: string) => {
        copyToClipboard(text, message)
        copied.value = true
        return Promise.resolve(true)
      },
    }),
  }
})

import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useClipboard } from '@/composables/useClipboard'
import HomeConnectPanel from '../HomeConnectPanel.vue'

const BASE = 'https://api.example.com'

// 用例失败时也会卸载，避免挂在 body 上的旧面板带着重复 id 干扰后续焦点断言。
enableAutoUnmount(afterEach)

function mountPanel() {
  return mount(HomeConnectPanel, {
    props: { baseUrl: BASE },
    attachTo: document.body,
    global: { stubs: { Icon: true } },
  })
}

function files(wrapper: ReturnType<typeof mountPanel>) {
  return wrapper.findAll('section').map((section) => ({
    path: section.get('.font-code').text(),
    text: section.findAll('.snippet-line').map((line) => line.element.textContent).join('\n'),
    button: section.get('button').text(),
  }))
}

describe('HomeConnectPanel', () => {
  beforeEach(() => {
    copyToClipboard.mockClear()
    useClipboard().copied.value = false
  })

  it('每个客户端的片段都填入本站地址', async () => {
    const wrapper = mountPanel()
    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs.map((tab) => tab.text())).toEqual([
      'keys.useKeyModal.cliTabs.claudeCode',
      'keys.useKeyModal.cliTabs.codexCli',
      'keys.useKeyModal.cliTabs.geminiCli',
    ])

    expect(files(wrapper)[0].text).toContain(`export ANTHROPIC_BASE_URL="${BASE}"`)
    await tabs[1].trigger('click')
    expect(files(wrapper)[0].text).toContain(`base_url = "${BASE}/v1"`)
    await tabs[2].trigger('click')
    expect(files(wrapper)[0].text).toContain(`export GOOGLE_GEMINI_BASE_URL="${BASE}"`)
  })

  it('Codex 的两个文件分块展示，各自复制出合法内容', async () => {
    const wrapper = mountPanel()
    await wrapper.findAll('[role="tab"]')[1].trigger('click')

    const [config, auth] = files(wrapper)
    expect(config.path).toBe('~/.codex/config.toml')
    expect(auth.path).toBe('~/.codex/auth.json')
    expect(config.text).not.toContain('OPENAI_API_KEY')
    expect(JSON.parse(auth.text)).toEqual({ OPENAI_API_KEY: 'home.connect.keyPlaceholder' })

    await wrapper.findAll('section')[1].get('button').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledOnce()
    expect(copyToClipboard.mock.calls[0][0]).toBe(auth.text)
  })

  it('"已复制"只标在刚复制的文件上，换标签后不再显示', async () => {
    const wrapper = mountPanel()
    const tabs = wrapper.findAll('[role="tab"]')
    await tabs[1].trigger('click')
    await wrapper.findAll('section')[1].get('button').trigger('click')

    expect(files(wrapper).map((file) => file.button)).toEqual([
      'home.connect.copy ~/.codex/config.toml',
      'home.connect.copied ~/.codex/auth.json',
    ])

    await tabs[0].trigger('click')
    expect(files(wrapper)[0].button).toBe('home.connect.copy bash / zsh')
  })

  it('本站地址单独标出，密钥占位符提示需要替换', () => {
    const wrapper = mountPanel()
    const marked = wrapper.findAll('pre span').filter((span) => span.classes().includes('font-semibold'))
    expect(marked.map((span) => span.text())).toEqual([BASE])
    expect(wrapper.get('pre').text()).toContain('home.connect.keyPlaceholder')
  })

  it('方向键在标签间循环切换并移动焦点', async () => {
    const wrapper = mountPanel()
    const tabs = wrapper.findAll('[role="tab"]')

    await tabs[0].trigger('keydown', { key: 'ArrowLeft' })
    expect(tabs[2].attributes('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(tabs[2].element)

    await tabs[2].trigger('keydown', { key: 'ArrowRight' })
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    expect(tabs[0].attributes('tabindex')).toBe('0')
    expect(tabs[1].attributes('tabindex')).toBe('-1')
  })
})
