import { describe, expect, it } from 'vitest'
import {
  EMPTY_FILTERS,
  buildPlazaCatalog,
  cheapestOffer,
  facetCounts,
  filterCatalog,
  groupCount,
  isDiscount,
  priceSummary,
  sortCatalog,
  visibleOffers,
  type PlazaFilters
} from '../plazaCatalog'
import { plazaGroup, requestPricing, requestTier, tokenModel } from './plazaFixtures'

const filters = (overrides: Partial<PlazaFilters> = {}): PlazaFilters => ({ ...EMPTY_FILTERS, ...overrides })

/** 同一个 claude-sonnet：Anthropic 默认组 1x、Antigravity 组 0.5x（大小写不同）、专属组默认 1x 但用户专属 0.3x。 */
function sampleGroups() {
  return [
    plazaGroup({ id: 1, name: 'default', rate_multiplier: 1, models: [tokenModel()] }),
    plazaGroup({
      id: 2,
      name: 'ag',
      platform: 'antigravity',
      rate_multiplier: 0.5,
      models: [
        tokenModel({ name: 'Claude-Sonnet', platform: 'antigravity' }),
        tokenModel({ name: 'gemini-pro', platform: 'antigravity', official_pricing: null })
      ]
    }),
    plazaGroup({
      id: 3,
      name: 'vip',
      is_exclusive: true,
      rate_multiplier: 1,
      user_rate_multiplier: 0.3,
      models: [
        tokenModel(),
        tokenModel({ name: 'search', pricing: requestPricing({ per_request_price: 0.04 }), official_pricing: null })
      ]
    })
  ]
}

describe('buildPlazaCatalog', () => {
  it('同名模型跨分组、跨平台、大小写不同都合并成一条，报价按实付倍率升序', () => {
    const catalog = buildPlazaCatalog(sampleGroups())
    expect(catalog.map((e) => e.key)).toEqual(['claude-sonnet', 'gemini-pro', 'search'])

    const sonnet = catalog[0]
    expect(sonnet.name).toBe('claude-sonnet')
    expect(sonnet.platforms).toEqual(['anthropic', 'antigravity'])
    expect(sonnet.offers.map((o) => o.group.name)).toEqual(['vip', 'ag', 'default'])
    expect(sonnet.official?.input_price).toBe(3e-6)
  })

  it('Composite 分组里同名不同平台的模型各自成为一个报价', () => {
    const catalog = buildPlazaCatalog([
      plazaGroup({
        platform: 'composite',
        models: [tokenModel({ name: 'shared' }), tokenModel({ name: 'shared', platform: 'openai' })]
      })
    ])
    expect(catalog).toHaveLength(1)
    expect(catalog[0].offers.map((o) => o.model.platform)).toEqual(['anthropic', 'openai'])
  })
})

describe('筛选与分面', () => {
  const catalog = buildPlazaCatalog(sampleGroups())

  it('搜索、平台、分组、计费类型组合过滤；报价只保留命中的', () => {
    expect(filterCatalog(catalog, filters({ search: ' SONNET ' })).map((e) => e.key)).toEqual(['claude-sonnet'])
    expect(filterCatalog(catalog, filters({ platform: 'antigravity' })).map((e) => e.key)).toEqual(['claude-sonnet', 'gemini-pro'])
    expect(filterCatalog(catalog, filters({ billing: 'per_request' })).map((e) => e.key)).toEqual(['search'])
    expect(filterCatalog(catalog, filters({ groupId: 2, billing: 'per_request' }))).toEqual([])
    expect(visibleOffers(catalog[0], filters({ platform: 'anthropic' })).map((o) => o.group.id)).toEqual([3, 1])
  })

  it('分面计数按「其他维度当前选择」计算，每个模型在同一选项只计一次', () => {
    const all = facetCounts(catalog, filters())
    expect(Object.fromEntries(all.platform)).toEqual({ anthropic: 2, antigravity: 2 })
    expect(Object.fromEntries(all.group)).toEqual({ 1: 1, 2: 2, 3: 2 })
    expect(Object.fromEntries(all.billing)).toEqual({ token: 2, per_request: 1 })

    // 选中 Antigravity 平台后：平台维度自身不受影响，分组只剩 ag，计费只剩 token
    const ag = facetCounts(catalog, filters({ platform: 'antigravity' }))
    expect(Object.fromEntries(ag.platform)).toEqual({ anthropic: 2, antigravity: 2 })
    expect(Object.fromEntries(ag.group)).toEqual({ 2: 2 })
    expect(Object.fromEntries(ag.billing)).toEqual({ token: 2 })

    // 搜索词对所有维度生效
    expect(Object.fromEntries(facetCounts(catalog, filters({ search: 'gemini' })).group)).toEqual({ 2: 1 })
  })
})

