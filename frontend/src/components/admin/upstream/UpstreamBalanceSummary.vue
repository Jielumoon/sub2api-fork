<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import type { UpstreamProbeSnapshot, UpstreamUsageItem, UpstreamUsageItemKind } from '@/types'

const props = defineProps<{ snapshot?: UpstreamProbeSnapshot | null; now: number }>()
const { t, n } = useI18n()

// 与后端 upstreamUsageItemKinds 顺序一致
const ITEM_KINDS: UpstreamUsageItemKind[] = [
  'wallet', 'key_total', 'key_5h', 'key_1d', 'key_7d',
  'subscription_daily', 'subscription_weekly', 'subscription_monthly', 'subscription_remaining'
]
const KNOWN_UPSTREAM_STATUS = ['active', 'disabled', 'expired', 'quota_exhausted']

const isFiniteNumber = (value: unknown): value is number => typeof value === 'number' && Number.isFinite(value)
const usd = (value: number) => {
  const text = n(Math.abs(value), { minimumFractionDigits: 2, maximumFractionDigits: 4 })
  return value < 0 ? `-$${text}` : `$${text}`
}
const parseTime = (value?: string) => (value ? Date.parse(value) : NaN)

const status = computed(() => props.snapshot?.status)
// unsupported / unavailable 时后端可能残留旧数据，与当前响应不符，一律不显示金额
const hideAmounts = computed(() => status.value === 'unsupported' || status.value === 'unavailable')
const data = computed(() => (hideAmounts.value ? undefined : props.snapshot?.data))

// 只列出上游实际返回的额度项；缺失项不显示，绝不当成 0
const items = computed(() => {
  const source = data.value?.items
  if (!source) return []
  return ITEM_KINDS.flatMap((kind) => {
    const item = source[kind]
    return item ? [{ kind, item }] : []
  })
})

const amount = (item: UpstreamUsageItem) => {
  if (item.unlimited) return t('admin.upstreams.balance.unlimited')
  return isFiniteNumber(item.remaining) ? usd(item.remaining) : '—'
}
const detail = (item: UpstreamUsageItem) => [
  isFiniteNumber(item.limit) ? `${t('admin.upstreams.balance.limit')} ${usd(item.limit)}` : '',
  isFiniteNumber(item.used) ? `${t('admin.upstreams.balance.used')} ${usd(item.used)}` : '',
  item.reset_at ? `${t('admin.upstreams.balance.resetsAt')} ${formatDateTime(item.reset_at)}` : ''
].filter(Boolean).join(' · ')

// 非 ok 或超过 fresh_until 都视为陈旧：失败时保留的是上一次成功值
const stale = computed(() => {
  const freshUntil = parseTime(props.snapshot?.fresh_until)
  return status.value !== 'ok' || !Number.isFinite(freshUntil) || freshUntil <= props.now
})
const receivedLabel = computed(() => {
  const value = props.snapshot?.received_at
  return Number.isFinite(parseTime(value)) ? formatDateTime(value) : ''
})
const upstreamStatusLabel = computed(() => {
  const value = data.value?.upstream_status
  if (!value) return ''
  return KNOWN_UPSTREAM_STATUS.includes(value) ? t(`admin.upstreams.balance.upstreamStatus.${value}`) : value
})
</script>

<template>
  <div class="min-w-40 space-y-1 text-sm" data-testid="upstream-balance">
    <span v-if="!snapshot" class="text-xs text-gray-400">{{ t('admin.upstreams.balance.pending') }}</span>
    <template v-else>
      <div
        v-if="status !== 'ok'"
        role="status"
        :class="['text-xs', status === 'failed' ? 'text-red-600 dark:text-red-400' : 'text-amber-600 dark:text-amber-400']"
        data-testid="upstream-status"
      >
        {{ t(`admin.upstreams.balance.status.${status}`) }}
        <code v-if="snapshot.last_error" class="ml-1 text-[11px] text-gray-500">{{ snapshot.last_error }}</code>
      </div>

      <dl v-if="items.length" class="space-y-1">
        <div v-for="{ kind, item } in items" :key="kind" :data-testid="`upstream-item-${kind}`">
          <div class="flex items-baseline justify-between gap-3">
            <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t(`admin.upstreams.balance.items.${kind}`) }}</dt>
            <dd
              :class="['font-mono tabular-nums', stale ? 'text-gray-400 dark:text-gray-500' : 'text-gray-900 dark:text-white']"
              data-testid="upstream-amount"
            >
              {{ amount(item) }}
            </dd>
          </div>
          <div v-if="detail(item)" class="text-[11px] text-gray-400">{{ detail(item) }}</div>
        </div>
      </dl>

      <div v-if="upstreamStatusLabel" class="text-xs text-gray-500">{{ upstreamStatusLabel }}</div>
      <div v-if="data?.expires_at" class="text-xs text-gray-500">
        {{ t('admin.upstreams.balance.expiresAt') }} {{ formatDateTime(data.expires_at) }}
      </div>
      <div v-if="items.length && receivedLabel" class="text-xs" :class="stale ? 'text-amber-600 dark:text-amber-400' : 'text-gray-400'" data-testid="upstream-received">
        {{ t(stale ? 'admin.upstreams.balance.lastSuccess' : 'admin.upstreams.balance.updated') }} {{ receivedLabel }}
      </div>
    </template>
  </div>
</template>
