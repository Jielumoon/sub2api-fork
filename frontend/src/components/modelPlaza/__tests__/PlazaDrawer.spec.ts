import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import PlazaDrawer from '../PlazaDrawer.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

let wrapper: VueWrapper
afterEach(() => wrapper?.unmount())

/** 面板内：关闭按钮 + 两个按钮 + 一个 tabindex=-1（不可 Tab 到）+ 一个禁用按钮。 */
async function openDrawer() {
  const outside = document.createElement('button')
  outside.textContent = 'outside'
  document.body.appendChild(outside)
  wrapper = mount(PlazaDrawer, {
    props: { show: true, title: 'T' },
    slots: {
      default: '<button id="a">a</button><button id="b">b</button><button tabindex="-1">skip</button><button disabled>off</button>'
    },
    attachTo: document.body,
    global: { stubs: { transition: true } }
  })
  await nextTick()
  await nextTick()
  return outside
}

const tab = (shiftKey = false) => {
  const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey, cancelable: true })
  document.dispatchEvent(event)
  return event
}
const byId = (id: string) => document.getElementById(id)!
const closeButton = () => document.querySelector<HTMLElement>('[aria-label="common.close"]')!

describe('PlazaDrawer 焦点约束', () => {
  it('打开时聚焦面板内首个可 Tab 元素', async () => {
    await openDrawer()
    expect(document.activeElement).toBe(closeButton())
  })

  it('Tab 到最后一个后回到第一个，Shift+Tab 在第一个时跳到最后一个；跳过 tabindex=-1 与禁用项', async () => {
    await openDrawer()
    byId('b').focus()
    expect(tab().defaultPrevented).toBe(true)
    expect(document.activeElement).toBe(closeButton())

    expect(tab(true).defaultPrevented).toBe(true)
    expect(document.activeElement).toBe(byId('b'))
  })

  it('中间位置的 Tab 交给浏览器默认处理', async () => {
    await openDrawer()
    byId('a').focus()
    expect(tab().defaultPrevented).toBe(false)
  })

  it('焦点跑到面板外时按 Tab 拉回面板', async () => {
    const outside = await openDrawer()
    outside.focus()
    tab()
    expect(document.activeElement).toBe(closeButton())
    outside.remove()
  })

  it('关闭后不再拦截 Tab', async () => {
    await openDrawer()
    await wrapper.setProps({ show: false })
    expect(tab().defaultPrevented).toBe(false)
  })
})
