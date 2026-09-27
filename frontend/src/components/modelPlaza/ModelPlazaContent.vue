<template>
  <div class="space-y-6">
    <!-- 页头：标题与搜索同一行；后台形态 AppHeader 已有标题，只留搜索 -->
    <header
      class="flex flex-col gap-5"
      :class="embedded ? '' : 'pt-2 sm:flex-row sm:items-end sm:justify-between sm:gap-10 sm:pt-6'"
    >
      <h1
        v-if="!embedded"
        class="font-display text-4xl font-[750] tracking-tight text-gray-900 dark:text-white sm:text-5xl"
      >
        {{ t('modelPlaza.title') }}
      </h1>
      <div class="relative w-full max-w-md">
        <Icon
          name="search"
          size="md"
          class="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-500"
        />
        <input
          v-model="filters.search"
          type="search"
          :placeholder="t('modelPlaza.searchPlaceholder')"
          :aria-label="t('modelPlaza.searchPlaceholder')"
          class="input rounded-full py-2.5 pl-11 pr-11 text-sm [&::-webkit-search-cancel-button]:hidden"
          data-plaza-search
        />
        <button
          v-if="filters.search"
          type="button"
          class="absolute right-3.5 top-1/2 -translate-y-1/2 rounded-full p-1 text-gray-400 transition-colors hover:text-gray-600 dark:text-dark-500 dark:hover:text-gray-300"
          :aria-label="t('modelPlaza.clearSearch')"
          @click="filters.search = ''"
        >
          <Icon name="x" size="sm" />
        </button>
      </div>
    </header>

    <!-- 全局价格说明(管理员配置,Markdown) -->
    <div
      v-if="descriptionHtml"
      class="plaza-description rounded-2xl border border-gray-200 bg-white px-5 py-4 text-sm dark:border-dark-700/70 dark:bg-dark-900/60"
      v-html="descriptionHtml"
    ></div>

    <!-- 未登录提示 -->
    <p v-if="!isAuthenticated" class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400">
      <Icon name="infoCircle" size="xs" class="h-3.5 w-3.5" />
      {{ t('modelPlaza.anonymousHint') }}
    </p>

    <!-- 加载：骨架屏 -->
    <div v-if="loading" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3" aria-busy="true" :aria-label="t('modelPlaza.loading')">
      <div
        v-for="i in 6"
        :key="i"
        class="h-52 animate-pulse rounded-2xl border border-gray-200 bg-white dark:border-dark-700/70 dark:bg-dark-900/60"
      ></div>
    </div>
    <div
      v-else-if="error"
      role="alert"
      class="rounded-2xl border border-red-200 bg-red-50 px-5 py-8 text-center text-sm text-red-600 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
    >
      {{ t('modelPlaza.loadFailed') }}
    </div>
    <div
      v-else-if="catalog.length === 0"
      class="rounded-2xl border border-dashed border-gray-300 px-5 py-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400"
    >
      {{ t('modelPlaza.empty') }}
    </div>
    <div v-else class="grid gap-8 lg:grid-cols-[220px_minmax(0,1fr)]">
      <aside class="hidden lg:block">
        <div class="sticky top-20 max-h-[calc(100dvh-6rem)] overflow-y-auto pb-4 pr-1">
          <PlazaSidebarFilters v-bind="sidebarProps" @update="patchFilters" @reset="resetFilters" />
        </div>
      </aside>

      <main class="min-w-0 space-y-4">
        <PlazaToolbar
          v-model:sort="sort"
          v-model:view="view"
          :count="filtered.length"
          :total="catalog.length"
          :active-filter-count="activeFilterCount"
          @open-filters="mobileFiltersOpen = true"
        />

        <!-- 卡片 / 表格切换时交叉淡入；卡片在筛选、排序时滑到新位置 -->
        <Transition name="plaza-fade" mode="out-in">
          <TransitionGroup
            v-if="sorted.length && view === 'card'"
            key="cards"
            tag="div"
            name="plaza-grid"
            class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3"
            data-plaza-cards
          >
            <PlazaModelCard
              v-for="(entry, index) in sorted"
              :key="entry.key"
              :entry="entry"
              :filters="filters"
              :intro="introActive"
              :style="{ '--i': Math.min(index, INTRO_STAGGER_CAP) }"
              @open="openEntry(entry)"
            />
          </TransitionGroup>
          <PlazaModelTable
            v-else-if="sorted.length"
            key="table"
            :entries="sorted"
            :filters="filters"
            :intro="introActive"
            @open="openEntry"
          />
          <div
            v-else
            key="empty"
            class="rounded-2xl border border-dashed border-gray-300 px-5 py-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400"
          >
            <p>{{ t('modelPlaza.noSearchResult') }}</p>
            <button type="button" class="btn btn-secondary btn-sm mt-3" @click="clearAll">
              {{ t('modelPlaza.filters.clearAll') }}
            </button>
          </div>
        </Transition>
      </main>
    </div>

    <!-- 窄屏：筛选收进左侧抽屉 -->
    <PlazaDrawer
      :show="mobileFiltersOpen"
      side="left"
      width="max-w-sm"
      :title="t('modelPlaza.filters.title')"
      @close="mobileFiltersOpen = false"
    >
      <div class="p-5">
        <PlazaSidebarFilters v-bind="sidebarProps" :show-title="false" @update="patchFilters" @reset="resetFilters" />
      </div>
    </PlazaDrawer>

    <PlazaModelDrawer
      :entry="selectedEntry"
      :highlight-group-id="filters.groupId === FILTER_ALL ? null : filters.groupId"
      :base-url="baseUrl"
      @close="selectedKey = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import '@fontsource-variable/bricolage-grotesque'
