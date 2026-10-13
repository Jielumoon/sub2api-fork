<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { Account } from '@/types'

const props = defineProps<{ account: Account; now: number; loading?: boolean; globalProbeEnabled?: boolean }>()
defineEmits<{ probe: [] }>()
const { t, n, locale } = useI18n()
const kind = computed(() => props.account.extra?.upstream_balance_probe_type ?? 'disabled')
const enabled = computed(() => props.account.type === 'apikey' && kind.value !== 'disabled')
const snapshot = computed(() => {
  const value = props.account.extra?.upstream_balance_probe
  return value?.type === kind.value ? value : undefined
})
const unavailable = computed(() => ['unsupported', 'unavailable'].includes(snapshot.value?.status ?? ''))
const amount = computed(() => {
  const data = snapshot.value?.data
  if (unavailable.value || !data) return '—'
  if (data.unlimited) return t('admin.accounts.upstreamBalance.unlimited')
  return typeof data.remaining === 'number' && Number.isFinite(data.remaining)
    ? `$${n(data.remaining, { minimumFractionDigits: 2, maximumFractionDigits: 4 })}` : '—'
})
const stale = computed(() => snapshot.value?.status !== 'ok' || !snapshot.value?.fresh_until || !Number.isFinite(Date.parse(snapshot.value.fresh_until)) || Date.parse(snapshot.value.fresh_until) <= props.now)
const timeLabel = computed(() => {
  const value = snapshot.value?.received_at
  return value && Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleString(locale.value) : ''
})
const detail = computed(() => {
  const data = snapshot.value?.data
  return [
    typeof data?.limit === 'number' ? `${t('admin.accounts.upstreamBalance.limit')}: $${n(data.limit)}` : '',
    typeof data?.used === 'number' ? `${t('admin.accounts.upstreamBalance.used')}: $${n(data.used)}` : '',
    data?.reset_at ? `${t('admin.accounts.upstreamBalance.resetsAt')}: ${data.reset_at}` : '',
    data?.expires_at ? `${t('admin.accounts.upstreamBalance.expiresAt')}: ${data.expires_at}` : ''
  ].filter(Boolean).join('\n')
})
</script>

<template>
  <span v-if="!enabled" class="text-gray-400">—</span>
  <div v-else class="min-w-32 space-y-1" :aria-busy="loading" :title="detail">
    <div class="flex items-center gap-2">
      <span class="font-mono text-sm tabular-nums" data-testid="balance-amount">{{ amount }}</span>
      <button type="button" class="rounded p-1 text-gray-500 hover:text-primary-600 disabled:opacity-50" :disabled="loading" :aria-label="t('admin.accounts.upstreamBalance.refresh')" @click.stop="$emit('probe')">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
      </button>
    </div>
    <div class="text-xs text-gray-500">{{ t(`admin.accounts.upstreamBalance.types.${kind}`) }}</div>
    <div v-if="snapshot?.data?.upstream_status" class="text-xs text-gray-500">{{ t(`admin.accounts.upstreamBalance.upstreamStatus.${snapshot.data.upstream_status}`) }}</div>
    <div v-if="snapshot?.status && snapshot.status !== 'ok'" class="text-xs text-amber-600" role="status">{{ t(`admin.accounts.upstreamBalance.status.${snapshot.status}`) }}</div>
    <div v-if="timeLabel" class="text-xs text-gray-400">
      {{ t(stale ? 'admin.accounts.upstreamBalance.lastSuccess' : 'admin.accounts.upstreamBalance.updated') }} {{ timeLabel }}
    </div>
    <span v-else-if="!snapshot" class="text-xs text-gray-400">{{ t('admin.accounts.upstreamBalance.pending') }}</span>
    <div v-if="globalProbeEnabled === false" class="text-xs text-gray-400">{{ t('admin.accounts.upstreamBalance.paused') }}</div>
  </div>
</template>
