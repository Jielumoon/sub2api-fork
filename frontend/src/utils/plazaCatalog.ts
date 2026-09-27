/**
 * 模型广场目录（纯函数）：把接口的「分组 → 模型」翻转成「模型 → 各分组报价」，
 * 并提供筛选、分面计数、排序与卡片价格口径。
 */
import type { ModelPlazaGroup, PlazaModel, PlazaOfficialPricing } from '@/api/modelPlaza'
import { billingMode, isTokenBilled, lowestUnitPrice, offerRate } from '@/utils/plazaPricing'

/** 某个分组里的这个模型：价格已按该分组合成，实付再乘该分组倍率。 */
export interface PlazaOffer {
  group: ModelPlazaGroup
  model: PlazaModel
}

export interface PlazaCatalogEntry {
  /** 同名判定与后端渠道去重一致：大小写不敏感。 */
  key: string
  /** 首次出现的原始大小写。 */
  name: string
  /** 出现过的具体平台（Composite 分组下也是具体平台），升序。 */
  platforms: string[]
  /** 官方参考价只由模型名决定，取第一个非空值。 */
  official: PlazaOfficialPricing | null
  /** 按实付倍率升序，同倍率按分组名。 */
  offers: PlazaOffer[]
}

export const FILTER_ALL = 'all' as const

export interface PlazaFilters {
  search: string
  platform: string
  groupId: number | typeof FILTER_ALL
  billing: string
}

export const EMPTY_FILTERS: PlazaFilters = {
  search: '',
  platform: FILTER_ALL,
  groupId: FILTER_ALL,
  billing: FILTER_ALL
}

export type PlazaSort = 'default' | 'name' | 'price-asc' | 'price-desc'

export type PlazaView = 'card' | 'table'

export function offerRateOf(o: PlazaOffer): number {
  return offerRate(o.model, o.group)
}

export function buildPlazaCatalog(groups: ModelPlazaGroup[]): PlazaCatalogEntry[] {
  const byKey = new Map<string, PlazaCatalogEntry>()
  for (const group of groups) {
    for (const model of group.models) {
      const key = model.name.toLowerCase()
      let entry = byKey.get(key)
      if (!entry) {
        entry = { key, name: model.name, platforms: [], official: null, offers: [] }
        byKey.set(key, entry)
      }
      entry.offers.push({ group, model })
      if (!entry.platforms.includes(model.platform)) entry.platforms.push(model.platform)
      entry.official ??= model.official_pricing
    }
  }
  const entries = [...byKey.values()]
  for (const e of entries) {
    e.platforms.sort()
    e.offers.sort((a, b) => offerRateOf(a) - offerRateOf(b) || a.group.name.localeCompare(b.group.name))
  }
  return entries
}

export function matchesOffer(o: PlazaOffer, f: PlazaFilters): boolean {
  return (
    (f.platform === FILTER_ALL || o.model.platform === f.platform) &&
    (f.groupId === FILTER_ALL || o.group.id === f.groupId) &&
    (f.billing === FILTER_ALL || billingMode(o.model) === f.billing)
  )
}

function matchesSearch(e: PlazaCatalogEntry, query: string): boolean {
  return !query || e.key.includes(query)
}

function normalizedQuery(f: PlazaFilters): string {
  return f.search.trim().toLowerCase()
}

/** 当前筛选下命中的报价（卡片 / 表格据此算「低至」与所选分组价）。 */
export function visibleOffers(e: PlazaCatalogEntry, f: PlazaFilters): PlazaOffer[] {
  return e.offers.filter((o) => matchesOffer(o, f))
}

/** 名称包含搜索词、且至少一个报价命中其余筛选的模型。 */
export function filterCatalog(entries: PlazaCatalogEntry[], f: PlazaFilters): PlazaCatalogEntry[] {
  const query = normalizedQuery(f)
  return entries.filter((e) => matchesSearch(e, query) && e.offers.some((o) => matchesOffer(o, f)))
}

export interface PlazaFacetCounts {
  platform: Map<string, number>
  group: Map<number, number>
  billing: Map<string, number>
}

