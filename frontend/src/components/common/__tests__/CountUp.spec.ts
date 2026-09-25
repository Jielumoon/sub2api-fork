import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import CountUp from '../CountUp.vue'

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

// 全局 setup 的 matchMedia 对所有查询都返回 true（即"减少动态效果"）；滚动用例需要显式关闭它。
function allowMotion() {
  vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => ({
    matches: !query.includes('prefers-reduced-motion'),
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn()
  }) as unknown as MediaQueryList)
}

async function advance(ms: number) {
  await vi.advanceTimersByTimeAsync(ms)
  await nextTick()
}

describe('CountUp', () => {
  it('整数目标从 0 滚动，中间帧保持整数，最终落在格式化后的目标值', async () => {
    allowMotion()
    vi.useFakeTimers({ toFake: ['requestAnimationFrame', 'cancelAnimationFrame', 'Date', 'performance'] })
    const wrapper = mount(CountUp, { props: { value: 1000, format: (v: number) => `${v} 次`, duration: 800 } })
    await nextTick()

    await advance(300)
    const mid = wrapper.text()
    expect(mid).toMatch(/^\d+ 次$/)
    expect(Number.parseInt(mid)).toBeGreaterThan(0)
    expect(Number.parseInt(mid)).toBeLessThan(1000)

    await advance(800)
    expect(wrapper.text()).toBe('1000 次')
  })

  it('小数目标的中间帧按目标小数位取整，数据刷新时从旧值滚到新值', async () => {
    allowMotion()
    vi.useFakeTimers({ toFake: ['requestAnimationFrame', 'cancelAnimationFrame', 'Date', 'performance'] })
    const wrapper = mount(CountUp, { props: { value: 12.5 } })
    await advance(1000)
    expect(wrapper.text()).toBe('12.5')

    await wrapper.setProps({ value: 20.25 })
    await advance(300)
    const mid = Number(wrapper.text())
    expect(mid).toBeGreaterThan(12.5)
    expect(mid).toBeLessThan(20.25)
    expect(wrapper.text().split('.')[1]?.length ?? 0).toBeLessThanOrEqual(2)

    await advance(1000)
    expect(wrapper.text()).toBe('20.25')
  })

  it('开启减少动态效果时直接显示目标值', async () => {
    const wrapper = mount(CountUp, { props: { value: 1234, format: (v: number) => v.toLocaleString() } })
    await nextTick()
    expect(wrapper.text()).toBe('1,234')
  })

  it('非有限值按 0 处理', () => {
    const wrapper = mount(CountUp, { props: { value: undefined } })
    expect(wrapper.text()).toBe('0')
  })
})