import '@fontsource-variable/jetbrains-mono'
import Icon from '@/components/icons/Icon.vue'
import PlazaDrawer from './PlazaDrawer.vue'
import PlazaModelCard from './PlazaModelCard.vue'
import PlazaModelDrawer from './PlazaModelDrawer.vue'
import PlazaModelTable from './PlazaModelTable.vue'
import PlazaSidebarFilters from './PlazaSidebarFilters.vue'
import PlazaToolbar from './PlazaToolbar.vue'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
  BILLING_MODE_VIDEO
} from '@/constants/channel'
import {
  EMPTY_FILTERS,
  FILTER_ALL,
  buildPlazaCatalog,
  facetCounts,
  filterCatalog,
  sortCatalog,
  type PlazaCatalogEntry,
  type PlazaFilters,
  type PlazaSort,
  type PlazaView
} from '@/utils/plazaCatalog'
import { billingMode, groupRate } from '@/utils/plazaPricing'
import { toApiRoot } from '@/utils/url'

const props = defineProps<{
  response: ModelPlazaResponse | null
  loading: boolean
  error?: boolean
  /** 后台内嵌形态(AppLayout 内):隐藏页头。 */
  embedded?: boolean
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const isAuthenticated = computed(() => authStore.isAuthenticated)

/** 计费类型筛选的展示顺序。 */
const BILLING_ORDER: string[] = [BILLING_MODE_TOKEN, BILLING_MODE_PER_REQUEST, BILLING_MODE_IMAGE, BILLING_MODE_VIDEO]

const filters = ref<PlazaFilters>({ ...EMPTY_FILTERS })
const sort = ref<PlazaSort>('default')
const view = ref<PlazaView>('card')
const selectedKey = ref<string | null>(null)
const mobileFiltersOpen = ref(false)

const baseUrl = computed(() => toApiRoot(appStore.cachedPublicSettings?.api_base_url || window.location.origin))

const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  if (!md) return ''
  return DOMPurify.sanitize(marked.parse(md) as string)
})

const groups = computed(() => props.response?.groups ?? [])
const catalog = computed(() => buildPlazaCatalog(groups.value))
const filtered = computed(() => filterCatalog(catalog.value, filters.value))
const sorted = computed(() => sortCatalog(filtered.value, sort.value, filters.value))
const selectedEntry = computed<PlazaCatalogEntry | null>(
  () => catalog.value.find((e) => e.key === selectedKey.value) ?? null
)