/**
 * 分面计数：某选项数量 = 在「其他维度当前选择」下、换成该选项后命中的模型数。
 * 数量为 0 的选项由界面置灰；每个模型在同一选项上只计一次。
 */
export function facetCounts(entries: PlazaCatalogEntry[], f: PlazaFilters): PlazaFacetCounts {
  const query = normalizedQuery(f)
  function count<K>(relaxed: Partial<PlazaFilters>, keyOf: (o: PlazaOffer) => K): Map<K, number> {
    const counts = new Map<K, number>()
    const others = { ...f, ...relaxed }
    for (const e of entries) {
      if (!matchesSearch(e, query)) continue
      const keys = new Set(e.offers.filter((o) => matchesOffer(o, others)).map(keyOf))
      for (const k of keys) counts.set(k, (counts.get(k) ?? 0) + 1)
    }
    return counts
  }
  return {
    platform: count({ platform: FILTER_ALL }, (o) => o.model.platform),
    group: count({ groupId: FILTER_ALL }, (o) => o.group.id),
    billing: count({ billing: FILTER_ALL }, (o) => billingMode(o.model))
  }
}

/** 报价的可比实付价：token 取输入价，按次 / 按图取最低单价；已乘倍率。 */
function offerComparablePrice(o: PlazaOffer): number | null {
  const base = isTokenBilled(o.model) ? (o.model.pricing?.input_price ?? null) : lowestUnitPrice(o.model)
  return base == null ? null : base * offerRateOf(o)
}

/**
 * 最便宜的报价：有 token 报价时只在 token 报价里比（按次与 token 量纲不同不混比），
 * 同价取倍率低的；都没价时取倍率最低的（offers 已按倍率升序）。
 */
export function cheapestOffer(offers: PlazaOffer[]): PlazaOffer | null {
  if (offers.length === 0) return null
  const token = offers.filter((o) => isTokenBilled(o.model))
  const pool = token.length ? token : offers
  let best: PlazaOffer | null = null
  let bestPrice: number | null = null
  for (const o of pool) {
    const price = offerComparablePrice(o)
    if (price != null && (bestPrice == null || price < bestPrice)) {
      best = o
      bestPrice = price
    }
  }
  return best ?? pool[0]
}

/** 实付 ÷ 原价低于它才算打折（再高就显示成「9.9 折以上」这类没意义的数）。 */
export const DISCOUNT_THRESHOLD = 0.99

/** 各价格项折扣相差不超过半个展示步长（0.1 折）才算一致，才能盖一个统一的折数。 */
const RATIO_TOLERANCE = 0.005

/** 报价覆盖的分组数：Composite 分组里同名模型按平台各算一个报价，但只算一个分组。 */
export function groupCount(offers: PlazaOffer[]): number {
  return new Set(offers.map((o) => o.group.id)).size
}

export function isDiscount(ratio: number | null | undefined): ratio is number {
  return ratio != null && ratio < DISCOUNT_THRESHOLD
}

/** 卡片 / 表格 / 抽屉摘要的价格口径（单位同接口：token 为 USD/token，按次为 USD/次）。 */
export interface PlazaPriceSummary {
  /** 价格来自哪个报价：选中分组时就是该分组，否则是最便宜的报价。 */
  offer: PlazaOffer
  token: boolean
  /** 实付，已乘倍率。 */
  input: number | null
  output: number | null
  unit: number | null
  /** 划线原价：token 优先官方价，没有官方价时用未乘倍率的渠道价；按次 / 按图用渠道单价。 */
  originalInput: number | null
  originalOutput: number | null
  originalUnit: number | null
  /** 各价格项是否打折（实付明显低于它自己的原价），决定是否划掉该项原价；各项独立判断。 */
  struck: { input: boolean; output: boolean; unit: boolean }
  /**
   * 有原价的各价格项折扣一致时的统一比值（印章用，取其中较高的一个，不夸大折扣）；
   * 各项折扣不一致（如输入打折、输出加价）或都没有原价时为 null，不盖章。
   */
  ratio: number | null
  /** 其他报价更贵：折扣前加「低至」。 */
  isFloor: boolean
}

