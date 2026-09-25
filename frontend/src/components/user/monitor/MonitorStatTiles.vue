<template>
  <div class="grid grid-cols-3 gap-2">
    <div
      v-for="item in items"
      :key="item.label"
      class="min-w-0 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-900/60"
    >
      <div class="truncate text-[11px] font-medium text-gray-500 dark:text-dark-400" :title="item.label">
        {{ item.label }}
      </div>
      <div
        class="mt-1 truncate font-mono text-lg font-semibold tabular-nums"
        :class="item.color ? '' : item.toneClass || 'text-gray-900 dark:text-gray-100'"
        :style="item.color ? { color: item.color } : undefined"
      >
        <!-- null / NaN 一律显示 -，不交给 CountUp 兜成 0 -->
        <CountUp v-if="Number.isFinite(item.value)" :value="item.value" :format="item.format" />
        <span v-else>-</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import CountUp from '@/components/common/CountUp.vue'

defineProps<{
  items: Array<{
    label: string
    value: number | null
    format: (value: number) => string
    toneClass?: string
    /** 连续色（如按可用率的红→绿渐变），优先于 toneClass */
    color?: string
  }>
}>()
</script>
