/**
 * 模型广场计价口径（纯函数）：卡片、表格、详情抽屉共用，保证各处数字一致。
 * 价格单位：token 计费为 USD / token，按次 / 按图为 USD / 次；展示时 token 价换算为 $ / 1M。
 */
import { formatScaled, resolveIntervalPrices } from '@/utils/pricing'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_TOKEN,
  REASONING_EFFORT_LEVELS,
  type BillingMode
} from '@/constants/channel'
import type { ModelPlazaGroup, PlazaModel, PlazaTimePricingPeriod } from '@/api/modelPlaza'
import type { UserPricingInterval } from '@/api/channels'

export const PER_MILLION = 1_000_000

/** 价格统一保底 2 位小数，更长的有效小数原样保留。 */
const MIN_DECIMALS = 2

type RateFields = Pick<ModelPlazaGroup, 'rate_multiplier' | 'user_rate_multiplier'>
type ImageRateFields = RateFields & Pick<ModelPlazaGroup, 'image_rate_independent' | 'image_rate_multiplier'>

export function billingMode(m: PlazaModel): BillingMode {
  return (m.pricing?.billing_mode || BILLING_MODE_TOKEN) as BillingMode
}

export function isTokenBilled(m: PlazaModel): boolean {
  return billingMode(m) === BILLING_MODE_TOKEN
}

/** 分组生效倍率 = 用户专属倍率 ?? 分组默认倍率。 */
export function groupRate(g: RateFields): number {
  return g.user_rate_multiplier ?? g.rate_multiplier
}

/** 用户专属倍率与分组默认倍率不同（展示时划线原倍率）。 */
export function hasCustomRate(g: RateFields): boolean {
  return g.user_rate_multiplier != null && g.user_rate_multiplier !== g.rate_multiplier
}

/** 图片计费模型且分组开启生图独立倍率：实付倍率取独立倍率，与计费口径一致。 */
export function usesIndependentImageRate(m: PlazaModel, g: ImageRateFields): boolean {
  return billingMode(m) === BILLING_MODE_IMAGE && g.image_rate_independent === true
}

/** 该模型在该分组的实付倍率（token 与按次都用它乘单价）。 */
export function offerRate(m: PlazaModel, g: ImageRateFields): number {
  return usesIndependentImageRate(m, g) ? (g.image_rate_multiplier ?? 1) : groupRate(g)
}

/** 分时时段的生效倍率 = 生效倍率 × 时段倍率（去掉浮点噪声）。 */
export function periodRate(rate: number, period: PlazaTimePricingPeriod): number {
  return Math.round(rate * period.multiplier * 1000) / 1000
}

/** token 实付价：单价 × 倍率，按 $/1M 展示。 */
export function paidPerMillion(value: number | null | undefined, rate: number): string {
  if (value == null) return '-'
  return formatScaled(value * rate, PER_MILLION, MIN_DECIMALS)
}

/** 按次 / 按图片实付单价（乘倍率，不换算 1M）。 */
export function paidUnitPrice(value: number | null | undefined, rate: number): string {
  if (value == null) return '-'
  return formatScaled(value * rate, 1, MIN_DECIMALS)
}

/** 已乘好倍率的实付价或官方原价（均为 USD / token），按 $/1M 展示，不再乘倍率。 */
export function perMillion(value: number | null | undefined): string {
  if (value == null) return '-'
  return formatScaled(value, PER_MILLION, MIN_DECIMALS)
}

type CachePrices = Pick<UserPricingInterval, 'cache_write_price' | 'cache_write_1h_price' | 'cache_read_price'>

export function hasCachePricing(p: CachePrices | null | undefined): boolean {
  return p?.cache_write_price != null || p?.cache_write_1h_price != null || p?.cache_read_price != null
}

/** 任一档带缓存价才按档渲染缓存列；否则沿用平价的写入/读取两行。 */
export function hasTierCachePricing(intervals: UserPricingInterval[]): boolean {
  return intervals.some((iv) =>
    hasCachePricing(iv) || iv.cache_write_multiplier != null || iv.cache_read_multiplier != null
  )
}

/** 上下文档位按下限升序展示（后端已升序，此处兜底）。 */
function sortByContext(intervals: UserPricingInterval[]): UserPricingInterval[] {
  return [...intervals].sort((a, b) => a.min_tokens - b.min_tokens)
}

/** token 模式的阶梯定价（档位只给倍率时按基础价解析）。 */
export function tokenIntervals(m: PlazaModel): UserPricingInterval[] {
  const pricing = m.pricing
  if (!pricing) return []
  return sortByContext(pricing.intervals ?? []).map((iv) => resolveIntervalPrices(iv, pricing))
}

