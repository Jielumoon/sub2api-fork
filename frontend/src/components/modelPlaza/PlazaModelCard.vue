<template>
  <article
    class="plaza-card relative flex h-full min-w-0 flex-col rounded-2xl border border-gray-200 bg-white p-5 transition-colors duration-200 focus-within:border-primary-400 hover:border-gray-300 dark:border-dark-700/70 dark:bg-dark-900/60 dark:hover:border-dark-600"
  >
    <header class="flex items-start gap-3">
      <ModelIcon :model="entry.name" size="28px" class="mt-0.5 shrink-0" />
      <div class="min-w-0 flex-1">
        <!-- 名称按钮用伪元素铺满整张卡片：整卡可点，复制按钮叠在上层不冲突 -->
        <h3
          class="line-clamp-2 font-code text-[15px] font-semibold leading-snug text-gray-900 [overflow-wrap:anywhere] dark:text-white"
        >
          <button
            type="button"
            class="text-left after:absolute after:inset-0 after:rounded-2xl after:content-[''] focus:outline-none focus-visible:after:ring-2 focus-visible:after:ring-primary-500/50"
            :title="entry.name"
            data-open-model
            @click="emit('open')"
          >
            {{ entry.name }}
          </button>
        </h3>
        <p class="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
          <span v-for="p in entry.platforms" :key="p" class="inline-flex items-center gap-1">
            <PlatformIcon :platform="p as GroupPlatform" size="xs" />
            {{ platformLabel(p) }}
          </span>
        </p>
      </div>
      <button
        type="button"
        class="relative z-10 -mr-1.5 -mt-1 shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:text-dark-500 dark:hover:bg-dark-800 dark:hover:text-gray-200"
        :aria-label="t('modelPlaza.card.copyName')"
        :title="t('modelPlaza.card.copyName')"
        data-copy-model
        @click="copyToClipboard(entry.name, t('modelPlaza.card.copied'))"
      >
        <Icon :name="copied ? 'check' : 'copy'" size="sm" />
      </button>
    </header>

    <ul v-if="badges.length" class="mt-3 flex flex-wrap gap-1.5">
      <li
        v-for="badge in badges"
        :key="badge"
        class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500 dark:bg-dark-800 dark:text-dark-300"
      >
        {{ badge }}
      </li>
    </ul>

    <!-- 价格：大字是实付价，旁边划掉的是原价；折扣印章压在右侧 -->
    <div v-if="summary" class="relative mt-auto pt-5" data-price-summary>
      <dl v-if="summary.token" class="grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-x-3 gap-y-2.5 pr-[5.75rem]">
        <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.table.input') }}</dt>
        <dd>
          <PlazaPrice
            size="lg"
            :paid="perMillion(summary.input)"
            :original="summary.struck.input ? perMillion(summary.originalInput) : null"
            :intro="intro"
          />
        </dd>
        <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('modelPlaza.table.output') }}</dt>
        <dd>
          <PlazaPrice
            size="lg"
            :paid="perMillion(summary.output)"
            :original="summary.struck.output ? perMillion(summary.originalOutput) : null"
            :intro="intro"
          />
        </dd>
      </dl>
      <div v-else class="pr-[5.75rem]">
        <PlazaPrice
          size="lg"
          :paid="paidUnitPrice(summary.unit, 1)"
          :suffix="unitSuffix"
          :original="summary.struck.unit ? paidUnitPrice(summary.originalUnit, 1) : null"
          :intro="intro"
        />
      </div>
      <div class="absolute bottom-0 right-0 top-5 flex items-center">
        <PlazaStamp :ratio="summary.ratio" :floor="summary.isFloor" :intro="intro" />
      </div>
    </div>

    <!-- 价格出自哪个分组，其余分组在详情里比 -->
    <footer
      class="mt-4 flex items-center justify-between gap-3 border-t border-gray-100 pt-3 text-xs dark:border-dark-800"
    >
      <span v-if="summary" class="inline-flex min-w-0 items-center gap-1.5 text-gray-600 dark:text-dark-300" data-best-group>
        <PlatformIcon :platform="summary.offer.group.platform as GroupPlatform" size="xs" />
        <span class="truncate">{{ summary.offer.group.name }}</span>
      </span>
      <span class="inline-flex shrink-0 items-center gap-0.5 text-gray-400 dark:text-dark-500">
        <template v-if="groups > 1">{{ t('modelPlaza.card.moreGroups', { count: groups - 1 }) }}</template>
        <Icon name="chevronRight" size="sm" class="plaza-card-chevron" />
      </span>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import PlazaPrice from './PlazaPrice.vue'
import PlazaStamp from './PlazaStamp.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { GroupPlatform } from '@/types'
import { groupCount, priceSummary, visibleOffers, type PlazaCatalogEntry, type PlazaFilters } from '@/utils/plazaCatalog'
import { paidUnitPrice, perMillion } from '@/utils/plazaPricing'
import { platformLabel } from '@/utils/platformColors'
import { plazaBadges, plazaUnitSuffix } from './plazaBadges'

const props = defineProps<{
  entry: PlazaCatalogEntry
  filters: PlazaFilters
  /** 首次加载：划线、盖章依次播放。 */
  intro?: boolean
}>()

const emit = defineEmits<{ open: [] }>()

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const offers = computed(() => visibleOffers(props.entry, props.filters))
const summary = computed(() => priceSummary(props.entry, props.filters))
const groups = computed(() => groupCount(offers.value))
const badges = computed(() => plazaBadges(offers.value, t))
const unitSuffix = computed(() => (summary.value ? plazaUnitSuffix(summary.value.offer, t) : ''))
</script>

<style scoped>
/* 悬停时印章轻轻回正一点，箭头往前挪一步：都在回应「这张卡可以点」 */
.plaza-card:hover {
  --stamp-tilt: -3deg;
}

@media (prefers-reduced-motion: no-preference) {
  .plaza-card-chevron {
    transition: transform 200ms ease;
  }

  .plaza-card:hover .plaza-card-chevron {
    transform: translateX(2px);
  }
}
</style>
