import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaToolbar from '../PlazaToolbar.vue'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { plazaGroup, requestPricing, requestTier, tokenModel } from '@/utils/__tests__/plazaFixtures'

// 带上插值参数，才能断言「3折」「+2 个分组」这类拼进文案的值
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}(${Object.values(params).join(',')})` : key)
    })
  }
})
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: { api_base_url: 'https://api.example.com/v1/' } })
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: false }) }))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: ref(false), copyToClipboard: vi.fn() })
}))

/** claude-sonnet 同时在 default(1x)、ag(0.5x, Antigravity)、vip(专属倍率 0.3x) 三个分组；另有 gemini-pro 与按次的 search。 */
function response(): ModelPlazaResponse {
  return {
    description: '**价格说明**',
    groups: [
      plazaGroup({ id: 1, name: 'default', rate_multiplier: 1, models: [tokenModel()] }),
      plazaGroup({
        id: 2,
        name: 'ag',
        platform: 'antigravity',
        rate_multiplier: 0.5,
        peak_rate_enabled: true,
        peak_start: '14:00',
        peak_end: '18:00',
        peak_rate_multiplier: 1.5,
        models: [
          tokenModel({ platform: 'antigravity' }),
          tokenModel({ name: 'gemini-pro', platform: 'antigravity', official_pricing: null })
        ]
      }),
      plazaGroup({
        id: 3,
        name: 'vip',
        rate_multiplier: 1,
        user_rate_multiplier: 0.3,
        models: [tokenModel(), tokenModel({ name: 'search', pricing: requestPricing({ per_request_price: 0.04 }), official_pricing: null })]
      }),
      plazaGroup({
        id: 4,
        name: 'img',
        platform: 'openai',
        models: [
          tokenModel({
            name: 'gpt-image-2',
            platform: 'openai',
            official_pricing: null,
            pricing: requestPricing({ billing_mode: 'image', intervals: [requestTier('1K', 0.04), requestTier('2K', 0.08)] })
          })
        ]
      })
    ]
  }
}

// 项目的 setup.ts 清空了 VTU 默认的过渡桩；这里要断言过渡后的最终 DOM，显式装回
const STUBS = { teleport: true, transition: true, 'transition-group': true }

let wrapper: VueWrapper
afterEach(() => wrapper?.unmount())

async function mountContent(res: ModelPlazaResponse | null = response()) {
  wrapper = mount(ModelPlazaContent, {
    props: { response: res, loading: false },
    attachTo: document.body,
    global: { stubs: STUBS }
  })
  await flushPromises()
  return wrapper
}

const cardNames = () => wrapper.findAll('[data-plaza-cards] [data-open-model]').map((b) => b.text())
const card = (name: string) => wrapper.findAll('[data-plaza-cards] article').find((a) => a.find('[data-open-model]').text() === name)!
const chip = (value: string) => wrapper.findAll(`[data-filter="${value}"]`)[0]
const paidPrices = (el: ReturnType<typeof card>) => el.findAll('[data-paid-price]').map((p) => p.text())
const originalPrices = (el: ReturnType<typeof card>) => el.findAll('[data-original-price]').map((p) => p.text().replace('modelPlaza.price.original', ''))

describe('ModelPlazaContent 列表', () => {
  it('同名模型跨分组、跨平台只出一张卡片；页头只有标题与搜索', async () => {
    await mountContent()
    expect(cardNames()).toEqual(['claude-sonnet', 'gemini-pro', 'search', 'gpt-image-2'])
    expect(wrapper.get('h1').text()).toBe('modelPlaza.title')
    expect(wrapper.html()).toContain('<strong>价格说明</strong>')
    expect(wrapper.get('[data-result-count]').text()).toContain('4')
    // 卡片不逐张写单位，这是唯一的计价基准说明：任何宽度都不能藏起来
    const unit = wrapper.get('[data-price-unit]')
    expect(unit.text()).toBe('modelPlaza.price.perMillion')
    expect(unit.classes()).not.toContain('hidden')
  })

  it('大字是最便宜分组的实付价，旁边划掉官方价，印章写「低至」和折数', async () => {
    await mountContent()
    const sonnet = card('claude-sonnet')
    expect(paidPrices(sonnet)).toEqual(['$0.90', '$4.50'])
    expect(originalPrices(sonnet)).toEqual(['$3.00', '$15.00'])
    const stamp = sonnet.get('[data-stamp]').text()
    expect(stamp).toContain('modelPlaza.discount.upTo')
    expect(stamp).toContain('modelPlaza.discount.off(3,70)')
    // 价格出自哪个分组，其余分组数
    expect(sonnet.get('[data-best-group]').text()).toBe('vip')
    expect(sonnet.text()).toContain('modelPlaza.card.moreGroups(2)')
  })

  it('选中分组后价格、原价和折数都换成该分组的，不再写「低至」', async () => {
    await mountContent()
    await chip('group:2').trigger('click')
    const sonnet = card('claude-sonnet')
    expect(paidPrices(sonnet)).toEqual(['$1.50', '$7.50'])
    expect(originalPrices(sonnet)).toEqual(['$3.00', '$15.00'])
    expect(sonnet.get('[data-stamp]').text()).toBe('modelPlaza.discount.off(5,50)')
    expect(sonnet.get('[data-best-group]').text()).toBe('ag')
    expect(sonnet.text()).not.toContain('modelPlaza.card.moreGroups')
  })

  it('没有官方价时划掉渠道原价；按次按单价；没打折就不划线、不盖章', async () => {
    await mountContent()
    expect(paidPrices(card('gemini-pro'))[0]).toBe('$1.50')
    expect(originalPrices(card('gemini-pro'))[0]).toBe('$3.00')

    const search = card('search')
    expect(paidPrices(search)).toEqual(['$0.012'])
    expect(originalPrices(search)).toEqual(['$0.04'])
    expect(search.text()).toContain('modelPlaza.table.perUnitRequest')
    expect(search.text()).toContain('modelPlaza.table.perRequest')

    const image = card('gpt-image-2')
    expect(paidPrices(image)).toEqual(['$0.04'])
    expect(image.find('[data-original-price]').exists()).toBe(false)
    expect(image.find('[data-stamp]').exists()).toBe(false)
    // 图片分辨率档位不是上下文阶梯
    expect(image.text()).toContain('modelPlaza.table.perImage')
    expect(image.text()).not.toContain('modelPlaza.badges.tiered')
  })
})

describe('ModelPlazaContent 筛选与排序', () => {
  it('搜索与平台/分组/计费筛选可组合，零结果选项置灰，重置保留搜索词', async () => {
    await mountContent()
    await chip('platform:antigravity').trigger('click')
    expect(cardNames()).toEqual(['claude-sonnet', 'gemini-pro'])
    // Antigravity 下没有按次模型，也没有 default / vip 分组
    expect(chip('billing:per_request').attributes('disabled')).toBeDefined()
    expect(chip('group:1').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-result-count]').text()).toContain('2 / 4')

    await wrapper.get('[data-plaza-search]').setValue('gemini')
    expect(cardNames()).toEqual(['gemini-pro'])

    await wrapper.findAll('[data-reset-filters]')[0].trigger('click')
    expect(cardNames()).toEqual(['gemini-pro'])
    expect((wrapper.get('[data-plaza-search]').element as HTMLInputElement).value).toBe('gemini')
  })

  it('无结果时给出清空全部条件的出口', async () => {
    await mountContent()
    await wrapper.get('[data-plaza-search]').setValue('nope')
    expect(wrapper.text()).toContain('modelPlaza.noSearchResult')
    await wrapper.findAll('button').find((b) => b.text() === 'modelPlaza.filters.clearAll')!.trigger('click')
    expect(cardNames()).toHaveLength(4)
  })

  it('排序切换改变卡片顺序', async () => {
    await mountContent()
    wrapper.findComponent(PlazaToolbar).vm.$emit('update:sort', 'name')
    await flushPromises()
    expect(cardNames()).toEqual(['claude-sonnet', 'gemini-pro', 'gpt-image-2', 'search'])
    wrapper.findComponent(PlazaToolbar).vm.$emit('update:sort', 'price-desc')
    await flushPromises()
    // claude 最便宜 0.9 < gemini 1.5；按次/按图沉到 token 之后（0.04 > 0.012）
    expect(cardNames()).toEqual(['gemini-pro', 'claude-sonnet', 'gpt-image-2', 'search'])
  })

  it('表格视图一行一个模型，价格格同样划掉原价、标出折扣与最低价分组', async () => {
    await mountContent()
    await wrapper.get('[data-view="table"]').trigger('click')
    expect(wrapper.find('[data-plaza-cards]').exists()).toBe(false)
    const rows = wrapper.findAll('[data-model-row]')
    expect(rows).toHaveLength(4)
    expect(rows[0].get('[data-input]').text()).toContain('$0.90')
    expect(rows[0].get('[data-input] [data-original-price]').text()).toContain('$3.00')
    expect(rows[0].get('[data-discount]').text()).toContain('modelPlaza.discount.upTo')
    expect(rows[0].get('[data-discount]').text()).toContain('modelPlaza.discount.off(3,70)')
    expect(rows[0].text()).toContain('vip')
    // 没打折的行折扣列为 -
    expect(rows[3].get('[data-discount]').text()).toBe('-')
  })
})

describe('ModelPlazaContent 入场动效', () => {
  it('首次加载只编排一次：结束后撤掉入场标记，之后筛选出来的卡片不再重播', async () => {
    vi.useFakeTimers()
    try {
      wrapper = mount(ModelPlazaContent, {
        props: { response: response(), loading: false },
        global: { stubs: STUBS }
      })
      await nextTick()
      expect(wrapper.findAll('[data-plaza-cards] .is-intro').length).toBeGreaterThan(0)

      vi.advanceTimersByTime(2000)
      await nextTick()
      expect(wrapper.find('.is-intro').exists()).toBe(false)

      await chip('group:2').trigger('click')
      await chip('group:2').trigger('click')
      expect(wrapper.find('.is-intro').exists()).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })

  it('动画未结束就卸载时清掉定时器', async () => {
    vi.useFakeTimers()
    try {
      wrapper = mount(ModelPlazaContent, {
        props: { response: response(), loading: false },
        global: { stubs: STUBS }
      })
      await nextTick()
      expect(vi.getTimerCount()).toBeGreaterThan(0)
      wrapper.unmount()
      expect(vi.getTimerCount()).toBe(0)
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('ModelPlazaContent 详情抽屉', () => {
  it('点卡片打开抽屉：顶部最低价与印章，分组表标出最低并高亮所选分组；Esc 关闭后焦点回到卡片', async () => {
    await mountContent()
    await chip('group:1').trigger('click')
    const open = card('claude-sonnet').get('[data-open-model]')
    ;(open.element as HTMLElement).focus()
    await open.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[role="dialog"]')
    // 抽屉讲模型本身：最低价在全部分组里找，不受列表筛选影响
    const summary = dialog.get('[data-drawer-summary]')
    expect(summary.text()).toContain('modelPlaza.drawer.lowestIn(vip)')
    expect(summary.text()).toContain('$0.90')
    expect(summary.get('[data-stamp]').text()).toContain('modelPlaza.discount.off(3,70)')

    const rows = dialog.findAll('tbody tr[data-group-id]')
    expect(rows.map((r) => r.attributes('data-group-id'))).toEqual(['3', '2', '1'])
    expect(rows[0].find('[data-lowest]').exists()).toBe(true)
    expect(rows[1].find('[data-lowest]').exists()).toBe(false)
    expect(rows[2].classes().join(' ')).toContain('bg-primary-50')
    expect(dialog.get('[data-example-code]').text()).toContain('https://api.example.com/v1/messages')
    // 高峰说明的倍率只出现一次
    expect(dialog.text()).toContain('modelPlaza.detail.peakNote(14:00-18:00,1.5)')
    expect(dialog.text()).not.toContain('×1.5')
    expect(document.activeElement).not.toBe(open.element)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(open.element)
    expect(document.body.classList.contains('modal-open')).toBe(false)
  })

  it('窄屏筛选按钮打开筛选抽屉', async () => {
    await mountContent()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    await wrapper.get('[data-open-filters]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain('modelPlaza.filters.platform')
    expect(document.body.classList.contains('modal-open')).toBe(true)
  })

  it('没有模型时显示空态', async () => {
    await mountContent({ description: '', groups: [] })
    expect(wrapper.text()).toContain('modelPlaza.empty')
  })
})