/** 官方阶梯（后端按目录规则合成，不受分组开关影响）。 */
export function officialIntervals(m: Pick<PlazaModel, 'official_pricing'>): UserPricingInterval[] {
  return sortByContext(m.official_pricing?.intervals ?? [])
}

/** 按次 / 按图模式的阶梯定价（仅保留配了按次价的档位）。 */
export function requestIntervals(m: PlazaModel): UserPricingInterval[] {
  return (m.pricing?.intervals ?? []).filter((iv) => iv.per_request_price != null)
}

/** 图片计费的尺寸档（后端 ImageBillingSize1K/2K/4K），每次请求都会落到其中一档。 */
const IMAGE_SIZE_TIERS = ['1K', '2K', '4K']

/**
 * 默认按次价会不会被用到：计费先按尺寸标签取档位价，其次按上下文区间，最后才回落默认价
 * （backend service/billing_service.go calculatePerRequestCost）。图片三个尺寸档都有标价时默认价用不到；
 * 按次模式下 0 token 的请求匹配不到任何区间（区间左开），默认价始终可能生效。
 */
function defaultUnitPriceReachable(m: PlazaModel): boolean {
  if (billingMode(m) !== BILLING_MODE_IMAGE) return true
  const labels = new Set(requestIntervals(m).map((iv) => iv.tier_label))
  return !IMAGE_SIZE_TIERS.every((tier) => labels.has(tier))
}

/** 按次 / 按图单价的一档；label 为 null 表示默认价（没有档位时就是唯一的价格）。 */
export interface UnitPriceTier {
  label: string | null
  price: number
}

/** 实际可能生效的单价档：各档位 + 仍可能用到的默认价。卡片最低价与分组表共用，保证两处一致。 */
export function unitPriceTiers(m: PlazaModel): UnitPriceTier[] {
  const tiers: UnitPriceTier[] = requestIntervals(m).map((iv) => ({ label: tierLabel(iv), price: iv.per_request_price! }))
  const fallback = m.pricing?.per_request_price
  if (fallback != null && defaultUnitPriceReachable(m)) tiers.push({ label: null, price: fallback })
  return tiers
}

/** 按次 / 按图模式的最低单价，无价为 null。 */
export function lowestUnitPrice(m: PlazaModel): number | null {
  const prices = unitPriceTiers(m).map((tier) => tier.price)
  return prices.length ? Math.min(...prices) : null
}

/** 分时倍率时段（后端只给出倍率 ≠ 1 的时段，已升序）。 */
export function timePeriods(m: PlazaModel): PlazaTimePricingPeriod[] {
  return m.time_pricing?.periods ?? []
}

/** 已配置且有效的思考等级倍率，按等级顺序。 */
export function reasoningEffortMultipliers(m: PlazaModel): [string, number][] {
  const multipliers = m.pricing?.reasoning_effort_multipliers
  return REASONING_EFFORT_LEVELS.flatMap((effort) => {
    const multiplier = multipliers?.[effort]
    return typeof multiplier === 'number' && Number.isFinite(multiplier) && multiplier > 0
      ? [[effort, multiplier] as [string, number]]
      : []
  })
}

/**
 * 档位标签：优先后端/管理员给出的 tier_label，否则按区间生成统一形态——
 * 有上限为「≤上限」，末档为「>下限」；档位升序排列，相邻的 ≤100K / ≤200K 即表示 (100K,200K]。
 */
export function tierLabel(iv: UserPricingInterval): string {
  if (iv.tier_label) return iv.tier_label
  const { min_tokens: min, max_tokens: max } = iv
  return max == null ? `>${formatTokenCount(min)}` : `≤${formatTokenCount(max)}`
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${trimZero(n / 1_000_000)}M`
  if (n >= 1_000) return `${trimZero(n / 1_000)}K`
  return String(n)
}

function trimZero(n: number): string {
  return String(Math.round(n * 100) / 100)
}

/** “00:30–08:30”；整分钟的 HH:mm:ss 省略秒。 */
export function formatTimeWindow(p: PlazaTimePricingPeriod): string {
  const clock = (v: string) => v.replace(/^(\d{2}:\d{2}):00$/, '$1')
  return `${clock(p.start_time)}–${clock(p.end_time)}`
}

/** 倍率展示：去掉浮点噪声，如 0.1 × 3 → 0.3。 */
export function formatRate(rate: number): string {
  return String(Math.round(rate * 1000) / 1000)
}