export function priceSummary(e: PlazaCatalogEntry, f: PlazaFilters): PlazaPriceSummary | null {
  const offers = visibleOffers(e, f)
  const offer = cheapestOffer(offers)
  if (!offer) return null
  const rate = offerRateOf(offer)
  const token = isTokenBilled(offer.model)
  const pricing = offer.model.pricing
  const paid = (v: number | null | undefined) => (v == null ? null : v * rate)
  const official = token && (e.official?.input_price != null || e.official?.output_price != null) ? e.official : null
  const originalInput = token ? (official ? official.input_price : (pricing?.input_price ?? null)) : null
  const originalOutput = token ? (official ? official.output_price : (pricing?.output_price ?? null)) : null
  const originalUnit = token ? null : lowestUnitPrice(offer.model)
  const input = token ? paid(pricing?.input_price) : null
  const output = token ? paid(pricing?.output_price) : null
  const unit = token ? null : paid(originalUnit)

  const itemRatio = (p: number | null, o: number | null) => (p != null && o != null && o > 0 ? p / o : null)
  const inputRatio = itemRatio(input, originalInput)
  const outputRatio = itemRatio(output, originalOutput)
  const unitRatio = itemRatio(unit, originalUnit)
  const present = [inputRatio, outputRatio, unitRatio].filter((r): r is number => r != null)
  const consistent = present.length > 0 && Math.max(...present) - Math.min(...present) <= RATIO_TOLERANCE
  const best = offerComparablePrice(offer)
  const isFloor =
    best != null &&
    offers.some((o) => o !== offer && isTokenBilled(o.model) === token && (offerComparablePrice(o) ?? 0) > best * (1 + 1e-9))

  return {
    offer,
    token,
    input,
    output,
    unit,
    originalInput,
    originalOutput,
    originalUnit,
    struck: { input: isDiscount(inputRatio), output: isDiscount(outputRatio), unit: isDiscount(unitRatio) },
    ratio: consistent ? Math.max(...present) : null,
    isFloor
  }
}

function compareNullableDesc(a: number | null | undefined, b: number | null | undefined): number {
  if (a != null && b != null) return b - a
  if (a != null) return -1
  if (b != null) return 1
  return 0
}

function hasTokenOffer(e: PlazaCatalogEntry, f: PlazaFilters): boolean {
  return visibleOffers(e, f).some((o) => isTokenBilled(o.model))
}

/**
 * - default：沿用改版前顺序——token 在前，官方输出价从高到低，无官方价排后，同价按名称降序（新版本号在前）；
 * - name：名称升序；
 * - price-*：按所选分组（或最便宜报价）的实付价；token 在前、按次在后，不混排；无价恒沉底。
 */
export function sortCatalog(entries: PlazaCatalogEntry[], sort: PlazaSort, f: PlazaFilters): PlazaCatalogEntry[] {
  const tokenFirst = (a: PlazaCatalogEntry, b: PlazaCatalogEntry) =>
    Number(hasTokenOffer(b, f)) - Number(hasTokenOffer(a, f))

  if (sort === 'name') return [...entries].sort((a, b) => a.name.localeCompare(b.name))

  if (sort === 'default') {
    return [...entries].sort(
      (a, b) =>
        tokenFirst(a, b) ||
        compareNullableDesc(a.official?.output_price, b.official?.output_price) ||
        b.name.localeCompare(a.name)
    )
  }

  const direction = sort === 'price-asc' ? 1 : -1
  const priceOf = new Map(
    entries.map((e) => {
      const best = cheapestOffer(visibleOffers(e, f))
      return [e.key, best ? offerComparablePrice(best) : null] as const
    })
  )
  return [...entries].sort((a, b) => {
    const pa = priceOf.get(a.key) ?? null
    const pb = priceOf.get(b.key) ?? null
    if ((pa == null) !== (pb == null)) return pa == null ? 1 : -1
    return (
      tokenFirst(a, b) ||
      (pa != null && pb != null ? (pa - pb) * direction : 0) ||
      a.name.localeCompare(b.name)
    )
  })
}
