<template>
  <button type="button" class="card card-hover flex w-full flex-col p-5 text-left" @click="emit('click')">
    <!-- Header: icon + name/model + status chip -->
    <header class="flex items-start gap-3">
      <span
        class="grid h-9 w-9 flex-shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10"
        :class="[providerGradient(item.provider), providerTint(item.provider)]"
      >
        <ProviderIcon :provider="item.provider" :size="20" />
      </span>
      <div class="min-w-0 flex-1">
        <div class="truncate text-base font-semibold text-gray-900 dark:text-gray-100" :title="item.name">
          {{ item.name }}
        </div>
        <div class="mt-1 flex min-w-0 items-center gap-1.5">
          <span
            class="inline-flex flex-shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium"
            :class="providerBadgeClass(item.provider)"
          >
            {{ providerLabel(item.provider) }}
          </span>
          <!-- 纯配额模式主模型是占位符 "quota"，展示层替换为本地化「配额」标签 -->
          <span class="truncate font-mono text-xs text-gray-500 dark:text-gray-400">
            {{ formatMonitorModel(item.primary_model) }}
          </span>
          <span
            v-if="item.group_name"
            class="inline-flex flex-shrink-0 items-center rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
          >
            {{ item.group_name }}
          </span>
          <span v-if="extraModelsCountLabel" class="flex-shrink-0 text-[10px] text-gray-400 dark:text-dark-400">
            {{ extraModelsCountLabel }}
          </span>
        </div>
      </div>
      <span
        class="inline-flex flex-shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold"
        :class="statusBadgeClass(item.primary_status)"
      >
        <span class="relative flex h-1.5 w-1.5">
          <span
            v-if="item.primary_status === 'operational'"
            class="absolute inline-flex h-full w-full rounded-full bg-emerald-500 opacity-60 motion-safe:animate-ping"
          ></span>
          <span class="relative inline-flex h-1.5 w-1.5 rounded-full" :class="statusDotClass(item.primary_status)"></span>
        </span>
        {{ statusLabel(item.primary_status) }}
      </span>
    </header>

    <MonitorStatTiles class="mt-4" :items="tiles" />

    <!-- 配额模式：最新用量/余额快照（服务端已按系统开关剥离，此处 flag 为纵深防御） -->
    <MonitorQuotaView v-if="quotaVisible" :snapshot="item.latest_quota" class="mt-3" />

    <MonitorStatusStrip
      :blocks="blocks"
      :label="t('monitorCommon.history60pts', { n: TIMELINE_LENGTH })"
      :hint="t('monitorCommon.nextUpdateIn', { n: countdownSeconds })"
    />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorStatus, UserMonitorView } from '@/api/channelMonitor'
import {
  useChannelMonitorFormat,
  hslForPct,
  providerGradient,
  providerTint,
} from '@/composables/useChannelMonitorFormat'
import { formatMonitorMs } from '@/features/channel-monitor-v2/monitorFormat'
import { isChannelMonitorQuotaVisible } from '@/utils/featureFlags'
import ProviderIcon from './ProviderIcon.vue'
import MonitorStatTiles from './MonitorStatTiles.vue'
import MonitorStatusStrip from './MonitorStatusStrip.vue'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'

/** 卡片只展示最近 30 次检测，保证色块宽度；完整历史在详情弹窗里看。 */
const TIMELINE_LENGTH = 30

const STATUS_BLOCK: Record<string, string> = {
  operational: 'bg-emerald-500',
  degraded: 'bg-amber-500',
  failed: 'bg-red-500',
  error: 'bg-red-500',
}

const props = defineProps<{
  item: UserMonitorView
  window: '7d' | '15d' | '30d'
  availabilityValue: number | null
  countdownSeconds: number
}>()

const emit = defineEmits<{
  (e: 'click'): void
}>()

const { t } = useI18n()
const {
  statusLabel,
  statusBadgeClass,
  statusDotClass,
  providerLabel,
  providerBadgeClass,
  formatLatency,
  formatMonitorModel,
  formatPercent,
  formatRelativeTime,
} = useChannelMonitorFormat()

const quotaVisible = computed(
  () => isChannelMonitorQuotaVisible() && !!props.item.latest_quota
)

const availabilityLabel = computed(() => {
  const win = t(`channelStatus.windowTab.${props.window}`)
  return `${t('monitorCommon.availabilityPrefix')} · ${win}`
})

const extraModelsCountLabel = computed(() => {
  const count = props.item.extra_models?.length ?? 0
  if (count === 0) return undefined
  return t('monitorCommon.extraModelsCount', { n: count })
})

const tiles = computed(() => [
  {
    label: availabilityLabel.value,
    value: props.availabilityValue,
    format: formatPercent,
    // 按可用率数值本身做红 → 绿渐变，而不是按最新一次检测状态。
    color: hslForPct(props.availabilityValue),
  },
  {
    label: t('monitorCommon.dialogLatency'),
    value: props.item.primary_latency_ms,
    format: formatMonitorMs,
  },
  {
    label: t('monitorCommon.endpointPing'),
    value: props.item.primary_ping_latency_ms,
    format: formatMonitorMs,
  },
])

function pointLines(status: MonitorStatus, latencyMs: number | null, pingMs: number | null, checkedAt: string) {
  const lines = [formatRelativeTime(checkedAt), statusLabel(status)]
  if (latencyMs != null) lines.push(`${t('monitorCommon.dialogLatency')} ${formatLatency(latencyMs)}ms`)
  if (pingMs != null) lines.push(`${t('monitorCommon.endpointPing')} ${formatLatency(pingMs)}ms`)
  return lines
}

const blocks = computed(() => {
  // 时间线是新 → 旧，取最近 N 个后反转，让最右侧代表「现在」；不足 N 个时左侧补空块。
  const real = (props.item.timeline ?? []).slice(0, TIMELINE_LENGTH).reverse()
  const padding = Array.from({ length: Math.max(0, TIMELINE_LENGTH - real.length) }, () => ({
    colorClass: 'bg-gray-200 dark:bg-dark-700',
    lines: [] as string[],
  }))
  return [
    ...padding,
    ...real.map((point) => ({
      colorClass: STATUS_BLOCK[point.status] ?? 'bg-gray-300 dark:bg-dark-600',
      lines: pointLines(point.status, point.latency_ms, point.ping_latency_ms, point.checked_at),
      state: statusLabel(point.status),
    })),
  ]
})
</script>
