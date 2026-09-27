<template>
  <!-- 折数变了（比如切换分组）就换一个 key，让印章重新盖一次 -->
  <Transition name="plaza-stamp-swap" mode="out-in">
    <span
      v-if="isDiscount(ratio)"
      :key="value"
      class="plaza-stamp font-display"
      :class="{ 'is-intro': intro }"
      data-stamp
    >
      <span v-if="caption" class="plaza-stamp-caption">{{ caption }}</span>
      <span class="plaza-stamp-value">{{ value }}</span>
    </span>
  </Transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { isDiscount } from '@/utils/plazaCatalog'
import { discountText } from './plazaBadges'

const props = withDefaults(
  defineProps<{
    /** 实付 ÷ 原价；不构成折扣（≥ 0.99 或缺失）时不渲染。 */
    ratio: number | null
    /** 还有更贵的分组：印章上写「低至」。 */
    floor?: boolean
    /** 首次加载的盖章动画；延迟由祖先元素上的 --i（第几张卡片）决定。 */
    intro?: boolean
  }>(),
  { floor: false, intro: false }
)

const { t } = useI18n()

const value = computed(() => (isDiscount(props.ratio) ? discountText(props.ratio, t) : ''))
const caption = computed(() => (props.floor ? t('modelPlaza.discount.upTo') : ''))
</script>

<style scoped>
/*
 * 双线朱印：外框 border + 内收的 outline 形成两道线，倾斜角由 --stamp-tilt 控制，
 * 卡片悬停时改这个变量让印章轻轻摆一下（变量会继承，不受 scoped 限制）。
 */
.plaza-stamp {
  --stamp-color: theme('colors.primary.600');
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--stamp-color);
  border: 2px solid currentColor;
  outline: 1px solid currentColor;
  outline-offset: 2px;
  border-radius: 10px;
  gap: 3px;
  padding: 6px 9px 7px;
  background-color: color-mix(in srgb, var(--stamp-color) 7%, transparent);
  line-height: 1;
  white-space: nowrap;
  user-select: none;
  transform: rotate(var(--stamp-tilt, -8deg));
}

.dark .plaza-stamp {
  --stamp-color: theme('colors.primary.400');
}

.plaza-stamp-caption {
  font-size: 10px;
  font-weight: 600;
}

.plaza-stamp-value {
  font-size: 17px;
  font-weight: 750;
  letter-spacing: -0.01em;
}

@media (prefers-reduced-motion: no-preference) {
  .plaza-stamp {
    transition: transform 260ms cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  /* 每张卡片先划线（PlazaPrice，起于 160ms），再盖章 */
  .plaza-stamp.is-intro {
    animation: plaza-stamp-press 520ms cubic-bezier(0.2, 0.9, 0.25, 1.2) both;
    animation-delay: calc(var(--i, 0) * 60ms + 460ms);
  }

  .plaza-stamp-swap-enter-active {
    animation: plaza-stamp-press 420ms cubic-bezier(0.2, 0.9, 0.25, 1.2) both;
  }

  .plaza-stamp-swap-leave-active {
    transition: opacity 90ms ease;
  }

  .plaza-stamp-swap-leave-to {
    opacity: 0;
  }
}

/* 从高处带着旋转压下来，轻微回弹后落定 */
@keyframes plaza-stamp-press {
  0% {
    opacity: 0;
    transform: rotate(calc(var(--stamp-tilt, -8deg) - 16deg)) scale(1.9);
  }
  55% {
    opacity: 1;
    transform: rotate(calc(var(--stamp-tilt, -8deg) + 2deg)) scale(0.92);
  }
  100% {
    opacity: 1;
    transform: rotate(var(--stamp-tilt, -8deg)) scale(1);
  }
}
</style>
