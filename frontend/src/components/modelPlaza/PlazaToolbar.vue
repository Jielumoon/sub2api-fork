<template>
  <div
    class="flex flex-wrap items-center justify-between gap-3"
  >
    <div class="flex items-center gap-2">
      <button
        type="button"
        class="btn btn-secondary btn-sm gap-1.5 lg:hidden"
        data-open-filters
        @click="emit('openFilters')"
      >
        <Icon name="filter" size="sm" />
        {{ t('modelPlaza.filters.title') }}
        <span
          v-if="activeFilterCount > 0"
          class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-gray-900 px-1 text-[10px] font-semibold text-white dark:bg-gray-100 dark:text-gray-900"
        >
          {{ activeFilterCount }}
        </span>
      </button>
      <p class="text-sm text-gray-500 dark:text-dark-400" aria-live="polite" data-result-count>
        <span class="font-display text-base font-semibold tabular-nums text-gray-900 dark:text-white">{{ count }}</span>
        <span v-if="count !== total" class="text-xs tabular-nums"> / {{ total }}</span>
        {{ t('modelPlaza.toolbar.models') }}
        <!-- 卡片上不再逐张写单位，这里是唯一的计价基准说明，窄屏也要留着（放不下就整段换行） -->
        <span class="ml-2 inline-block whitespace-nowrap text-xs text-gray-400 dark:text-dark-500" data-price-unit>{{ t('modelPlaza.price.perMillion') }}</span>
      </p>
    </div>

    <div class="flex items-center gap-2">
      <div class="w-40">
        <Select
          :model-value="sort"
          :options="sortOptions"
          :searchable="false"
          :aria-label="t('modelPlaza.sort.label')"
          @update:model-value="emit('update:sort', $event as PlazaSort)"
        />
      </div>
      <div
        role="group"
        :aria-label="t('modelPlaza.view.label')"
        class="inline-flex rounded-lg p-0.5 ring-1 ring-inset ring-gray-200 dark:ring-dark-700"
      >
        <button
          v-for="option in viewOptions"
          :key="option.value"
          type="button"
          class="rounded-md p-1.5 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
          :class="
            view === option.value
              ? 'bg-gray-100 text-gray-900 dark:bg-dark-700 dark:text-white'
              : 'text-gray-400 hover:text-gray-700 dark:text-dark-500 dark:hover:text-gray-200'
          "
          :aria-pressed="view === option.value"
          :aria-label="option.label"
          :title="option.label"
          :data-view="option.value"
          @click="emit('update:view', option.value)"
        >
          <Icon :name="option.icon" size="sm" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import type { PlazaSort, PlazaView } from '@/utils/plazaCatalog'

defineProps<{
  /** 当前筛选后的模型数。 */
  count: number
  /** 全部模型数。 */
  total: number
  sort: PlazaSort
  view: PlazaView
  /** 已生效的筛选维度数（窄屏「筛选」按钮上的徽章）。 */
  activeFilterCount: number
}>()

const emit = defineEmits<{
  'update:sort': [value: PlazaSort]
  'update:view': [value: PlazaView]
  openFilters: []
}>()

const { t } = useI18n()

const sortOptions = computed(() => [
  { value: 'default', label: t('modelPlaza.sort.default') },
  { value: 'name', label: t('modelPlaza.sort.name') },
  { value: 'price-asc', label: t('modelPlaza.sort.priceAsc') },
  { value: 'price-desc', label: t('modelPlaza.sort.priceDesc') }
])

const viewOptions = computed(() => [
  { value: 'card' as const, label: t('modelPlaza.view.card'), icon: 'grid' as const },
  { value: 'table' as const, label: t('modelPlaza.view.table'), icon: 'menu' as const }
])
</script>
