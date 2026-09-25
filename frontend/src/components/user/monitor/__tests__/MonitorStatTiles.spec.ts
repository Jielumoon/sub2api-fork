vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import MonitorStatTiles from '../MonitorStatTiles.vue'
import MonitorStatusStrip from '../MonitorStatusStrip.vue'

const percent = (v: number) => `${v.toFixed(2)}%`

// jsdom 没有 PointerEvent，手动带上 pointerType
function pointer(el: Element, type: string, pointerType: string) {
  const event = new Event(type)
  Object.defineProperty(event, 'pointerType', { value: pointerType })
  el.dispatchEvent(event)
}

describe('MonitorStatTiles', () => {
  it('null 与 NaN 都显示 -，不会被兜成 0', () => {
    const wrapper = mount(MonitorStatTiles, {
      props: {
        items: [
          { label: 'a', value: null, format: percent },
          { label: 'b', value: Number.NaN, format: percent },
          { label: 'c', value: 99.5, format: percent },
        ],
      },
    })
    const values = wrapper.findAll('.font-mono').map((cell) => cell.text())
    expect(values.slice(0, 2)).toEqual(['-', '-'])
    expect(values[2]).not.toBe('-')
  })

  it('连续色优先于 toneClass，以内联样式生效', () => {
    const wrapper = mount(MonitorStatTiles, {
      props: { items: [{ label: 'a', value: 50, format: percent, toneClass: 'text-red-600', color: 'hsl(60 70% 45%)' }] },
    })
    const cell = wrapper.find('.font-mono')
    // jsdom 会把 hsl() 规范化成 rgb()
    expect(cell.attributes('style')).toMatch(/color:\s*rgb\(/)
    expect(cell.classes()).not.toContain('text-red-600')
  })
})

describe('MonitorStatusStrip', () => {
  it('给读屏器汇总状态分布，补位空块不计入', () => {
    const wrapper = mount(MonitorStatusStrip, {
      props: {
        label: '近 5 次记录',
        blocks: [
          { colorClass: 'x', lines: [] },
          { colorClass: 'x', lines: ['t'], state: '正常' },
          { colorClass: 'x', lines: ['t'], state: '正常' },
          { colorClass: 'x', lines: ['t'], state: '失败' },
          { colorClass: 'x', lines: ['t'], state: '正常' },
        ],
      },
    })
    expect(wrapper.find('[role="img"]').attributes('aria-label')).toBe('近 5 次记录 (正常 3, 失败 1)')
  })

  it('触屏点一下显示明细，鼠标移开才收起', async () => {
    const wrapper = mount(MonitorStatusStrip, {
      props: { label: 'l', blocks: [{ colorClass: 'x', lines: ['09/25 17:00', '正常'], state: '正常' }] },
      // jsdom 不触发 transitionend，离场动画会一直挂着，这里只测显隐逻辑
      global: { stubs: { transition: true } },
    })
    const container = wrapper.find('.relative').element
    pointer(wrapper.find('.strip-block').element, 'pointerenter', 'touch')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('09/25 17:00')
    pointer(container, 'pointerleave', 'touch')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('09/25 17:00')
    pointer(container, 'pointerleave', 'mouse')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).not.toContain('09/25 17:00')
  })
})
