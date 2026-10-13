<script setup lang="ts">
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import type { UpstreamBalanceProbeType } from '@/types'

const model = defineModel<UpstreamBalanceProbeType>({ required: true })
const { t } = useI18n()
const id = useId()
const kinds: UpstreamBalanceProbeType[] = [
  'disabled', 'wallet', 'key_total', 'key_5h', 'key_1d', 'key_7d',
  'subscription_daily', 'subscription_weekly', 'subscription_monthly', 'subscription_remaining'
]
const options = computed(() => kinds.map(value => ({ value, label: t(`admin.accounts.upstreamBalance.types.${value}`) })))
</script>

<template>
  <div class="space-y-2" data-testid="upstream-balance-probe-select">
    <label :for="id" class="input-label">{{ t('admin.accounts.upstreamBalance.title') }}</label>
    <Select :id="id" v-model="model" :options="options" :aria-label="t('admin.accounts.upstreamBalance.title')" :aria-describedby="`${id}-hint`" />
    <p :id="`${id}-hint`" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.hint') }}</p>
  </div>
</template>
