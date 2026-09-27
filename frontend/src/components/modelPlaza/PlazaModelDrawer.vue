<template>
  <PlazaDrawer :show="entry !== null" width="sm:max-w-3xl" @close="emit('close')">
    <template #header="{ titleId }">
      <div v-if="shown" class="flex items-start gap-3">
        <span
          class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gray-50 ring-1 ring-inset ring-gray-100 dark:bg-dark-800 dark:ring-dark-700"
        >
          <ModelIcon :model="shown.name" size="26px" />
        </span>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-1.5">
            <h2
              :id="titleId"
              class="break-all font-code text-base font-semibold leading-snug text-gray-900 [overflow-wrap:anywhere] dark:text-white"
            >
              {{ shown.name }}
            </h2>
            <button
              type="button"
              class="shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:text-dark-500 dark:hover:bg-dark-800 dark:hover:text-gray-200"
              :aria-label="t('modelPlaza.card.copyName')"
              :title="t('modelPlaza.card.copyName')"
              @click="copyToClipboard(shown.name, t('modelPlaza.card.copied'))"
            >
              <Icon :name="copied ? 'check' : 'copy'" size="sm" />
            </button>
          </div>
          <div class="mt-1.5 flex flex-wrap items-center gap-1.5">
            <span
              v-for="p in shown.platforms"
              :key="p"
              :class="['inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] font-medium', platformBadgeLightClass(p)]"
            >
              <PlatformIcon :platform="p as GroupPlatform" size="xs" />
              {{ platformLabel(p) }}
            </span>
            <span class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('modelPlaza.card.groups', { count: groupCount(shown.offers) }) }}
            </span>
          </div>
        </div>
      </div>
    </template>

    <div v-if="shown" class="space-y-7 py-5">
      <!-- 最低价：和卡片同一套语言，打开时划线、盖章 -->
      <section v-if="summary" class="px-5" data-drawer-summary>
        <div class="relative rounded-2xl bg-gray-50 px-5 py-4 dark:bg-dark-800/60">
          <p class="text-xs text-gray-500 dark:text-dark-400">
            {{ t(summary.isFloor ? 'modelPlaza.drawer.lowestIn' : 'modelPlaza.drawer.priceIn', { group: summary.offer.group.name }) }}
          </p>
          <dl v-if="summary.token" class="mt-3 grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-x-4 gap-y-2.5 pr-28">
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.table.input') }}</dt>
            <dd>
              <PlazaPrice size="lg" :paid="perMillion(summary.input)" :original="summary.struck.input ? perMillion(summary.originalInput) : null" intro />
            </dd>
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.table.output') }}</dt>
            <dd>
              <PlazaPrice size="lg" :paid="perMillion(summary.output)" :original="summary.struck.output ? perMillion(summary.originalOutput) : null" intro />
            </dd>
          </dl>
          <div v-else class="mt-3 pr-28">
            <PlazaPrice
              size="lg"
              :paid="paidUnitPrice(summary.unit, 1)"
              :suffix="plazaUnitSuffix(summary.offer, t)"
              :original="summary.struck.unit ? paidUnitPrice(summary.originalUnit, 1) : null"
              intro
            />
          </div>
          <p v-if="summary.token" class="mt-2 text-[11px] text-gray-400 dark:text-dark-500">{{ t('modelPlaza.price.perMillion') }}</p>
          <div class="absolute inset-y-0 right-6 flex items-center">
            <PlazaStamp :ratio="summary.ratio" :floor="summary.isFloor" intro />
          </div>
        </div>
      </section>

      <!-- 按分组定价：抽屉的主要任务 -->
      <section>
        <div class="px-5">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('modelPlaza.drawer.byGroup') }}</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.drawer.byGroupHint') }}</p>
        </div>
        <PlazaOfferPricingTable
          class="mt-3"
          :offers="shown.offers"
          :highlight-group-id="highlightGroupId"
          :lowest-offer="summary?.offer ?? null"
        />
      </section>

      <!-- 官方参考价：不乘本站倍率 -->
      <section class="px-5">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('modelPlaza.drawer.official') }}</h3>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.drawer.officialHint') }}</p>
        <dl
          v-if="officialItems.length"
          class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-[repeat(auto-fit,minmax(7.5rem,1fr))]"
          data-official-prices
        >
          <div v-for="item in officialItems" :key="item.label" class="rounded-lg border border-gray-100 px-3 py-2 dark:border-dark-800">
            <dt class="text-[11px] text-gray-500 dark:text-dark-400">{{ item.label }}</dt>
            <dd class="mt-0.5 font-display text-sm font-semibold tabular-nums text-gray-900 dark:text-gray-50">{{ item.value }}</dd>
          </div>
        </dl>
        <p v-else class="mt-3 text-sm text-gray-400 dark:text-dark-500">{{ t('modelPlaza.drawer.noOfficial') }}</p>
        <table
          v-if="officialTiers.length > 1"
          class="mt-3 w-full text-left text-xs tabular-nums text-gray-600 dark:text-dark-300"
          data-official-tiers
        >
          <thead class="text-[11px] text-gray-400 dark:text-dark-500">
            <tr>
              <th class="py-1 pr-3 font-medium" :title="t('modelPlaza.table.tierHint')">{{ t('modelPlaza.drawer.tier') }}</th>
              <th class="py-1 pr-3 font-medium">{{ t('modelPlaza.table.input') }}</th>
              <th class="py-1 pr-3 font-medium">{{ t('modelPlaza.table.output') }}</th>
              <th class="py-1 pr-3 font-medium">{{ t('modelPlaza.table.cacheWrite') }}</th>
              <th class="py-1 font-medium">{{ t('modelPlaza.table.cacheRead') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(iv, idx) in officialTiers" :key="idx">
              <td class="py-0.5 pr-3 text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</td>
              <td class="py-0.5 pr-3">{{ perMillion(iv.input_price) }}</td>
              <td class="py-0.5 pr-3">{{ perMillion(iv.output_price) }}</td>
              <td class="py-0.5 pr-3">{{ perMillion(iv.cache_write_price) }}</td>
              <td class="py-0.5">{{ perMillion(iv.cache_read_price) }}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <!-- 调用示例 -->
      <section class="px-5">
        <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('modelPlaza.example.title') }}</h3>
        <PlazaCallExample :entry="shown" :base-url="baseUrl" />
      </section>
    </div>
  </PlazaDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import PlazaDrawer from './PlazaDrawer.vue'
