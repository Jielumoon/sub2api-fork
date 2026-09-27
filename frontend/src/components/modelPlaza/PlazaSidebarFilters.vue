<template>
  <div class="space-y-1">
    <div class="mb-2 flex items-center justify-between gap-2">
      <h2 v-if="showTitle" class="text-sm font-bold text-gray-900 dark:text-white">{{ t('modelPlaza.filters.title') }}</h2>
      <span v-else></span>
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-medium text-gray-500 transition-colors enabled:hover:bg-gray-100 enabled:hover:text-gray-800 disabled:cursor-not-allowed disabled:opacity-40 dark:text-dark-400 dark:enabled:hover:bg-dark-800 dark:enabled:hover:text-gray-200"
        :disabled="!hasActiveFilters"
        data-reset-filters
        @click="emit('reset')"
      >
        <Icon name="refresh" size="xs" class="h-3.5 w-3.5" />
        {{ t('modelPlaza.filters.reset') }}
      </button>
    </div>

    <section
      v-for="section in sections"
      :key="section.key"
      class="border-b border-gray-100 pb-3 last:border-b-0 dark:border-dark-700/60"
      :aria-labelledby="`plaza-filter-${section.key}`"
    >
      <h3
        :id="`plaza-filter-${section.key}`"
        class="py-2 text-xs font-medium text-gray-500 dark:text-dark-400"
      >
        {{ section.title }}
      </h3>
      <div class="flex flex-wrap gap-1.5">
        <button
          type="button"
          class="rounded-full px-3 py-1 text-xs font-medium transition-colors"
          :class="chipClass(section.value === FILTER_ALL)"
          :aria-pressed="section.value === FILTER_ALL"
          @click="section.select(FILTER_ALL)"
        >
          {{ t('modelPlaza.filters.all') }}
        </button>
        <button
          v-for="option in section.options"
          :key="option.value"
          type="button"
          class="inline-flex max-w-full items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-35"
          :class="chipClass(section.value === option.value)"
          :aria-pressed="section.value === option.value"
          :disabled="option.count === 0 && section.value !== option.value"
          :data-filter="`${section.key}:${option.value}`"
          @click="section.select(option.value)"
        >
          <PlatformIcon v-if="option.icon" :platform="option.icon as GroupPlatform" size="xs" />
          <span class="truncate">{{ option.label }}</span>
          <span v-if="option.suffix" class="tabular-nums opacity-70">{{ option.suffix }}</span>
          <span class="tabular-nums opacity-50">{{ option.count }}</span>
        </button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import type { GroupPlatform } from '@/types'
import { FILTER_ALL, type PlazaFacetCounts, type PlazaFilters } from '@/utils/plazaCatalog'
import { formatRate, groupRate } from '@/utils/plazaPricing'
import { platformLabel } from '@/utils/platformColors'

const props = withDefaults(defineProps<{
  filters: PlazaFilters
  counts: PlazaFacetCounts
  /** 数据中出现的具体平台（升序）。 */
  platforms: string[]
  /** 有模型的分组（接口顺序，已按倍率升序）。 */
  groups: ModelPlazaGroup[]
  /** 数据中出现的计费模式。 */
  billingModes: string[]
  /** 放进抽屉时由抽屉标题代替。 */
  showTitle?: boolean
}>(), { showTitle: true })

const emit = defineEmits<{
  update: [patch: Partial<PlazaFilters>]
  reset: []
}>()

const { t } = useI18n()

const hasActiveFilters = computed(
  () =>
    props.filters.platform !== FILTER_ALL ||
    props.filters.groupId !== FILTER_ALL ||
    props.filters.billing !== FILTER_ALL
)

interface FilterOption {
  value: string | number
  label: string
  count: number
  suffix?: string
  icon?: string
}

const sections = computed(() => [
  {
    key: 'platform',
    title: t('modelPlaza.filters.platform'),
    value: props.filters.platform as string | number,
    select: (value: string | number) => emit('update', { platform: String(value) }),
    options: props.platforms.map<FilterOption>((p) => ({
      value: p,
      label: platformLabel(p),
      count: props.counts.platform.get(p) ?? 0,
      icon: p
    }))
  },
  {
    key: 'group',
    title: t('modelPlaza.filters.group'),
    value: props.filters.groupId as string | number,
    select: (value: string | number) =>
      emit('update', { groupId: value === FILTER_ALL ? FILTER_ALL : Number(value) }),
    options: props.groups.map<FilterOption>((g) => ({
      value: g.id,
      label: g.name,
      count: props.counts.group.get(g.id) ?? 0,
      suffix: `${formatRate(groupRate(g))}x`,
      icon: g.platform
    }))
  },
  {
    key: 'billing',
    title: t('modelPlaza.filters.billing'),
    value: props.filters.billing as string | number,
    select: (value: string | number) => emit('update', { billing: String(value) }),
    options: props.billingModes.map<FilterOption>((mode) => ({
      value: mode,
      label: t(`modelPlaza.billing.${mode}`),
      count: props.counts.billing.get(mode) ?? 0
    }))
  }
])

/** 颜色留给价格和印章：筛选只用墨色，选中即反白。 */
function chipClass(active: boolean): string {
  return active
    ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'
    : 'text-gray-600 ring-1 ring-inset ring-gray-200 enabled:hover:text-gray-900 enabled:hover:ring-gray-400 dark:text-dark-300 dark:ring-dark-700 dark:enabled:hover:text-white dark:enabled:hover:ring-dark-500'
}
</script>