describe('价格口径', () => {
  const catalog = buildPlazaCatalog(sampleGroups())
  const [sonnet, gemini, search] = catalog

  it('未选分组：最便宜报价的实付价，划线原价用官方价，折扣按两者之比', () => {
    const s = priceSummary(sonnet, filters())!
    expect(s.offer.group.name).toBe('vip')
    expect(s).toMatchObject({ token: true, originalInput: 3e-6, originalOutput: 1.5e-5, isFloor: true })
    expect(s.input).toBeCloseTo(9e-7, 12)
    expect(s.output).toBeCloseTo(4.5e-6, 12)
    expect(s.ratio).toBeCloseTo(0.3)
    // 平台筛选后只剩一个报价：不再是「低至」
    const ag = priceSummary(sonnet, filters({ platform: 'antigravity' }))!
    expect(ag).toMatchObject({ isFloor: false })
    expect(ag.ratio).toBeCloseTo(0.5)
  })

  it('渠道价与官方价不同时划掉的是官方价，折数按实付 ÷ 官方价算', () => {
    const [entry] = buildPlazaCatalog([
      plazaGroup({ rate_multiplier: 0.5, models: [tokenModel({ pricing: { ...tokenModel().pricing!, input_price: 2e-6, output_price: 1e-5 } })] })
    ])
    const s = priceSummary(entry, filters())!
    expect(s).toMatchObject({ originalInput: 3e-6, originalOutput: 1.5e-5 })
    expect(s.input).toBeCloseTo(1e-6, 12)
    expect(s.ratio).toBeCloseTo(1 / 3)
  })

  it('无官方价时用未乘倍率的渠道价当原价；按次模型按单价', () => {
    const g = priceSummary(gemini, filters())!
    expect(g).toMatchObject({ originalInput: 3e-6, isFloor: false })
    expect(g.ratio).toBeCloseTo(0.5)

    const s = priceSummary(search, filters())!
    expect(s).toMatchObject({ token: false, input: null, originalUnit: 0.04 })
    expect(s.unit).toBeCloseTo(0.012)
    expect(s.ratio).toBeCloseTo(0.3)
  })

  it('选中分组：显示该分组价格；没打折时比值为 1', () => {
    const s = priceSummary(sonnet, filters({ groupId: 1 }))!
    expect(s.offer.group.name).toBe('default')
    expect(s).toMatchObject({ input: 3e-6, output: 1.5e-5, ratio: 1, isFloor: false })
    expect(isDiscount(s.ratio)).toBe(false)
  })

  it('各价格项独立判断是否打折：输入打折、输出加价时只划输入，不盖统一折数', () => {
    const [entry] = buildPlazaCatalog([
      plazaGroup({ models: [tokenModel({ pricing: { ...tokenModel().pricing!, input_price: 1.5e-6, output_price: 3e-5 } })] })
    ])
    const s = priceSummary(entry, filters())!
    expect(s.struck).toEqual({ input: true, output: false, unit: false })
    expect(s.ratio).toBeNull()
  })

  it('输入、输出折扣都在但不一致时同样不盖章；一致时盖较高的那个', () => {
    const [mixed] = buildPlazaCatalog([
      plazaGroup({ models: [tokenModel({ pricing: { ...tokenModel().pricing!, input_price: 1.5e-6, output_price: 9e-6 } })] })
    ])
    expect(priceSummary(mixed, filters())).toMatchObject({ struck: { input: true, output: true }, ratio: null })

    const [even] = buildPlazaCatalog([plazaGroup({ rate_multiplier: 0.5, models: [tokenModel()] })])
    expect(priceSummary(even, filters())!.ratio).toBeCloseTo(0.5)
  })

  it('默认价被尺寸档完整覆盖的图片报价不能凭默认价压过别的分组', () => {
    const [entry] = buildPlazaCatalog([
      plazaGroup({
        id: 1,
        name: 'tiers',
        models: [tokenModel({ name: 'img', pricing: requestPricing({
          billing_mode: 'image',
          per_request_price: 0.01,
          intervals: [requestTier('1K', 0.1), requestTier('2K', 0.2), requestTier('4K', 0.3)]
        }) })]
      }),
      plazaGroup({ id: 2, name: 'flat', models: [tokenModel({ name: 'img', pricing: requestPricing({ billing_mode: 'image', per_request_price: 0.05 }) })] })
    ])
    const s = priceSummary(entry, filters())!
    expect(s.offer.group.name).toBe('flat')
    expect(s.unit).toBeCloseTo(0.05)
  })

  it('分组数按分组去重：Composite 分组里同名模型的两个平台报价只算一个分组', () => {
    const [entry] = buildPlazaCatalog([
      plazaGroup({ id: 9, platform: 'composite', models: [tokenModel({ name: 'shared' }), tokenModel({ name: 'shared', platform: 'openai' })] })
    ])
    expect(entry.offers).toHaveLength(2)
    expect(groupCount(entry.offers)).toBe(1)
  })

  it('最便宜报价优先在 token 报价里比，按次与 token 不混比', () => {
    const mixed = buildPlazaCatalog([
      plazaGroup({ id: 1, rate_multiplier: 1, models: [tokenModel({ name: 'm', pricing: requestPricing({ per_request_price: 1e-9 }) })] }),
      plazaGroup({ id: 2, rate_multiplier: 2, models: [tokenModel({ name: 'm' })] })
    ])[0]
    expect(cheapestOffer(mixed.offers)?.group.id).toBe(2)
    expect(cheapestOffer([])).toBeNull()
  })
})

