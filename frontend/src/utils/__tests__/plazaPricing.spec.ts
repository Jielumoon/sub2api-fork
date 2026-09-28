import { describe, expect, it } from 'vitest'
import {
  formatTimeWindow,
  groupRate,
  hasCustomRate,
  hasTierCachePricing,
  lowestUnitPrice,
  officialIntervals,
  perMillion,
  offerRate,
  paidPerMillion,
  paidUnitPrice,
  periodRate,
  reasoningEffortMultipliers,
  tierLabel,
  tokenIntervals,
  unitPriceTiers
} from '../plazaPricing'
import { ladderIntervals, ladderModel, plazaGroup, requestPricing, requestTier, tokenModel } from './plazaFixtures'

describe('plazaPricing 倍率', () => {
  it('生效倍率取用户专属倍率，专属与默认不同才算自定义', () => {
    expect(groupRate(plazaGroup({ rate_multiplier: 1 }))).toBe(1)
    expect(groupRate(plazaGroup({ rate_multiplier: 1, user_rate_multiplier: 0.8 }))).toBe(0.8)
    expect(hasCustomRate(plazaGroup({ rate_multiplier: 1, user_rate_multiplier: 0.8 }))).toBe(true)
    expect(hasCustomRate(plazaGroup({ rate_multiplier: 1, user_rate_multiplier: 1 }))).toBe(false)
    expect(hasCustomRate(plazaGroup({ rate_multiplier: 1 }))).toBe(false)
  })

  it('生图独立倍率只作用于图片计费模型', () => {
    const image = tokenModel({ pricing: requestPricing({ billing_mode: 'image', per_request_price: 0.2 }) })
    const group = plazaGroup({ rate_multiplier: 0.1, image_rate_independent: true, image_rate_multiplier: 1 })
    expect(offerRate(image, group)).toBe(1)
    expect(offerRate(tokenModel(), group)).toBe(0.1)
    expect(offerRate(image, { ...group, image_rate_independent: false })).toBe(0.1)
  })

  it.each([
    { enabled: true, multiplier: 1, userRate: 0.05, expected: 1 },
    { enabled: true, multiplier: 0.5, userRate: null, expected: 0.5 },
    { enabled: true, multiplier: 0, userRate: 0.05, expected: 0 },
    { enabled: true, multiplier: -1, userRate: null, expected: 0 },
    { enabled: false, multiplier: 1, userRate: 0.05, expected: 0.05 },
    { enabled: false, multiplier: 1, userRate: null, expected: 0.15 }
  ])('视频独立倍率 independent=$enabled 时倍率为 $expected，不串用生图独立倍率', (tc) => {
    const video = tokenModel({ pricing: requestPricing({ billing_mode: 'video', per_request_price: 2 }) })
    const group = plazaGroup({
      rate_multiplier: 0.15,
      user_rate_multiplier: tc.userRate,
      image_rate_independent: true,
      image_rate_multiplier: 9,
      video_rate_independent: tc.enabled,
      video_rate_multiplier: tc.multiplier
    })
    expect(offerRate(video, group)).toBe(tc.expected)
  })

  it('独立倍率为负数时按 0 计，与后端一致', () => {
    const image = tokenModel({ pricing: requestPricing({ billing_mode: 'image', per_request_price: 0.2 }) })
    expect(offerRate(image, plazaGroup({ image_rate_independent: true, image_rate_multiplier: -1 }))).toBe(0)
  })

  it('时段倍率 = 生效倍率 × 时段倍率，去掉浮点噪声', () => {
    expect(periodRate(0.8, { start_time: '00:30', end_time: '08:30', multiplier: 0.5 })).toBe(0.4)
    expect(periodRate(0.8, { start_time: '18:00', end_time: '22:00', multiplier: 1.2 })).toBe(0.96)
  })
})

describe('plazaPricing 价格格式', () => {
  it('token 实付按 $/1M 乘倍率，保底 2 位小数，更长小数原样保留', () => {
    expect(paidPerMillion(3e-6, 1)).toBe('$3.00')
    expect(paidPerMillion(1.5e-5, 0.5)).toBe('$7.50')
    expect(paidPerMillion(3e-6, 0.8)).toBe('$2.40')
    expect(paidPerMillion(6.25e-6, 0.5)).toBe('$3.125')
    expect(paidPerMillion(3e-6, periodRate(0.8, { start_time: '', end_time: '', multiplier: 1.2 }))).toBe('$2.88')
    expect(paidPerMillion(null, 1)).toBe('-')
  })

  it('按次单价乘倍率不换算 1M；官方价不乘倍率', () => {
    expect(paidUnitPrice(0.04, 0.5)).toBe('$0.02')
    expect(paidUnitPrice(0.01, 0.1)).toBe('$0.001')
    expect(perMillion(1.5e-5)).toBe('$15.00')
    expect(perMillion(undefined)).toBe('-')
  })

  it('按图最低单价取默认价与各档最小值，不把每 token 的图片输出价当按次价', () => {
    const model = tokenModel({
      pricing: requestPricing({
        billing_mode: 'image',
        image_output_price: 3e-5,
        intervals: [requestTier('1K', 0.01), requestTier('2K', 0.02)]
      })
    })
    expect(lowestUnitPrice(model)).toBe(0.01)
    expect(lowestUnitPrice(tokenModel({ pricing: requestPricing({ per_request_price: 0.04 }) }))).toBe(0.04)
    expect(lowestUnitPrice(tokenModel({ pricing: requestPricing() }))).toBeNull()
  })
})

