<template>
  <div>
    <div
      v-if="loading && items.length === 0"
      class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
    >
      <div v-for="i in 6" :key="i" class="card animate-pulse p-5">
        <div class="flex items-start gap-3">
          <div class="h-9 w-9 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
            <div class="h-3 w-1/2 rounded bg-gray-200 dark:bg-dark-700"></div>
          </div>
          <div class="h-6 w-16 rounded-full bg-gray-200 dark:bg-dark-700"></div>
        </div>
        <div class="mt-4 grid grid-cols-3 gap-2">
          <div v-for="j in 3" :key="j" class="h-14 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
        </div>
        <div class="mt-6 h-5 w-full rounded bg-gray-100 dark:bg-dark-900/40"></div>
      </div>
    </div>

    <EmptyState
      v-else-if="items.length === 0"
      :title="t('channelStatus.empty.title')"
      :description="t('channelStatus.empty.description')"
    />

    <div v-else class="space-y-8">
      <MonitorProviderSection
        v-for="section in sections"
        :key="section.provider"
        :provider="section.provider"
        :count="section.items.length"
      >
        <MonitorCard
          v-for="item in section.items"
          :key="item.id"
          :item="item"
          :window="window"
          :availability-value="resolveAvailability(item)"
          :countdown-seconds="countdownSeconds"
          @click="emit('cardClick', item)"
        />
      </MonitorProviderSection>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView, UserMonitorDetail } from '@/api/channelMonitor'
import EmptyState from '@/components/common/EmptyState.vue'
import MonitorCard from './MonitorCard.vue'
import MonitorProviderSection from './MonitorProviderSection.vue'
import { groupBy } from '@/utils/groupBy'

const props = defineProps<{
  items: UserMonitorView[]
  window: '7d' | '15d' | '30d'
  countdownSeconds: number
  loading: boolean
  detailCache: Record<number, UserMonitorDetail>
}>()

const emit = defineEmits<{
  (e: 'cardClick', item: UserMonitorView): void
}>()

const { t } = useI18n()

// 按供应商分组，保持后端返回的先后顺序。
const sections = computed(() =>
  groupBy(props.items, (item) => item.provider).map(({ key, items }) => ({ provider: key, items }))
)

function resolveAvailability(item: UserMonitorView): number | null {
  if (props.window === '7d') {
    return item.availability_7d ?? null
  }
  const detail = props.detailCache[item.id]
  if (!detail) return null
  const primary = detail.models.find(m => m.model === item.primary_model)
  if (!primary) return null
  return props.window === '15d' ? primary.availability_15d ?? null : primary.availability_30d ?? null
}
</script>
