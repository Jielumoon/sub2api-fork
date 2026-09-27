<template>
  <Teleport to="body">
    <Transition name="plaza-drawer" :duration="DURATION">
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex"
        :class="[side === 'left' ? 'plaza-drawer-left justify-start' : 'plaza-drawer-right justify-end']"
      >
        <div
          class="plaza-drawer-backdrop absolute inset-0 bg-gray-900/40 dark:bg-black/60"
          aria-hidden="true"
          @click="emit('close')"
        ></div>
        <section
          ref="panelRef"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          class="plaza-drawer-panel relative flex h-full w-full flex-col bg-white shadow-2xl dark:bg-dark-900"
          :class="width"
        >
          <header
            class="flex items-start justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700/60"
          >
            <div class="min-w-0 flex-1">
              <slot name="header" :title-id="titleId">
                <h2 :id="titleId" class="text-base font-semibold text-gray-900 dark:text-white">{{ title }}</h2>
              </slot>
            </div>
            <button
              type="button"
              class="-mr-2 shrink-0 rounded-xl p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:text-dark-500 dark:hover:bg-dark-800 dark:hover:text-dark-300"
              :aria-label="t('common.close')"
              @click="emit('close')"
            >
              <Icon name="x" size="md" />
            </button>
          </header>
          <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain">
            <slot></slot>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts">
let drawerIdCounter = 0
</script>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useEventListener } from '@vueuse/core'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(
  defineProps<{
    show: boolean
    /** 标题；自定义 header 插槽时用插槽的 titleId 标注标题元素。 */
    title?: string
    side?: 'left' | 'right'
    /** 面板最大宽度（Tailwind 类），窄屏总是占满。 */
    width?: string
  }>(),
  { title: '', side: 'right', width: 'sm:max-w-2xl' }
)

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()

/** 与样式里的过渡时长一致；嵌套子元素做动画，需要显式告诉 Transition 时长。 */
const DURATION = 220
const titleId = `plaza-drawer-title-${++drawerIdCounter}`
const panelRef = ref<HTMLElement | null>(null)
let previousFocus: HTMLElement | null = null

/** 可用 Tab 到达的元素：排除禁用项与 tabindex="-1"（如标签页里未激活的标签）。 */
const TABBABLE = ['a[href]', 'button', 'input', 'select', 'textarea', '[tabindex]']
  .map((selector) => `${selector}:not([disabled]):not([tabindex="-1"])`)
  .join(', ')

function tabbables(): HTMLElement[] {
  return Array.from(panelRef.value?.querySelectorAll<HTMLElement>(TABBABLE) ?? [])
}

function lockScroll(locked: boolean) {
  document.body.classList.toggle('modal-open', locked)
}

// 打开：锁滚动并聚焦面板内首个可聚焦元素；关闭：解锁并把焦点还给触发元素（同 BaseDialog）。
watch(
  () => props.show,
  async (open) => {
    if (open) {
      previousFocus = document.activeElement as HTMLElement | null
      lockScroll(true)
      await nextTick()
      tabbables()[0]?.focus()
      return
    }
    lockScroll(false)
    previousFocus?.focus?.()
    previousFocus = null
  },
  { immediate: true }
)

/**
 * aria-modal 的面板必须把 Tab 焦点困在里面：首尾循环；焦点若已跑到面板外（如点了遮罩）就拉回首个元素。
 */
function trapTab(event: KeyboardEvent) {
  const items = tabbables()
  if (items.length === 0) {
    event.preventDefault()
    return
  }
  const first = items[0]
  const last = items[items.length - 1]
  const active = document.activeElement as HTMLElement | null
  const inside = active != null && panelRef.value?.contains(active)
  if (!inside || (event.shiftKey ? active === first : active === last)) {
    event.preventDefault()
    ;(event.shiftKey && inside ? last : first).focus()
  }
}

useEventListener(document, 'keydown', (event: KeyboardEvent) => {
  if (!props.show) return
  if (event.key === 'Escape') emit('close')
  else if (event.key === 'Tab') trapTab(event)
})

onBeforeUnmount(() => {
  if (props.show) lockScroll(false)
})
</script>

<style scoped>
@media (prefers-reduced-motion: no-preference) {
  .plaza-drawer-enter-active .plaza-drawer-backdrop,
  .plaza-drawer-leave-active .plaza-drawer-backdrop {
    transition: opacity 220ms ease;
  }

  .plaza-drawer-enter-active .plaza-drawer-panel,
  .plaza-drawer-leave-active .plaza-drawer-panel {
    transition: transform 220ms cubic-bezier(0.32, 0.72, 0, 1);
  }

  .plaza-drawer-enter-from .plaza-drawer-backdrop,
  .plaza-drawer-leave-to .plaza-drawer-backdrop {
    opacity: 0;
  }

  .plaza-drawer-right.plaza-drawer-enter-from .plaza-drawer-panel,
  .plaza-drawer-right.plaza-drawer-leave-to .plaza-drawer-panel {
    transform: translateX(100%);
  }

  .plaza-drawer-left.plaza-drawer-enter-from .plaza-drawer-panel,
  .plaza-drawer-left.plaza-drawer-leave-to .plaza-drawer-panel {
    transform: translateX(-100%);
  }
}
</style>
