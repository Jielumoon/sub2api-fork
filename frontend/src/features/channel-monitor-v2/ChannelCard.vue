<template>
  <!-- V2 卡片不可点击，只做描边反馈，不用会暗示可点的上浮效果。 -->
  <article class="card flex flex-col p-5 hover:border-gray-300 dark:hover:border-dark-600">
    <header class="flex items-start gap-3">
      <span
        class="grid h-9 w-9 flex-shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10"
        :class="[providerGradient(row.platform), providerTint(row.platform)]"
      >
        <ProviderIcon :provider="row.platform" :size="20" />
      </span>
      <div class="min-w-0 flex-1">
        <h3 class="truncate text-base font-semibold text-gray-900 dark:text-gray-100" :title="name">{{ name }}</h3>
        <div class="mt-1 flex min-w-0 flex-wrap items-center gap-1.5">
          <span
            class="inline-flex flex-shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium"
            :class="providerBadgeClass(row.platform)"
          >
            {{ providerLabel(row.platform) }}
          </span>
          <span
            v-if="row.group_name && row.model"
            class="truncate font-mono text-[11px] text-gray-500 dark:text-gray-400"
          >
            {{ row.model }}
          </span>
          <span
            v-if="rate != null"
            class="inline-flex flex-shrink-0 items-center rounded-md bg-gray-100 px-1.5 py-0.5 font-mono text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
          >
            {{ t('channelMonitorV2.cards.rate', { rate: formatRate(rate) }) }}
          </span>
        </div>
      </div>
      <span
        class="inline-flex flex-shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold"
        :class="statusBadgeClass(AS_MONITOR_STATUS[healthState])"
      >
        <span class="relative flex h-1.5 w-1.5">
          <span
            v-if="healthState === 'healthy'"
            class="absolute inline-flex h-full w-full rounded-full opacity-60 motion-safe:animate-ping"
            :class="statusDotClass('operational')"
          ></span>
          <span
            class="relative inline-flex h-1.5 w-1.5 rounded-full"
            :class="statusDotClass(AS_MONITOR_STATUS[healthState])"
          ></span>
        </span>
        {{ t(`channelMonitorV2.cards.health.${healthState}`) }}
      </span>
    </header>

    <MonitorStatTiles class="mt-4" :items="tiles" />

    <MonitorStatusStrip
      :blocks="blocks"
      :label="t('channelMonitorV2.cards.recent', { n: slots.length })"
      :hint="countdownSeconds == null ? undefined : t('monitorCommon.nextUpdateIn', { n: countdownSeconds })"
    />
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { HealthState, MonitorMatrixRow } from '@/api/channelMonitorV2'
import type { MonitorStatus } from '@/api/channelMonitor'
import {
  providerGradient,
  providerTint,
  useChannelMonitorFormat,
} from '@/composables/useChannelMonitorFormat'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import MonitorStatTiles from '@/components/user/monitor/MonitorStatTiles.vue'
import MonitorStatusStrip from '@/components/user/monitor/MonitorStatusStrip.vue'
import type { BucketSlot } from './monitorBuckets'
import {
  formatMonitorMs,
  formatMonitorPercent,
  healthScoreClass,
  ttftDisplayState,
  type HealthDisplayMode,
} from './monitorFormat'
import './healthColors.css'

type BadgeState = 'healthy' | 'warning' | 'critical' | 'unknown'

// 徽章与圆点复用 V1 的状态配色：健康≈正常、需关注≈降级、异常≈失败，无流量为中性灰。
const AS_MONITOR_STATUS: Record<BadgeState, MonitorStatus | ''> = {
  healthy: 'operational',
  warning: 'degraded',
  critical: 'failed',
  unknown: '',
}

const TONE: Partial<Record<HealthState, string>> = {
  healthy: 'text-emerald-600 dark:text-emerald-400',
  warning: 'text-amber-600 dark:text-amber-400',
  critical: 'text-red-600 dark:text-red-400',
}

const props = defineProps<{
  row: MonitorMatrixRow
  slots: BucketSlot[]
  bucketSeconds: number
  healthMode: HealthDisplayMode
  rate?: number
  countdownSeconds: number | null
}>()

const { t, locale } = useI18n()
const { providerLabel, providerBadgeClass, statusBadgeClass, statusDotClass } = useChannelMonitorFormat()

const name = computed(() => props.row.group_name || props.row.model || providerLabel(props.row.platform))
// 后端只返回有流量的桶；整个窗口都没有桶时指标一律显示 -，避免把 0 错误率显示成 100%。
const hasTraffic = computed(() => (props.row.buckets?.length ?? 0) > 0)
const healthState = computed<BadgeState>(() => {
  const overall = hasTraffic.value ? props.row.health.overall : 'unknown'
  return overall in AS_MONITOR_STATUS ? (overall as BadgeState) : 'unknown'
})

const tiles = computed(() => {
  const { metrics, health } = props.row
  const traffic = hasTraffic.value
  return [
    {
      label: t('channelMonitorV2.metrics.cacheRate'),
      value: traffic ? metrics.cache_rate : null,
      format: formatMonitorPercent,
    },
    {
      label: t('channelMonitorV2.metrics.successRate'),
      value: traffic ? 1 - metrics.error_rate : null,
      format: formatMonitorPercent,
      toneClass: traffic ? TONE[health.error_rate] : undefined,
    },
    {
      label: t('channelMonitorV2.metrics.ttft'),
      value: traffic ? metrics.ttft.p50_ms ?? null : null,
      format: formatMonitorMs,
      toneClass: traffic ? TONE[ttftDisplayState(health.ttft, metrics.ttft) ?? 'unknown'] : undefined,
    },
  ]
})

const slotFormatter = computed(
  () => new Intl.DateTimeFormat(locale.value || undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
)

function formatSlotRange(start: string): string {
  const from = new Date(start)
  const to = new Date(from.getTime() + props.bucketSeconds * 1000)
  return `${slotFormatter.value.format(from)} – ${slotFormatter.value.format(to)}`
}

const blocks = computed(() =>
  props.slots.map((slot) => {
    const range = formatSlotRange(slot.start)
    if (!slot.bucket) {
      return {
        colorClass: 'health-unknown',
        lines: [range, t('channelMonitorV2.matrix.noTraffic')],
        state: t('channelMonitorV2.cards.health.unknown'),
      }
    }
    const { metrics, health } = slot.bucket
    const state: BadgeState = health.overall in AS_MONITOR_STATUS ? (health.overall as BadgeState) : 'unknown'
    return {
      colorClass: healthScoreClass(health, props.healthMode, metrics.request_count),
      state: t(`channelMonitorV2.cards.health.${state}`),
      lines: [
        range,
        t('channelMonitorV2.metrics.successRateValue', { value: formatMonitorPercent(1 - metrics.error_rate) }),
        t('channelMonitorV2.metrics.cacheRateValue', { value: formatMonitorPercent(metrics.cache_rate) }),
        t('channelMonitorV2.metrics.ttftValue', { value: formatMonitorMs(metrics.ttft.p50_ms) }),
      ],
    }
  })
)

function formatRate(rate: number): string {
  return String(Number(rate.toFixed(4)))
}
</script>
