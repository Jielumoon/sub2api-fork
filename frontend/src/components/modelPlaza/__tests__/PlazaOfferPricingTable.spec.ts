import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlazaOfferPricingTable from '../PlazaOfferPricingTable.vue'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import {
  ladderIntervals,
  ladderModel,
  plazaGroup,
  requestPricing,
  requestTier,
  timePricedModel,
  tokenModel
} from '@/utils/__tests__/plazaFixtures'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null })
}))

/** 单个报价：模型挂在给定分组上。列序：分组 / 倍率 / 输入 / 输出 / 缓存。 */
function mountOffer(model: PlazaModel, group: Partial<ModelPlazaGroup> = {}, highlightGroupId?: number) {
  const g = plazaGroup({ models: [model], ...group })
  return mount(PlazaOfferPricingTable, {
    props: { offers: [{ group: g, model }], highlightGroupId },
    global: { stubs: { GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' } } }
  })
}

const cells = (wrapper: ReturnType<typeof mountOffer>, row = 0) => wrapper.findAll('tbody tr')[row].findAll('td')

describe('PlazaOfferPricingTable token 计费', () => {
  it('倍率为 1 时展示渠道单价原值（$/1M），价格保底 2 位小数', () => {
    const c = cells(mountOffer(tokenModel()))
    expect(c[1].text()).toBe('1x')
    expect(c[2].text()).toBe('$3.00')
    expect(c[3].text()).toBe('$15.00')
    expect(c[4].text()).toContain('$3.75')
    expect(c[4].text()).toContain('$0.30')
  })

  it('倍率 ≠ 1 时为折后实付价', () => {
    const c = cells(mountOffer(tokenModel(), { rate_multiplier: 0.5 }))
    expect(c[1].text()).toBe('0.5x')
    expect(c[2].text()).toBe('$1.50')
    expect(c[3].text()).toBe('$7.50')
  })

  it('用户专属倍率覆盖分组倍率，并划线展示原倍率', () => {
    const c = cells(mountOffer(tokenModel(), { rate_multiplier: 1, user_rate_multiplier: 0.8 }))
    expect(c[2].text()).toBe('$2.40')
    expect(c[3].text()).toBe('$12.00')
    expect(c[1].find('.line-through').text()).toBe('1x')
    expect(c[1].text()).toContain('0.8x')
  })

  it('分别展示自定义 5m 与 1h 缓存写入价', () => {
    const model = tokenModel()
    model.pricing!.cache_write_1h_price = 7e-6
    const cache = cells(mountOffer(model))[4].text()
    expect(cache).toContain('$3.75')
    expect(cache).toContain('(1h')
    expect(cache).toContain('$7.00')
  })

  it('阶梯内联进输入/输出/缓存列并按倍率折算，每档一行，档位标签只在输入列', () => {
    const c = cells(mountOffer(ladderModel(), { rate_multiplier: 0.5 }))
    expect(c[2].text()).toContain('≤272K')
    expect(c[2].text()).toContain('>272K')
    expect(c[2].text()).toContain('$2.50')
    expect(c[2].text()).toContain('$5.00')
    expect(c[3].text()).not.toContain('272K')
    expect(c[3].text()).toContain('$15.00')
    expect(c[3].text()).toContain('$22.50')
    const cacheRows = c[4].findAll('.leading-5')
    expect(cacheRows).toHaveLength(2)
    expect(cacheRows[0].text()).toContain('$3.125')
    expect(cacheRows[0].text()).toContain('$0.25')
    expect(cacheRows[1].text()).toContain('$6.25')
    expect(cacheRows[1].text()).toContain('$0.50')
    expect(c[4].text()).not.toContain('272K')
  })

  it('无标签的多档按区间生成统一标签并按下限升序', () => {
    const base = ladderIntervals()
    const model = ladderModel({
      pricing: {
        ...ladderModel().pricing!,
        intervals: [
          { ...base[1], min_tokens: 1000000, tier_label: '' },
          { ...base[0], min_tokens: 100000, max_tokens: 200000, tier_label: '' },
          { ...base[0], max_tokens: 100000, tier_label: '' },
          { ...base[1], min_tokens: 200000, max_tokens: 1000000, tier_label: '' }
        ]
      }
    })
    const labels = cells(mountOffer(model))[2].findAll('.leading-5').map((r) => r.text().split(/\s+/)[0])
    expect(labels).toEqual(['≤100K', '≤200K', '≤1M', '>1M'])
  })

  it('整单计价的档位带说明；边际计价加徽章并换用边际说明', () => {
    const whole = mountOffer(ladderModel())
    expect(whole.findAll('span[title="modelPlaza.table.tierHint"]').length).toBeGreaterThan(0)
    expect(whole.text()).not.toContain('modelPlaza.table.marginalBadge')

    const marginal = mountOffer(ladderModel({ long_context_basis: 'marginal' }))
    expect(marginal.findAll('span[title="modelPlaza.table.tierHintMarginal"]').length).toBeGreaterThan(0)
    expect(cells(marginal)[0].text()).toContain('modelPlaza.table.marginalBadge')
  })

  it('思考等级倍率徽章按等级顺序，过滤无效值', () => {
    const model = tokenModel()
    model.pricing!.reasoning_effort_multipliers = { max: 3, none: 0.5, high: 1.5, low: 0 }
    const badges = mountOffer(model).findAll('[data-reasoning-effort]')
    expect(badges.map((b) => b.attributes('data-reasoning-effort'))).toEqual(['none', 'high', 'max'])
  })
})

describe('PlazaOfferPricingTable 按次 / 按图计费', () => {
  it('per_request 按单价 × 倍率，三列合并并带单位后缀', () => {
    const model = tokenModel({ pricing: requestPricing({ per_request_price: 0.04 }), official_pricing: null })
    const wrapper = mountOffer(model, { rate_multiplier: 0.5 })
    const c = cells(wrapper)
    expect(c).toHaveLength(3)
    expect(c[2].attributes('colspan')).toBe('3')
    expect(c[2].text()).toContain('$0.02')
    expect(c[2].text()).toContain('modelPlaza.table.perUnitRequest')
    expect(c[0].text()).toContain('modelPlaza.table.perRequest')
  })

  it('按图阶梯芯片按倍率折算，不把每 token 的图片输出价当按次价', () => {
    const model = tokenModel({
      pricing: requestPricing({
        billing_mode: 'image',
        image_output_price: 3e-5,
        intervals: [requestTier('1K', 0.01), requestTier('2K', 0.02)]
      }),
      official_pricing: null
    })
    const text = mountOffer(model, { rate_multiplier: 0.1 }).text()
    expect(text).toContain('modelPlaza.table.perImage')
    expect(text).toContain('$0.001')
    expect(text).toContain('$0.002')
    expect(text).toContain('modelPlaza.table.perUnitImage')
    expect(text).not.toContain('$0.000003')
  })

  it('生图独立倍率开启时按独立倍率计价，倍率列展示独立倍率', () => {
    const model = tokenModel({
      pricing: requestPricing({ billing_mode: 'image', intervals: [requestTier('1K', 0.02)] }),
      official_pricing: null
    })
    const on = cells(mountOffer(model, { rate_multiplier: 0.1, image_rate_independent: true, image_rate_multiplier: 1 }))
    expect(on[1].text()).toBe('1x')
    expect(on[2].text()).toContain('$0.02')
    expect(on[2].text()).not.toContain('$0.002')

    const off = cells(mountOffer(tokenModel({ pricing: requestPricing({ billing_mode: 'image', per_request_price: 0.2 }) }), { rate_multiplier: 0.1 }))
    expect(off[1].text()).toBe('0.1x')
    expect(off[2].text()).toContain('$0.02')
  })
})

describe('PlazaOfferPricingTable 分时计价', () => {
  it('标准行 + 每时段一行，时段行按 生效倍率 × 时段倍率 折算', () => {
    const wrapper = mountOffer(timePricedModel(), { rate_multiplier: 0.8 })
    expect(wrapper.findAll('tbody tr')).toHaveLength(3)

    const base = cells(wrapper, 0)
    expect(base[1].text()).toBe('0.8x')
    expect(base[2].text()).toBe('$2.40')

    const night = cells(wrapper, 1)
    expect(night[0].text()).toContain('00:30–08:30')
    expect(night[0].text()).not.toContain('Asia/Shanghai')
    expect(night[0].find('[title="modelPlaza.table.timePricingRowHint"]').exists()).toBe(true)
    expect(night[1].text()).toBe('0.4x')
    expect(night[2].text()).toBe('$1.20')
    expect(night[3].text()).toBe('$6.00')
    expect(night[4].text()).toContain('$1.50')

    const evening = cells(wrapper, 2)
    expect(evening[0].text()).toContain('18:00–22:00')
    expect(evening[1].text()).toBe('0.96x')
    expect(evening[2].text()).toBe('$2.88')
  })

  it('仅工作日生效时带工作日前缀并换用周末回落说明', () => {
    const model = timePricedModel()
    model.time_pricing!.weekdays_only = true
    const night = cells(mountOffer(model), 1)
    expect(night[0].text()).toContain('modelPlaza.table.timePricingWeekdays')
    expect(night[0].find('[title="modelPlaza.table.timePricingRowHintWeekdays"]').exists()).toBe(true)
  })

  it('分组启用高峰时时段行说明追加高峰披露，标准行展示高峰说明；价格不含高峰因子', () => {
    const wrapper = mountOffer(timePricedModel(), {
      rate_multiplier: 0.8,
      peak_rate_enabled: true,
      peak_start: '14:00',
      peak_end: '18:00',
      peak_rate_multiplier: 1.5
    })
    const night = cells(wrapper, 1)
    expect(night[0].find('[title*="modelPlaza.table.timePricingRowHint"]').attributes('title')).toContain(
      'modelPlaza.table.timePricingRowHintPeak'
    )
    expect(night[2].text()).toBe('$1.20')
    expect(cells(wrapper, 0)[0].text()).toContain('modelPlaza.detail.peakNote')
    // 时段行不重复高峰说明
    expect(night[0].text()).not.toContain('modelPlaza.detail.peakNote')
  })

  it('未启用高峰时不披露；无分时倍率只有一行', () => {
    const wrapper = mountOffer(timePricedModel())
    expect(wrapper.find('[title*="modelPlaza.table.timePricingRowHint"]').attributes('title')).not.toContain(
      'modelPlaza.table.timePricingRowHintPeak'
    )
    expect(mountOffer(tokenModel()).findAll('tbody tr')).toHaveLength(1)
  })
})

describe('PlazaOfferPricingTable 分组信息', () => {
  it('多个报价各占一行，按传入顺序；专属/订阅徽章、分组描述、所选分组高亮', () => {
    const cheap = plazaGroup({ id: 7, name: 'vip', rate_multiplier: 0.5, is_exclusive: true, description: '专线' })
    const sub = plazaGroup({ id: 8, name: 'monthly', subscription_type: 'subscription' })
    const wrapper = mount(PlazaOfferPricingTable, {
      props: {
        offers: [
          { group: cheap, model: tokenModel() },
          { group: sub, model: tokenModel() }
        ],
        highlightGroupId: 8
      },
      global: { stubs: { GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' } } }
    })
    const rows = wrapper.findAll('tbody tr')
    expect(rows.map((r) => r.attributes('data-group-id'))).toEqual(['7', '8'])
    expect(rows[0].text()).toContain('modelPlaza.badges.exclusive')
    expect(rows[0].text()).toContain('专线')
    expect(rows[1].text()).toContain('modelPlaza.badges.subscription')
    expect(rows[1].classes().join(' ')).toContain('bg-primary-50')
    expect(rows[0].classes().join(' ')).not.toContain('bg-primary-50')
  })

  it('Composite 分组里的报价标出具体平台', () => {
    const c = cells(mountOffer(tokenModel({ platform: 'openai' }), { platform: 'composite' }))
    expect(c[0].text()).toContain('OpenAI')
    expect(cells(mountOffer(tokenModel()))[0].text()).not.toContain('Anthropic')
  })

  it('分组关闭长上下文阶梯且模型官方带阶梯时标注；旧后端缺字段不标注', () => {
    expect(mountOffer(ladderModel(), { long_context_pricing_enabled: false }).find('[data-long-context-off]').exists()).toBe(true)
    expect(mountOffer(ladderModel(), { long_context_pricing_enabled: true }).find('[data-long-context-off]').exists()).toBe(false)
    expect(mountOffer(tokenModel(), { long_context_pricing_enabled: false }).find('[data-long-context-off]').exists()).toBe(false)

    const legacy = plazaGroup({ models: [ladderModel()] })
    delete (legacy as Partial<ModelPlazaGroup>).long_context_pricing_enabled
    const wrapper = mount(PlazaOfferPricingTable, { props: { offers: [{ group: legacy, model: ladderModel() }] } })
    expect(wrapper.find('[data-long-context-off]').exists()).toBe(false)
  })
})
