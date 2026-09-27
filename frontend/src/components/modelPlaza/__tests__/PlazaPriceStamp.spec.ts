import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlazaPrice from '../PlazaPrice.vue'
import PlazaStamp from '../PlazaStamp.vue'
import { discountText } from '../plazaBadges'

const t = (key: string, params?: Record<string, unknown>) => (params ? `${key}(${Object.values(params).join(',')})` : key)
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t }) }
})

describe('discountText', () => {
  it('同时给出中文折数与英文减免百分比，由文案各取所需', () => {
    expect(discountText(0.35, t)).toBe('modelPlaza.discount.off(3.5,65)')
    expect(discountText(0.8, t)).toBe('modelPlaza.discount.off(8,20)')
    expect(discountText(0.333, t)).toBe('modelPlaza.discount.off(3.3,67)')
  })

  it('极端比值有上下限，不出现「0折」「100% off」', () => {
    expect(discountText(0.0004, t)).toBe('modelPlaza.discount.off(0.1,99)')
    expect(discountText(0.989, t)).toBe('modelPlaza.discount.off(9.9,1)')
  })
})

describe('PlazaStamp', () => {
  it('只在真正打折时盖章，有更贵分组时写「低至」', () => {
    const floor = mount(PlazaStamp, { props: { ratio: 0.35, floor: true } })
    expect(floor.get('[data-stamp]').text()).toContain('modelPlaza.discount.upTo')
    expect(floor.get('[data-stamp]').text()).toContain('modelPlaza.discount.off(3.5,65)')

    expect(mount(PlazaStamp, { props: { ratio: 0.35 } }).text()).not.toContain('modelPlaza.discount.upTo')
    expect(mount(PlazaStamp, { props: { ratio: 0.995 } }).find('[data-stamp]').exists()).toBe(false)
    expect(mount(PlazaStamp, { props: { ratio: 1.2 } }).find('[data-stamp]').exists()).toBe(false)
    expect(mount(PlazaStamp, { props: { ratio: null } }).find('[data-stamp]').exists()).toBe(false)
  })

  it('入场标记只在 intro 时挂上', () => {
    expect(mount(PlazaStamp, { props: { ratio: 0.5, intro: true } }).get('[data-stamp]').classes()).toContain('is-intro')
    expect(mount(PlazaStamp, { props: { ratio: 0.5 } }).get('[data-stamp]').classes()).not.toContain('is-intro')
  })
})

describe('PlazaPrice', () => {
  it('原价用 <del> 划掉，并给读屏补上「原价」', () => {
    const wrapper = mount(PlazaPrice, { props: { paid: '$1.05', original: '$3.00', suffix: '/ 次' } })
    expect(wrapper.get('[data-paid-price]').text()).toBe('$1.05')
    const del = wrapper.get('del')
    expect(del.text()).toContain('$3.00')
    expect(del.get('.sr-only').text()).toBe('modelPlaza.price.original')
    expect(wrapper.text()).toContain('/ 次')
  })

  it('不打折时只显示实付价', () => {
    const wrapper = mount(PlazaPrice, { props: { paid: '$1.00' } })
    expect(wrapper.find('del').exists()).toBe(false)
  })
})
