<template>
  <div class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700/60">
    <div
      class="mb-2 flex justify-between text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-dark-400"
    >
      <span>{{ label }}</span>
      <span v-if="hint" class="tabular-nums">{{ hint }}</span>
    </div>

    <div class="relative" @pointerleave="onLeave">
      <div class="flex h-5 w-full gap-[3px]" role="img" :aria-label="ariaLabel">
        <div
          v-for="(block, index) in blocks"
          :key="index"
          class="strip-block min-w-0 flex-1 rounded-[3px] hover:scale-y-125"
          :class="[block.colorClass, active === index ? 'brightness-110' : '']"
          :style="{ '--i': index }"
          @pointerenter="onEnter(index, $event)"
        ></div>
      </div>

      <Transition name="strip-tip">
        <div
          v-if="activeBlock && activeBlock.lines.length"
          class="pointer-events-none absolute bottom-full z-20 mb-2 w-max max-w-[18rem] rounded-lg bg-gray-900 px-2.5 py-1.5 text-[11px] leading-relaxed text-white shadow-lg dark:bg-dark-700"
          :style="tipStyle"
        >
          <div v-for="(line, index) in activeBlock.lines" :key="index" class="whitespace-nowrap">{{ line }}</div>
        </div>
      </Transition>
    </div>

    <div class="mt-1 flex justify-between text-[9px] uppercase tracking-widest text-gray-400 dark:text-dark-400">
      <span>{{ t('monitorCommon.past') }}</span>
      <span>{{ t('monitorCommon.now') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  /** state：该块的状态文案，用于给读屏器汇总分布；补位的空块不传 */
  blocks: Array<{ colorClass: string; lines: string[]; state?: string }>
  label: string
  hint?: string
}>()

const { t } = useI18n()
const active = ref<number | null>(null)
let touchTimer: number | undefined

// 触屏点一下也能看明细：非鼠标指针不会触发悬停离开，改为 2.5 秒后自动收起。
function onEnter(index: number, event: PointerEvent) {
  active.value = index
  window.clearTimeout(touchTimer)
  if (event.pointerType !== 'mouse') {
    touchTimer = window.setTimeout(() => (active.value = null), 2500)
  }
}

function onLeave(event: PointerEvent) {
  if (event.pointerType === 'mouse') active.value = null
}

onBeforeUnmount(() => window.clearTimeout(touchTimer))

// 读屏器拿不到逐块提示，至少读出状态分布，例如「近 30 次记录 (正常 25, 降级 3, 失败 2)」。
const ariaLabel = computed(() => {
  const counts = new Map<string, number>()
  for (const block of props.blocks) {
    if (block.state) counts.set(block.state, (counts.get(block.state) ?? 0) + 1)
  }
  const parts = [...counts].map(([state, count]) => `${state} ${count}`)
  return parts.length ? `${props.label} (${parts.join(', ')})` : props.label
})

const activeBlock = computed(() => (active.value == null ? null : props.blocks[active.value] ?? null))

// 提示框靠近两端时改为左 / 右对齐，避免超出卡片太多。
const tipStyle = computed(() => {
  const pct = active.value == null ? 50 : ((active.value + 0.5) / Math.max(1, props.blocks.length)) * 100
  const shift = pct < 30 ? '-12%' : pct > 70 ? '-88%' : '-50%'
  return { left: `${pct}%`, transform: `translateX(${shift})` }
})
</script>

<style scoped>
.strip-tip-enter-active,
.strip-tip-leave-active {
  transition: opacity 0.15s ease, margin-bottom 0.15s ease;
}

.strip-tip-enter-from,
.strip-tip-leave-to {
  opacity: 0;
  margin-bottom: 0.25rem;
}

/* 刷新时颜色慢慢过渡，悬停放大要跟手。 */
.strip-block {
  transition:
    background-color 0.5s ease,
    transform 0.15s ease;
}

/* 色块从左到右依次点亮；数据刷新不会重放，只做颜色过渡。 */
@keyframes strip-block-in {
  from {
    opacity: 0;
    transform: scaleY(0.3);
  }
}

@media (prefers-reduced-motion: no-preference) {
  .strip-block {
    animation: strip-block-in 0.45s cubic-bezier(0.16, 1, 0.3, 1) backwards;
    animation-delay: calc(var(--i) * 18ms);
  }
}
</style>
