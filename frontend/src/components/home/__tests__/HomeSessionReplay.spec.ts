vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import HomeSessionReplay from '../HomeSessionReplay.vue'

// 与 i18n mock 一致：提示词就是 key 本身。
const PROMPT = 'home.replay.prompt'
// 首次播放 1300 起步（等窗口入场），重播 450 起步；之后逐字 45ms + 400 + 三拍停留（1400 + 900 + 1100）
const START = 1300
const REPLAY_START = 450
const AFTER_START = PROMPT.length * 45 + 400 + 1400 + 900 + 1100

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

// 全局 setup 的 matchMedia 对所有查询都返回 true（即"减少动态效果"）；播放用例需要显式关闭它。
function allowMotion() {
  vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => ({
    matches: !query.includes('prefers-reduced-motion'),
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }) as unknown as MediaQueryList)
}

function mountReplay() {
  return mount(HomeSessionReplay, {
    props: { baseUrl: 'https://api.example.com' },
    global: {
      stubs: {
        Icon: true,
        CountUp: { props: ['value'], template: '<span class="count">{{ value }}</span>' },
      },
    },
  })
}

async function advance(ms: number) {
  await vi.advanceTimersByTimeAsync(ms)
  await nextTick()
}

function state(wrapper: ReturnType<typeof mountReplay>) {
  const line = (name: string) => wrapper.get(`[data-line="${name}"]`)
  const hidden = (name: string) => line(name).classes().includes('is-hidden')
  return {
    prompt: line('prompt').text().replace('>', '').trim(),
    readHidden: hidden('read'),
    replyHidden: hidden('reply'),
    status: line('status').attributes('data-state'),
    tokens: wrapper.findAll('[data-tokens] .count').map((count) => count.text()),
  }
}

describe('HomeSessionReplay', () => {
  it('按顺序播放：打字 → 思考 → 工具调用 → 完成，并停在终帧', async () => {
    allowMotion()
    vi.useFakeTimers()
    const wrapper = mountReplay()
    await nextTick()

    expect(state(wrapper)).toMatchObject({ prompt: '', readHidden: true, replyHidden: true, status: 'idle' })

    await advance(START + 5 * 45)
    expect(state(wrapper).prompt).toBe(PROMPT.slice(0, 5))

    await advance((PROMPT.length - 5) * 45 + 400)
    expect(state(wrapper)).toMatchObject({ prompt: PROMPT, readHidden: true, status: 'spinning' })

    await advance(1400)
    expect(state(wrapper)).toMatchObject({ readHidden: false, replyHidden: true, status: 'spinning' })
    // 工具调用进行中圆点闪烁，下一拍才变绿
    expect(wrapper.get('[data-line="read"] span').classes()).toContain('replay-running')

    await advance(900)
    expect(wrapper.get('[data-line="read"] span').classes()).toContain('text-emerald-400')

    await advance(1100)
    expect(state(wrapper)).toMatchObject({
      readHidden: false,
      replyHidden: false,
      status: 'done',
      tokens: ['3204', '1187'],
    })
    expect(wrapper.get('figure').classes()).toContain('is-done')
    expect(vi.getTimerCount()).toBe(0)
  })

  it('重播会回到开头重新播放', async () => {
    allowMotion()
    vi.useFakeTimers()
    const wrapper = mountReplay()
    await advance(START + AFTER_START)
    expect(state(wrapper).replyHidden).toBe(false)

    await wrapper.get('button').trigger('click')
    expect(state(wrapper)).toMatchObject({ prompt: '', replyHidden: true, status: 'idle', tokens: ['0', '0'] })

    await advance(REPLAY_START + AFTER_START)
    expect(state(wrapper)).toMatchObject({ prompt: PROMPT, replyHidden: false })
  })

  it('播放中途卸载不留定时器', async () => {
    allowMotion()
    vi.useFakeTimers()
    const wrapper = mountReplay()
    await advance(800)
    expect(vi.getTimerCount()).toBeGreaterThan(0)

    wrapper.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('减少动效时首帧即终帧，不启动定时器', async () => {
    vi.useFakeTimers()
    const wrapper = mountReplay()
    await nextTick()

    expect(state(wrapper)).toMatchObject({
      prompt: PROMPT,
      readHidden: false,
      replyHidden: false,
      status: 'done',
      tokens: ['3204', '1187'],
    })
    expect(wrapper.find('.replay-cursor').exists()).toBe(false)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('给读屏器一段说明，终端内容本身对读屏器隐藏', () => {
    const wrapper = mountReplay()
    const figure = wrapper.get('[role="img"]')
    expect(figure.attributes('aria-label')).toBe('home.replay.ariaLabel')
    expect(figure.get('div').attributes('aria-hidden')).toBe('true')
    expect(wrapper.get('button').attributes('type')).toBe('button')
  })
})
