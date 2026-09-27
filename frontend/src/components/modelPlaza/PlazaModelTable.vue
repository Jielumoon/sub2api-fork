<template>
  <div class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700/70 dark:bg-dark-900/60">
    <div class="overflow-x-auto">
      <table class="w-full min-w-[820px] table-auto border-collapse text-sm tabular-nums">
        <thead>
          <tr class="border-b border-gray-200 text-left text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
            <th class="whitespace-nowrap py-3 pl-5 pr-3 font-medium">{{ t('modelPlaza.table.model') }}</th>
            <th class="whitespace-nowrap px-3 py-3 font-medium">{{ t('modelPlaza.filters.platform') }}</th>
            <th class="whitespace-nowrap px-3 py-3 font-medium">
              {{ t('modelPlaza.table.input') }}<span class="ml-1 font-normal text-gray-400 dark:text-dark-500">/ 1M</span>
            </th>
            <th class="whitespace-nowrap px-3 py-3 font-medium">
              {{ t('modelPlaza.table.output') }}<span class="ml-1 font-normal text-gray-400 dark:text-dark-500">/ 1M</span>
            </th>
            <th class="whitespace-nowrap px-3 py-3 font-medium">{{ t('modelPlaza.table.discount') }}</th>
            <th class="whitespace-nowrap py-3 pl-3 pr-5 font-medium">{{ t('modelPlaza.table.lowestGroup') }}</th>
          </tr>
        </thead>
        <TransitionGroup tag="tbody" name="plaza-row">
          <tr
            v-for="(row, index) in rows"
            :key="row.entry.key"
            class="cursor-pointer border-b border-gray-100 transition-colors last:border-b-0 hover:bg-gray-50 dark:border-dark-800 dark:hover:bg-dark-800/60"
            :style="{ '--i': Math.min(index, 11) }"
            data-model-row
            @click="emit('open', row.entry)"
          >
            <td class="py-3 pl-5 pr-3">
              <div class="flex min-w-0 items-center gap-2.5">
                <ModelIcon :model="row.entry.name" size="20px" class="shrink-0" />
                <button
                  type="button"
                  class="min-w-0 truncate text-left font-code text-[13px] font-medium text-gray-900 hover:text-primary-700 focus:outline-none focus-visible:rounded focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:text-white dark:hover:text-primary-300"
                  :title="row.entry.name"
                  data-open-model
                  @click.stop="emit('open', row.entry)"
                >
                  {{ row.entry.name }}
                </button>
                <button
                  type="button"
                  class="shrink-0 rounded p-1 text-gray-300 transition-colors hover:text-gray-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:text-dark-600 dark:hover:text-gray-300"
                  :aria-label="t('modelPlaza.card.copyName')"
                  :title="t('modelPlaza.card.copyName')"
                  @click.stop="copyToClipboard(row.entry.name, t('modelPlaza.card.copied'))"
                >
                  <Icon name="copy" size="xs" />
                </button>
                <span
                  v-for="badge in row.badges"
                  :key="badge"
                  class="shrink-0 rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-500 dark:bg-dark-800 dark:text-dark-300"
                >
                  {{ badge }}
                </span>
              </div>
            </td>
            <td class="px-3 py-3">
              <div class="flex items-center gap-1.5" :title="row.entry.platforms.map(platformLabel).join(', ')">
                <PlatformIcon v-for="p in row.entry.platforms" :key="p" :platform="p as GroupPlatform" size="sm" />
              </div>
            </td>
            <template v-if="!row.summary">
              <td colspan="2" class="px-3 py-3 text-gray-300 dark:text-dark-600">-</td>
            </template>
            <template v-else-if="row.summary.token">
              <td class="whitespace-nowrap px-3 py-3" data-input>
                <PlazaPrice
                  size="sm"
                  :paid="perMillion(row.summary.input)"
                  :original="row.summary.struck.input ? perMillion(row.summary.originalInput) : null"
                  :intro="intro"
                />
              </td>
              <td class="whitespace-nowrap px-3 py-3">
                <PlazaPrice
                  size="sm"
                  :paid="perMillion(row.summary.output)"
                  :original="row.summary.struck.output ? perMillion(row.summary.originalOutput) : null"
                  :intro="intro"
                />
              </td>
            </template>
            <td v-else colspan="2" class="whitespace-nowrap px-3 py-3" data-input>
              <PlazaPrice
                size="sm"
                :paid="paidUnitPrice(row.summary.unit, 1)"
                :suffix="plazaUnitSuffix(row.summary.offer, t)"
                :original="row.summary.struck.unit ? paidUnitPrice(row.summary.originalUnit, 1) : null"
                :intro="intro"
              />
            </td>
            <td class="whitespace-nowrap px-3 py-3 text-xs" data-discount>
              <span v-if="row.discount" class="font-display font-semibold text-primary-600 dark:text-primary-400">
                <span v-if="row.summary?.isFloor" class="mr-0.5 font-sans font-normal">{{ t('modelPlaza.discount.upTo') }}</span>{{ row.discount }}
              </span>
              <span v-else class="text-gray-300 dark:text-dark-600">-</span>
            </td>
            <td class="py-3 pl-3 pr-5 text-xs">
              <span v-if="row.summary" class="inline-flex min-w-0 items-center gap-1.5 text-gray-600 dark:text-dark-300">
                <PlatformIcon :platform="row.summary.offer.group.platform as GroupPlatform" size="xs" />
                <span class="truncate">{{ row.summary.offer.group.name }}</span>
                <span v-if="row.groupCount > 1" class="shrink-0 text-gray-400 dark:text-dark-500">
                  {{ t('modelPlaza.card.moreGroups', { count: row.groupCount - 1 }) }}
                </span>
              </span>
            </td>
          </tr>
        </TransitionGroup>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import PlazaPrice from './PlazaPrice.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { GroupPlatform } from '@/types'
import { groupCount, isDiscount, priceSummary, visibleOffers, type PlazaCatalogEntry, type PlazaFilters } from '@/utils/plazaCatalog'
import { paidUnitPrice, perMillion } from '@/utils/plazaPricing'
import { platformLabel } from '@/utils/platformColors'
import { discountText, plazaBadges, plazaUnitSuffix } from './plazaBadges'

const props = defineProps<{
  entries: PlazaCatalogEntry[]
  filters: PlazaFilters
  /** 首次加载：划线依次画出。 */
  intro?: boolean
}>()

const emit = defineEmits<{ open: [entry: PlazaCatalogEntry] }>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const rows = computed(() =>
  props.entries.map((entry) => {
    const offers = visibleOffers(entry, props.filters)
    const summary = priceSummary(entry, props.filters)
    return {
      entry,
      summary,
      discount: isDiscount(summary?.ratio) ? discountText(summary.ratio, t) : '',
      badges: plazaBadges(offers, t),
      groupCount: groupCount(offers)
    }
  })
)
</script>

<style scoped>
/* 排序 / 筛选时行滑到新位置，新出现的行淡入 */
@media (prefers-reduced-motion: no-preference) {
  .plaza-row-move {
    transition: transform 380ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .plaza-row-enter-active {
    transition: opacity 240ms ease;
  }

  .plaza-row-enter-from {
    opacity: 0;
  }
}
</style>