describe('plazaPricing 按次 / 按图单价档', () => {
  it('图片三个尺寸档都有标价时默认价用不到，不参与最低价', () => {
    const covered = tokenModel({
      pricing: requestPricing({
        billing_mode: 'image',
        per_request_price: 0.01,
        intervals: [requestTier('1K', 0.1), requestTier('2K', 0.2), requestTier('4K', 0.3)]
      })
    })
    expect(unitPriceTiers(covered).map((tier) => tier.label)).toEqual(['1K', '2K', '4K'])
    expect(lowestUnitPrice(covered)).toBe(0.1)
  })

  it('尺寸档没配全时默认价仍会兜底，作为无标签的一档保留', () => {
    const partial = tokenModel({
      pricing: requestPricing({ billing_mode: 'image', per_request_price: 0.01, intervals: [requestTier('1K', 0.1)] })
    })
    expect(unitPriceTiers(partial)).toEqual([{ label: '1K', price: 0.1 }, { label: null, price: 0.01 }])
    expect(lowestUnitPrice(partial)).toBe(0.01)
  })

  it('按次模式下 0 token 请求落不进任何区间，默认价始终保留', () => {
    const perRequest = tokenModel({
      pricing: requestPricing({ per_request_price: 0.02, intervals: [requestTier('1K', 0.1), requestTier('2K', 0.2), requestTier('4K', 0.3)] })
    })
    expect(unitPriceTiers(perRequest).at(-1)).toEqual({ label: null, price: 0.02 })
  })
})

describe('plazaPricing 阶梯', () => {
  it('无标签的多档按区间生成 ≤上限 / >下限，并按下限升序', () => {
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
    expect(tokenIntervals(model).map(tierLabel)).toEqual(['≤100K', '≤200K', '≤1M', '>1M'])
    expect(tierLabel({ ...base[0], tier_label: '自定义' })).toBe('自定义')
  })

  it('档位只给倍率时按基础价解析 input / output / cache', () => {
    const model = tokenModel({
      pricing: {
        billing_mode: 'token',
        input_price: 10e-6,
        output_price: 50e-6,
        cache_write_price: 12.5e-6,
        cache_write_1h_price: 12.5e-6,
        cache_read_price: 2e-6,
        image_input_price: null,
        image_output_price: null,
        per_request_price: null,
        intervals: [{
          min_tokens: 272000,
          max_tokens: null,
          tier_label: '>272K',
          input_price: null,
          output_price: null,
          cache_write_price: null,
          cache_write_1h_price: null,
          cache_read_price: null,
          input_multiplier: 2,
          output_multiplier: 1.5,
          cache_write_multiplier: 2,
          cache_read_multiplier: 2,
          per_request_price: null
        }]
      }
    })
    const [tier] = tokenIntervals(model)
    expect(paidPerMillion(tier.input_price, 1)).toBe('$20.00')
    expect(paidPerMillion(tier.output_price, 1)).toBe('$75.00')
    expect(paidPerMillion(tier.cache_write_price, 1)).toBe('$25.00')
    expect(paidPerMillion(tier.cache_read_price, 1)).toBe('$4.00')
    expect(hasTierCachePricing(tokenIntervals(model))).toBe(true)
  })

  it('官方阶梯原样返回；旧响应无 intervals 时为空', () => {
    expect(officialIntervals(ladderModel()).map((iv) => perMillion(iv.input_price))).toEqual(['$5.00', '$10.00'])
    expect(officialIntervals(tokenModel())).toEqual([])
  })
})

describe('plazaPricing 其他', () => {
  it('思考等级倍率按等级顺序，过滤无效与未知等级', () => {
    const model = tokenModel()
    model.pricing!.reasoning_effort_multipliers = { max: 3, none: 0.5, high: 1.5 }
    expect(reasoningEffortMultipliers(model).map(([effort]) => effort)).toEqual(['none', 'high', 'max'])

    model.pricing!.reasoning_effort_multipliers = { max: 0, high: Infinity, unknown: 2, low: 1 }
    expect(reasoningEffortMultipliers(model)).toEqual([['low', 1]])
    expect(reasoningEffortMultipliers(tokenModel({ name: 'claude-fable-5-1' }))).toEqual([])
  })

  it('时段窗口省略整分钟的秒', () => {
    expect(formatTimeWindow({ start_time: '00:30', end_time: '08:30:00', multiplier: 0.5 })).toBe('00:30–08:30')
    expect(formatTimeWindow({ start_time: '18:00:30', end_time: '22:00', multiplier: 1 })).toBe('18:00:30–22:00')
  })
})
