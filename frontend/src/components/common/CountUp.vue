<template>
  <span>{{ text }}</span>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { TransitionPresets, usePreferredReducedMotion, useTransition } from '@vueuse/core'

const props = withDefaults(
  defineProps<{
    value: number | null | undefined
    format?: (value: number) => string
    duration?: number
  }>(),
  {
    format: (value: number) => String(value),
    duration: 800
  }
)

const target = computed(() => {
  const value = Number(props.value)
  return Number.isFinite(value) ? value : 0
})

// 从 0 起跳，挂载后再交给过渡；之后数据刷新时从旧值滚到新值。
const source = ref(0)
const reducedMotion = usePreferredReducedMotion()
const output = useTransition(source, {
  duration: props.duration,
  transition: TransitionPresets.easeOutCubic,
  disabled: computed(() => reducedMotion.value === 'reduce')
})

onMounted(() => {
  source.value = target.value
})
watch(target, (value) => {
  source.value = value
})

// 中间帧按目标值的小数位取整，避免整数指标滚出 2.37 这类过渡值。
const decimals = computed(() => {
  if (Number.isInteger(target.value)) return 0
  return Math.min(String(target.value).split('.')[1]?.length ?? 0, 4)
})

const text = computed(() => {
  if (output.value === target.value) return props.format(target.value)
  const factor = 10 ** decimals.value
  return props.format(Math.round(output.value * factor) / factor)
})
</script>
