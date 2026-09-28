<template>
  <div class="plaza-offer-table overflow-x-auto">
    <table class="w-full min-w-[560px] table-auto border-collapse text-sm tabular-nums">
      <thead>
        <tr
          class="border-b border-gray-200 text-left text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400"
        >
          <th class="whitespace-nowrap py-2 pl-5 pr-3 font-medium">{{ t('modelPlaza.drawer.group') }}</th>
          <th class="whitespace-nowrap px-3 py-2 text-right font-medium">{{ t('modelPlaza.table.rate') }}</th>
          <th class="whitespace-nowrap px-3 py-2 font-medium">{{ t('modelPlaza.table.input') }}</th>
          <th class="whitespace-nowrap px-3 py-2 font-medium">{{ t('modelPlaza.table.output') }}</th>
          <th class="whitespace-nowrap py-2 pl-3 pr-5 font-medium">{{ t('modelPlaza.table.cache') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="({ offer, period, key }, index) in rows"
          :key="key"
          class="plaza-offer-row border-b border-gray-100 align-top transition-colors last:border-b-0 dark:border-dark-800"
          :class="
            offer.group.id === highlightGroupId
              ? 'bg-primary-50/70 dark:bg-primary-900/15'
              : 'hover:bg-gray-50/70 dark:hover:bg-dark-800/50'
          "
          :data-group-id="offer.group.id"
          :style="{ '--r': Math.min(index, 12) }"
        >
          <!-- 分组：徽章 + 专属/订阅；标准行附描述与高峰，时段行标注时段 -->
          <td class="py-2.5 pl-5 pr-3">
            <div class="flex flex-wrap items-center gap-1.5">
              <GroupBadge
                :name="offer.group.name"
                :platform="offer.group.platform as GroupPlatform"
                :subscription-type="(offer.group.subscription_type || 'standard') as SubscriptionType"
                :show-rate="false"
              />
              <span
                v-if="period"
                class="inline-flex items-center whitespace-nowrap rounded-md bg-gray-100 px-1 py-0.5 font-mono text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
                :title="timePricingRowHint(offer)"
              >
                <span v-if="offer.model.time_pricing?.weekdays_only" class="mr-1 font-sans">{{
                  t('modelPlaza.table.timePricingWeekdays')
                }}</span>
                {{ formatTimeWindow(period) }}
              </span>
              <template v-else>
                <span
                  v-if="offer === lowestOffer && offers.length > 1"
                  class="rounded-md bg-primary-600 px-1.5 py-0.5 text-[10px] font-semibold text-white dark:bg-primary-500"
                  data-lowest
                >
                  {{ t('modelPlaza.drawer.lowestBadge') }}
                </span>
                <span
                  v-if="offer.group.is_exclusive"
                  class="inline-flex items-center gap-1 rounded-md bg-purple-50 px-1.5 py-0.5 text-[10px] font-medium text-purple-600 dark:bg-purple-900/20 dark:text-purple-400"
                >
                  <Icon name="shield" size="xs" class="h-3 w-3" />
                  {{ t('modelPlaza.badges.exclusive') }}
                </span>
                <span
                  v-if="offer.group.subscription_type === 'subscription'"
                  class="rounded-md bg-violet-50 px-1.5 py-0.5 text-[10px] font-medium text-violet-600 dark:bg-violet-900/20 dark:text-violet-400"
                >
                  {{ t('modelPlaza.badges.subscription') }}
                </span>
                <span
                  v-if="offer.model.platform !== offer.group.platform"
                  :class="['rounded-md px-1.5 py-0.5 text-[10px] font-medium', platformBadgeLightClass(offer.model.platform)]"
                >
                  {{ platformLabel(offer.model.platform) }}
                </span>
                <span
                  v-if="!isTokenBilled(offer.model)"
                  class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
                >
                  {{ billingModeLabel(offer.model, t) }}
                </span>
                <span
                  v-if="offer.model.long_context_basis === 'marginal'"
                  class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
                  :title="t('modelPlaza.table.tierHintMarginal')"
                >
                  {{ t('modelPlaza.table.marginalBadge') }}
                </span>
                <span
                  v-if="longContextDisabled(offer)"
                  class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
                  :title="t('modelPlaza.detail.longContextDisabledNote')"
                  data-long-context-off
                >
                  {{ t('modelPlaza.drawer.longContextOff') }}
                </span>
                <span
                  v-for="([effort, multiplier]) in reasoningEffortMultipliers(offer.model)"
                  :key="effort"
                  class="rounded-md bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
                  :title="t('modelPlaza.table.reasoningMultiplierHint', { effort, multiplier })"
                  :data-reasoning-effort="effort"
                >
                  {{ t('modelPlaza.table.reasoningMultiplierBadge', { effort, multiplier }) }}
                </span>
              </template>
            </div>
            <template v-if="!period">
              <p
                v-if="offer.group.description"
                class="mt-1 line-clamp-2 max-w-xs text-xs text-gray-500 dark:text-dark-400"
                :title="offer.group.description"
              >
                {{ offer.group.description }}
              </p>
              <p
                v-if="peakWindow(offer)"
                class="mt-1 inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400"
              >
                <Icon name="clock" size="xs" class="h-3 w-3" />
                {{ t('modelPlaza.detail.peakNote', { window: peakWindow(offer), multiplier: offer.group.peak_rate_multiplier }) }}
              </p>
            </template>
          </td>

          <!-- 倍率：时段行 = 生效倍率 × 时段倍率；图片 / 视频独立倍率行 = 独立倍率；专属倍率划线原倍率 -->
          <td class="whitespace-nowrap px-3 py-2.5 text-right font-display text-xs" data-rate>
            <span
              v-if="period"
              class="font-bold text-primary-600 dark:text-primary-400"
              :title="t('modelPlaza.table.timePricingRateHint', { rate: groupRate(offer.group), multiplier: period.multiplier })"
              >{{ formatRate(rowRate(offer, period)) }}x</span
            >
            <span v-else-if="usesIndependentMediaRate(offer.model, offer.group)" class="font-bold text-gray-700 dark:text-gray-300"
              >{{ formatRate(rowRate(offer, null)) }}x</span
            >
            <template v-else-if="hasCustomRate(offer.group)">
              <span class="mr-1 text-gray-400 line-through dark:text-dark-500">{{ offer.group.rate_multiplier }}x</span>
              <span class="font-bold text-primary-600 dark:text-primary-400">{{ formatRate(groupRate(offer.group)) }}x</span>
            </template>
            <span v-else class="font-bold text-gray-700 dark:text-gray-300">{{ formatRate(groupRate(offer.group)) }}x</span>
          </td>

          <!-- token 计费：输入 / 输出 / 缓存，有阶梯时每档一行；档位标签只放输入列 -->
          <template v-if="isTokenBilled(offer.model)">
            <td class="px-3 py-2.5 font-display font-semibold text-gray-900 dark:text-gray-50">
              <template v-if="tokenIntervals(offer.model).length">
                <div v-for="(iv, idx) in tokenIntervals(offer.model)" :key="idx" class="whitespace-nowrap text-xs leading-5">
                  <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500" :title="tierHint(offer)">{{ tierLabel(iv) }}</span>
                  {{ paidPerMillion(iv.input_price, rowRate(offer, period)) }}
                </div>
              </template>
              <template v-else>{{ paidPerMillion(offer.model.pricing?.input_price, rowRate(offer, period)) }}</template>
            </td>
            <td class="px-3 py-2.5 font-display font-semibold text-gray-900 dark:text-gray-50">
              <template v-if="tokenIntervals(offer.model).length">
                <div
                  v-for="(iv, idx) in tokenIntervals(offer.model)"
                  :key="idx"
                  class="whitespace-nowrap text-xs leading-5"
                  :title="tierHint(offer)"
                >
                  {{ paidPerMillion(iv.output_price, rowRate(offer, period)) }}
                </div>
              </template>
              <template v-else>{{ paidPerMillion(offer.model.pricing?.output_price, rowRate(offer, period)) }}</template>
            </td>
            <td class="py-2.5 pl-3 pr-5">
              <template v-if="hasTierCachePricing(tokenIntervals(offer.model))">
                <div
                  v-for="(iv, idx) in tokenIntervals(offer.model)"
                  :key="idx"
                  class="whitespace-nowrap font-display text-xs leading-5 text-gray-800 dark:text-gray-200"
                  :title="tierHint(offer)"
                >
                  <template v-if="hasCachePricing(iv)">
                    <span class="font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheWriteShort') }}</span>
                    {{ paidPerMillion(iv.cache_write_price, rowRate(offer, period)) }}
                    <template v-if="iv.cache_write_1h_price != null"
                      ><span class="font-sans font-normal text-gray-400 dark:text-dark-500"> (1h </span>{{ paidPerMillion(iv.cache_write_1h_price, rowRate(offer, period))
                      }}<span class="font-sans font-normal text-gray-400 dark:text-dark-500">)</span></template
                    >
                    <span class="ml-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheReadShort') }}</span>
                    {{ paidPerMillion(iv.cache_read_price, rowRate(offer, period)) }}
                  </template>
                  <span v-else class="text-gray-400 dark:text-dark-500">-</span>
                </div>
              </template>
              <div
                v-else-if="hasCachePricing(offer.model.pricing)"
                class="space-y-0.5 whitespace-nowrap font-display text-xs text-gray-800 dark:text-gray-200"
              >
                <div>
                  <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheWrite') }}</span>
                  {{ paidPerMillion(offer.model.pricing?.cache_write_price, rowRate(offer, period))
                  }}<template v-if="offer.model.pricing?.cache_write_1h_price != null"
                    ><span class="font-sans font-normal text-gray-400 dark:text-dark-500"> (1h </span>{{ paidPerMillion(offer.model.pricing.cache_write_1h_price, rowRate(offer, period))
                    }}<span class="font-sans font-normal text-gray-400 dark:text-dark-500">)</span></template
                  >
                </div>
                <div>
                  <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheRead') }}</span>
                  {{ paidPerMillion(offer.model.pricing?.cache_read_price, rowRate(offer, period)) }}
                </div>
              </div>
              <span v-else class="text-gray-400 dark:text-dark-500">-</span>
            </td>
          </template>

          <!-- 按次 / 按图片：三列合并；有档位时逐档列出（仍可能生效的默认价记为「其他」），否则单一单价 -->
          <td v-else colspan="3" class="py-2.5 pl-3 pr-5">
            <div v-if="unitPriceTiers(offer.model).some((tier) => tier.label)" class="flex flex-wrap items-center gap-1.5">
              <span
                v-for="(tier, idx) in unitPriceTiers(offer.model)"
                :key="idx"
                class="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-0.5 font-display text-xs text-gray-800 dark:bg-dark-700/60 dark:text-gray-200"
                data-unit-tier
              >
                <span class="font-sans text-gray-400 dark:text-dark-500">{{ tier.label ?? t('modelPlaza.table.otherTier') }}</span>
                {{ paidUnitPrice(tier.price, rowRate(offer, period))
                }}<span class="font-sans text-gray-400 dark:text-dark-500">{{ plazaUnitSuffix(offer, t) }}</span>
              </span>
            </div>
            <template v-else-if="unitPriceTiers(offer.model).length">
              <span class="font-display font-semibold text-gray-900 dark:text-gray-50">
                {{ paidUnitPrice(unitPriceTiers(offer.model)[0].price, rowRate(offer, period)) }}
              </span>
              <span class="ml-1 text-xs text-gray-400 dark:text-dark-500">{{ plazaUnitSuffix(offer, t) }}</span>
            </template>
            <span v-else class="text-gray-400 dark:text-dark-500">-</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import type { PlazaTimePricingPeriod } from '@/api/modelPlaza'
import type { GroupPlatform, SubscriptionType } from '@/types'
import type { PlazaOffer } from '@/utils/plazaCatalog'
import {
  formatRate,
  formatTimeWindow,
  groupRate,
  hasCachePricing,
  hasCustomRate,
  hasTierCachePricing,
  isTokenBilled,
  officialIntervals,
  offerRate,
  paidPerMillion,
  paidUnitPrice,
  periodRate,
  reasoningEffortMultipliers,
  tierLabel,
  timePeriods,
  tokenIntervals,
  unitPriceTiers,
  usesIndependentMediaRate
} from '@/utils/plazaPricing'
import { platformBadgeLightClass, platformLabel } from '@/utils/platformColors'
import { hasPeakRate, serverTimezoneLabel } from '@/utils/peak-rate'
import { useAppStore } from '@/stores/app'
import { billingModeLabel, plazaUnitSuffix } from './plazaBadges'

const props = defineProps<{
  offers: PlazaOffer[]
  /** 当前筛选选中的分组，对应行高亮。 */
  highlightGroupId?: number | null
  /** 最便宜的报价，标「最低」。 */
  lowestOffer?: PlazaOffer | null
}>()

const { t } = useI18n()
const appStore = useAppStore()

/** 表格行：每个报价一行标准价；配置了分时倍率的再按时段各加一行。 */
interface OfferRow {
  offer: PlazaOffer
  period: PlazaTimePricingPeriod | null
  key: string
}

const rows = computed<OfferRow[]>(() =>
  props.offers.flatMap((offer) => {
    const base = `${offer.group.id}:${offer.model.platform}`
    return [
      { offer, period: null, key: base },
      ...timePeriods(offer.model).map((period, idx) => ({ offer, period, key: `${base}:${idx}` }))
    ]
  })
)

/** 行实付倍率：时段行再乘时段倍率。 */
function rowRate(offer: PlazaOffer, period: PlazaTimePricingPeriod | null): number {
  const rate = offerRate(offer.model, offer.group)
  return period ? periodRate(rate, period) : rate
}

/** 档位说明：整单按档计价，或（平台旧规则）仅超出部分按档计价。 */
function tierHint(offer: PlazaOffer): string {
  return offer.model.long_context_basis === 'marginal'
    ? t('modelPlaza.table.tierHintMarginal')
    : t('modelPlaza.table.tierHint')
}

/**
 * 高峰窗口「14:00-18:00 (UTC+08:00)」，分组未启用高峰为空串。
 * 不含倍率：引用它的两条文案都另外写了 ×{multiplier}。
 */
function peakWindow(offer: PlazaOffer): string {
  if (!hasPeakRate(offer.group)) return ''
  const range = `${offer.group.peak_start}-${offer.group.peak_end}`
  const tz = serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)
  return tz ? `${range} (${tz})` : range
}

/**
 * 分组关闭了长上下文阶梯、但该模型官方带阶梯：实付只按基础档，官方阶梯仅供参考。
 * 字段缺失（旧后端）不提示。
 */
function longContextDisabled(offer: PlazaOffer): boolean {
  return offer.group.long_context_pricing_enabled === false && officialIntervals(offer.model).length > 1
}

/**
 * 时段行 tooltip：仅工作日生效的配置换用带周末回落说明的文案；
 * 分组启用高峰倍率时追加披露——本行价格不含高峰因子，与高峰窗口重叠的部分实付再乘高峰倍率。
 */
function timePricingRowHint(offer: PlazaOffer): string {
  const timePricing = offer.model.time_pricing
  const key = timePricing?.weekdays_only ? 'modelPlaza.table.timePricingRowHintWeekdays' : 'modelPlaza.table.timePricingRowHint'
  let hint = t(key, { timezone: timePricing?.timezone })
  const window = peakWindow(offer)
  if (window) {
    hint += t('modelPlaza.table.timePricingRowHintPeak', { window, multiplier: offer.group.peak_rate_multiplier ?? 1 })
  }
  return hint
}
</script>

<style scoped>
/* 抽屉打开时各分组依次滑入，先看到的是最便宜的那一行 */
@media (prefers-reduced-motion: no-preference) {
  .plaza-offer-row {
    animation: plaza-row-in 320ms cubic-bezier(0.22, 1, 0.36, 1) both;
    animation-delay: calc(var(--r, 0) * 40ms + 140ms);
  }
}

@keyframes plaza-row-in {
  from {
    opacity: 0;
    transform: translateX(12px);
  }
}
</style>
