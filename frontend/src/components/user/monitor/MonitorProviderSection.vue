<template>
  <section class="space-y-3">
    <header class="flex items-center gap-2.5">
      <span
        class="grid h-8 w-8 flex-shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10"
        :class="[providerGradient(provider), providerTint(provider)]"
      >
        <ProviderIcon :provider="provider" :size="18" />
      </span>
      <h2 class="font-serif text-lg font-bold text-gray-900 dark:text-gray-100">
        {{ providerLabel(provider) }}
      </h2>
      <span
        class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium tabular-nums text-gray-600 dark:bg-dark-700 dark:text-gray-300"
      >
        {{ count }}
      </span>
    </header>
    <div class="stagger grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  providerGradient,
  providerTint,
  useChannelMonitorFormat,
} from '@/composables/useChannelMonitorFormat'
import ProviderIcon from './ProviderIcon.vue'

defineProps<{
  provider: string
  count: number
}>()

const { providerLabel } = useChannelMonitorFormat()
</script>
