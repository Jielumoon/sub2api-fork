<template>
  <span class="inline-flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
    <span class="inline-flex items-baseline whitespace-nowrap">
      <!-- 价格变了（切换分组）就翻一下，让人看到是哪个数字变了 -->
      <Transition name="plaza-flip" mode="out-in">
        <span
          :key="paid"
          class="inline-block font-display font-semibold tracking-tight text-gray-900 tabular-nums dark:text-white"
          :class="SIZE[size].paid"
          data-paid-price
          >{{ paid }}</span
        >
      </Transition>
      <span v-if="suffix" class="ml-1 text-xs text-gray-400 dark:text-dark-500">{{ suffix }}</span>
    </span>
    <del
      v-if="original"
      class="plaza-del relative whitespace-nowrap text-gray-500 no-underline tabular-nums dark:text-dark-400"
      :class="[SIZE[size].original, { 'is-intro': intro }]"
      data-original-price
      ><span class="sr-only">{{ t('modelPlaza.price.original') }}</span>{{ original }}</del
    >
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

withDefaults(
  defineProps<{
    /** 已格式化的实付价，如 $1.05。 */
    paid: string
    /** 已格式化的划线原价；不打折时不传。 */
    original?: string | null
    /** 单位，如「/ 次」。 */
    suffix?: string
    size?: 'lg' | 'md' | 'sm'
    /** 首次加载时把划线从左往右画出来；延迟由祖先元素上的 --i 决定。 */
    intro?: boolean
  }>(),
  { original: null, suffix: '', size: 'md', intro: false }
)

const { t } = useI18n()

const SIZE = {
  lg: { paid: 'text-[22px] leading-none', original: 'text-[13px]' },
  md: { paid: 'text-lg leading-none', original: 'text-xs' },
  sm: { paid: 'text-sm', original: 'text-[11px]' }
} as const
</script>

<style scoped>
/* 斜着划掉：比 line-through 更像随手划的一笔，颜色和印章同源 */
.plaza-del::after {
  content: '';
  position: absolute;
  top: 52%;
  right: -3px;
  left: -3px;
  height: 1.5px;
  border-radius: 1px;
  background-color: theme('colors.primary.500');
  transform: rotate(-8deg);
  transform-origin: left center;
}

.dark .plaza-del::after {
  background-color: theme('colors.primary.400');
}

@media (prefers-reduced-motion: no-preference) {
  .plaza-del.is-intro::after {
    animation: plaza-strike 360ms cubic-bezier(0.65, 0, 0.35, 1) both;
    animation-delay: calc(var(--i, 0) * 60ms + 160ms);
  }

  .plaza-flip-enter-active,
  .plaza-flip-leave-active {
    transition:
      transform 170ms cubic-bezier(0.2, 0.8, 0.2, 1),
      opacity 170ms ease;
  }

  .plaza-flip-enter-from {
    opacity: 0;
    transform: translateY(45%);
  }

  .plaza-flip-leave-to {
    opacity: 0;
    transform: translateY(-45%);
  }
}

@keyframes plaza-strike {
  from {
    transform: rotate(-8deg) scaleX(0);
  }
  to {
    transform: rotate(-8deg) scaleX(1);
  }
}
</style>
