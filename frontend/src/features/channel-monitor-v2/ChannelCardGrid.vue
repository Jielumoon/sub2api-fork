<template>
  <div class="space-y-8">
    <EmptyState
      v-if="sections.length === 0"
      :title="t('channelMonitorV2.empty.title')"
      :description="t('channelMonitorV2.empty.description')"
    />
    <MonitorProviderSection
      v-for="section in sections"
      :key="section.platform"
      :provider="section.platform"
      :count="section.entries.length"
    >
      <ChannelCard
        v-for="{ row, slots } in section.entries"
        :key="rowKey(row)"
        :row="row"
        :slots="slots"
        :bucket-seconds="coverage.bucket_seconds"
        :health-mode="healthMode"
        :rate="row.group_id != null ? rates[row.group_id] : undefined"
        :countdown-seconds="countdownSeconds"
      />
    </MonitorProviderSection>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorCoverage, MonitorMatrixRow } from '@/api/channelMonitorV2'
import EmptyState from '@/components/common/EmptyState.vue'
import MonitorProviderSection from '@/components/user/monitor/MonitorProviderSection.vue'
import ChannelCard from './ChannelCard.vue'
import { groupBy } from '@/utils/groupBy'
import { alignRows, bucketStartsForCoverage } from './monitorBuckets'
import type { HealthDisplayMode } from './monitorFormat'

const props = defineProps<{
  rows: MonitorMatrixRow[]
  coverage: MonitorCoverage
  healthMode: HealthDisplayMode
  /** group_id → 生效倍率（专属倍率优先）；缺失时卡片不显示倍率标签。 */
  rates: Record<number, number>
  countdownSeconds: number | null
}>()

const { t } = useI18n()

// 时间槽只随数据和 coverage 变化；倒计时每秒刷新不会触发重新对位和格式化。
const sections = computed(() =>
  groupBy(alignRows(props.rows, bucketStartsForCoverage(props.coverage)), (entry) => entry.row.platform)
    .map(({ key, items }) => ({ platform: key, entries: items }))
)

function rowKey(row: MonitorMatrixRow): string {
  return [row.platform, row.group_id || 0, row.model || ''].join(':')
}
</script>