describe('sortCatalog', () => {
  function official(output: number | null) {
    return output == null
      ? null
      : { input_price: 1e-6, output_price: output, cache_write_price: null, cache_write_1h_price: null, cache_read_price: null }
  }

  it('默认：token 在前 → 官方输出价降序（无官方价在后）→ 名称降序', () => {
    const catalog = buildPlazaCatalog([
      plazaGroup({
        models: [
          tokenModel({ name: 'model-cheap', official_pricing: official(5e-6) }),
          tokenModel({ name: 'gpt-image-2', pricing: requestPricing({ billing_mode: 'image', per_request_price: 0.002 }), official_pricing: official(1e-5) }),
          tokenModel({ name: 'model-no-official', official_pricing: null }),
          tokenModel({ name: 'model-expensive', official_pricing: official(7.5e-5) }),
          tokenModel({ name: 'gpt-5.5', official_pricing: official(6e-6) }),
          tokenModel({ name: 'gpt-5.6-sol', official_pricing: official(6e-6) })
        ]
      })
    ])
    expect(sortCatalog(catalog, 'default', filters()).map((e) => e.name)).toEqual([
      'model-expensive',
      'gpt-5.6-sol',
      'gpt-5.5',
      'model-cheap',
      'model-no-official',
      'gpt-image-2'
    ])
  })

  it('价格排序按实付价：token 在前、按次在后，无价恒沉底', () => {
    const catalog = buildPlazaCatalog([
      plazaGroup({
        id: 1,
        rate_multiplier: 1,
        models: [
          tokenModel({ name: 'a-cheap', pricing: { ...tokenModel().pricing!, input_price: 1e-6 } }),
          tokenModel({ name: 'b-pricy', pricing: { ...tokenModel().pricing!, input_price: 9e-6 } }),
          tokenModel({ name: 'c-request', pricing: requestPricing({ per_request_price: 0.01 }) }),
          tokenModel({ name: 'd-free', pricing: { ...tokenModel().pricing!, input_price: null } }),
          tokenModel({ name: 'e-image', pricing: requestPricing({ billing_mode: 'image', intervals: [requestTier('1K', 0.001)] }) })
        ]
      }),
      // 另一个分组里 b 打 0.1 折，最便宜的实付变成 0.9
      plazaGroup({ id: 2, rate_multiplier: 0.1, models: [tokenModel({ name: 'b-pricy', pricing: { ...tokenModel().pricing!, input_price: 9e-6 } })] })
    ])
    expect(sortCatalog(catalog, 'price-asc', filters()).map((e) => e.name)).toEqual(['b-pricy', 'a-cheap', 'e-image', 'c-request', 'd-free'])
    expect(sortCatalog(catalog, 'price-desc', filters()).map((e) => e.name)).toEqual(['a-cheap', 'b-pricy', 'c-request', 'e-image', 'd-free'])
    // 选中分组后按该分组价
    expect(sortCatalog(filterCatalog(catalog, filters({ groupId: 1 })), 'price-asc', filters({ groupId: 1 })).map((e) => e.name))
      .toEqual(['a-cheap', 'b-pricy', 'e-image', 'c-request', 'd-free'])
  })

  it('名称升序', () => {
    const catalog = buildPlazaCatalog([plazaGroup({ models: [tokenModel({ name: 'b' }), tokenModel({ name: 'a' })] })])
    expect(sortCatalog(catalog, 'name', filters()).map((e) => e.name)).toEqual(['a', 'b'])
  })
})