import PlazaOfferPricingTable from './PlazaOfferPricingTable.vue'
import PlazaCallExample from './PlazaCallExample.vue'
import PlazaPrice from './PlazaPrice.vue'
import PlazaStamp from './PlazaStamp.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { GroupPlatform } from '@/types'
import { EMPTY_FILTERS, groupCount, priceSummary, type PlazaCatalogEntry } from '@/utils/plazaCatalog'
import { officialIntervals, paidUnitPrice, perMillion, tierLabel } from '@/utils/plazaPricing'
import { plazaUnitSuffix } from './plazaBadges'
import { platformBadgeLightClass, platformLabel } from '@/utils/platformColors'

const props = defineProps<{
  /** 为 null 时关闭抽屉。 */
  entry: PlazaCatalogEntry | null
  highlightGroupId?: number | null
  baseUrl: string
}>()

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

// 关闭动画期间 entry 已被清空，保留最后一个模型直到下次打开，避免内容在滑出时消失。
const shown = ref<PlazaCatalogEntry | null>(props.entry)
watch(
  () => props.entry,
  (entry) => {
    if (entry) shown.value = entry
  }
)

const officialItems = computed(() => {
  const o = shown.value?.official
  if (!o) return []
  const items = [
    { label: t('modelPlaza.table.input'), value: o.input_price },
    { label: t('modelPlaza.table.output'), value: o.output_price },
    { label: t('modelPlaza.table.cacheWrite'), value: o.cache_write_price },
    { label: `${t('modelPlaza.table.cacheWrite')} (1h)`, value: o.cache_write_1h_price },
    { label: t('modelPlaza.table.cacheRead'), value: o.cache_read_price }
  ]
  return items.filter((item) => item.value != null).map((item) => ({ ...item, value: perMillion(item.value) }))
})

/** 抽屉讲的是模型本身：最低价在全部分组里找，不受列表筛选影响。 */
const summary = computed(() => (shown.value ? priceSummary(shown.value, EMPTY_FILTERS) : null))

const officialTiers = computed(() => (shown.value ? officialIntervals({ official_pricing: shown.value.official }) : []))
</script>