/**
 * 首次拿到数据时编排一次：卡片依次划线、盖章（按 --i 错开，最多错开这么多张）。
 * 之后筛选出来的新卡片不再重播，动效只回应用户的操作。
 */
const INTRO_STAGGER_CAP = 11
const INTRO_MS = INTRO_STAGGER_CAP * 60 + 1200
const introActive = ref(false)
let introTimer: ReturnType<typeof setTimeout> | null = null
let introPlayed = false
watch(
  () => catalog.value.length > 0,
  (ready) => {
    if (!ready || introPlayed) return
    introPlayed = true
    introActive.value = true
    introTimer = setTimeout(() => {
      introActive.value = false
      introTimer = null
    }, INTRO_MS)
  },
  { immediate: true }
)
onBeforeUnmount(() => {
  if (introTimer) clearTimeout(introTimer)
})

const sidebarProps = computed(() => {
  const offers = catalog.value.flatMap((e) => e.offers)
  const modes = new Set(offers.map((o) => billingMode(o.model) as string))
  return {
    filters: filters.value,
    counts: facetCounts(catalog.value, filters.value),
    platforms: [...new Set(offers.map((o) => o.model.platform))].sort(),
    // 专属倍率会改变生效值，不能只依赖后端按默认倍率的排序。
    groups: [...groups.value]
      .filter((g) => g.models.length > 0)
      .sort((a, b) => groupRate(a) - groupRate(b) || a.name.localeCompare(b.name)),
    billingModes: BILLING_ORDER.filter((mode) => modes.has(mode))
  }
})

const activeFilterCount = computed(
  () =>
    [filters.value.platform, filters.value.groupId, filters.value.billing].filter((v) => v !== FILTER_ALL).length
)

function patchFilters(patch: Partial<PlazaFilters>) {
  filters.value = { ...filters.value, ...patch }
}

function resetFilters() {
  filters.value = { ...EMPTY_FILTERS, search: filters.value.search }
}

function clearAll() {
  filters.value = { ...EMPTY_FILTERS }
}

function openEntry(entry: PlazaCatalogEntry) {
  selectedKey.value = entry.key
}
</script>

<style scoped>
@media (prefers-reduced-motion: no-preference) {
  .plaza-grid-move {
    transition: transform 420ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .plaza-grid-enter-active {
    transition:
      opacity 260ms ease,
      transform 360ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .plaza-grid-enter-from {
    opacity: 0;
    transform: translateY(10px) scale(0.97);
  }

  .plaza-fade-enter-active,
  .plaza-fade-leave-active {
    transition: opacity 160ms ease;
  }

  .plaza-fade-enter-from,
  .plaza-fade-leave-to {
    opacity: 0;
  }
}

.plaza-description {
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.plaza-description :deep(h1),
.plaza-description :deep(h2),
.plaza-description :deep(h3) {
  @apply mb-2 mt-3 font-semibold text-gray-900 first:mt-0 dark:text-white;
}

.plaza-description :deep(p) {
  @apply mb-2 text-gray-700 last:mb-0 dark:text-dark-200;
}

.plaza-description :deep(a) {
  @apply text-primary-600 underline underline-offset-4 hover:text-primary-700 dark:text-primary-300;
}

.plaza-description :deep(ul) {
  @apply mb-2 list-disc pl-5;
}

.plaza-description :deep(ol) {
  @apply mb-2 list-decimal pl-5;
}

.plaza-description :deep(li) {
  @apply mb-0.5 text-gray-700 dark:text-dark-200;
}

.plaza-description :deep(code) {
  @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs dark:bg-dark-800;
}

.plaza-description :deep(blockquote) {
  @apply my-2 border-l-4 border-gray-300 pl-3 text-gray-600 dark:border-dark-600 dark:text-dark-300;
}
</style>
