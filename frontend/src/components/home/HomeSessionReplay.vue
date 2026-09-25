<template>
  <div class="replay-stage relative isolate" @pointermove="onPointerMove" @pointerleave="resetTilt">
    <!-- 背后的光晕：思考时变亮，表示"正在干活" -->
    <div
      class="replay-glow pointer-events-none absolute -inset-x-8 bottom-2 top-16 -z-10 rounded-[3rem] bg-primary-500/25 blur-3xl dark:bg-primary-500/20"
      :class="status === 'spinning' ? 'opacity-100' : 'opacity-40'"
      aria-hidden="true"
    ></div>

    <figure
      class="replay-window relative overflow-hidden rounded-2xl border border-dark-700 bg-dark-900 text-left shadow-card-hover"
      :class="{ 'is-done': status === 'done' }"
      :style="tiltStyle"
    >
      <div class="replay-sheen pointer-events-none absolute inset-0" aria-hidden="true"></div>

      <div class="flex items-center border-b border-dark-700 bg-dark-800 px-4 py-2.5">
        <span class="flex gap-1.5" aria-hidden="true">
          <span v-for="n in 3" :key="n" class="h-2.5 w-2.5 rounded-full bg-dark-600"></span>
        </span>
        <span class="flex-1 pr-10 text-center font-code text-xs text-dark-400">claude</span>
      </div>

      <div
        role="img"
        :aria-label="t('home.replay.ariaLabel', { host })"
        class="overflow-x-auto px-5 py-4 font-code text-xs leading-6 text-gray-100 [font-variant-ligatures:none] sm:text-[13px]"
      >
        <!-- 所有行一开始就占位，只切换可见状态，播放过程中窗口高度不跳 -->
        <div aria-hidden="true">
          <p class="text-dark-400"><span class="text-primary-400">✻</span> Welcome to Claude Code</p>
          <p class="whitespace-nowrap pl-5 text-dark-400">API Base URL: {{ baseUrl }}</p>

          <p class="mt-4 flex gap-2" data-line="prompt">
            <span class="text-dark-400">&gt;</span>
            <span>{{ typedPrompt }}<span v-if="frame <= 1" class="replay-cursor"></span></span>
          </p>

          <p class="replay-line mt-4 flex gap-2 whitespace-nowrap" data-line="read" :class="{ 'is-hidden': frame < 3 }">
            <span :class="bulletClass(3)">●</span>
            <span><span class="font-semibold">Read</span>(src/views/HomeView.vue)</span>
          </p>
          <p class="replay-line replay-delay flex gap-2 pl-1 text-dark-400" :class="{ 'is-hidden': frame < 3 }">
            <span>└</span>
            <span>Read <CountUp :value="frame >= 3 ? 749 : 0" /> lines</span>
          </p>

          <p class="replay-line mt-2 flex gap-2 whitespace-nowrap" data-line="update" :class="{ 'is-hidden': frame < 4 }">
            <span :class="bulletClass(4)">●</span>
            <span><span class="font-semibold">Update</span>(src/views/HomeView.vue)</span>
          </p>
          <p class="replay-line replay-delay flex gap-2 pl-1 text-dark-400" :class="{ 'is-hidden': frame < 4 }">
            <span>└</span>
            <span>
              <span class="text-emerald-400">+<CountUp :value="frame >= 4 ? 128 : 0" /></span>
              <span class="ml-2 text-red-400">−<CountUp :value="frame >= 4 ? 214 : 0" /></span>
            </span>
          </p>

          <p class="replay-line mt-4 flex gap-2" data-line="reply" :class="{ 'is-hidden': frame < FINAL_FRAME }">
            <span>●</span>
            <span>{{ t('home.replay.reply') }}</span>
          </p>

          <p class="replay-line mt-4" data-line="status" :data-state="status" :class="{ 'is-hidden': status === 'idle' }">
            <span v-if="status === 'done'" class="text-dark-400">✻ Vibed for 4s</span>
            <template v-else>
              <span class="replay-spinner text-primary-400"></span>
              <span class="replay-shimmer"> Vibing…</span>
              <span class="text-dark-400"> (esc to interrupt)</span>
            </template>
          </p>
        </div>
      </div>

      <figcaption
        class="flex items-center gap-3 border-t border-dark-700 px-4 py-2 font-code text-[11px] text-dark-400 sm:text-xs"
      >
        <span class="min-w-0 truncate">{{ host }}</span>
        <span class="ml-auto shrink-0 tabular-nums" aria-hidden="true" data-tokens>
          ↑ <CountUp :value="tokens[0]" :format="formatCount" /> ↓
          <CountUp :value="tokens[1]" :format="formatCount" /> tokens
        </span>
        <button
          type="button"
          class="group inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-dark-300 transition-colors hover:bg-dark-800 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-400"
          @click="play()"
        >
          <Icon name="refresh" size="xs" class="transition-transform duration-500 motion-safe:group-hover:-rotate-180" />
          {{ t('home.replay.replay') }}
        </button>
      </figcaption>
    </figure>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePreferredReducedMotion } from '@vueuse/core'
import CountUp from '@/components/common/CountUp.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  /** 站点 API 根地址，已去掉结尾的 /v1 和 /。 */
  baseUrl: string
}>()

// 画面帧：0 等待，1 打字，2 思考，3 Read，4 Update，5 完成。
const FINAL_FRAME = 5
// 等窗口入场动画落定再开始打字。
const START_DELAY = 1300
const TYPE_DELAY = 45
const AFTER_PROMPT = 400
// 每一拍推进到哪一帧、token 计数滚到多少、停留多久（演示数据）。
const BEATS: ReadonlyArray<{ frame: number; tokens: [number, number]; hold: number }> = [
  { frame: 2, tokens: [1840, 96], hold: 1400 },
  { frame: 3, tokens: [2710, 312], hold: 900 },
  { frame: 4, tokens: [3204, 1014], hold: 1100 },
  { frame: FINAL_FRAME, tokens: [3204, 1187], hold: 0 },
]
// 鼠标跟随倾斜的最大角度（度）。
const TILT_X = 5
const TILT_Y = 7

const { t } = useI18n()
const reducedMotion = usePreferredReducedMotion()

const frame = ref(0)
const typed = ref(0)
const tokens = ref<[number, number]>([0, 0])
const tilt = ref({ x: 0, y: 0, mx: 50, my: 50 })

const host = computed(() => {
  try {
    return new URL(props.baseUrl).host
  } catch {
    return props.baseUrl
  }
})
const promptChars = computed(() => Array.from(t('home.replay.prompt')))
// 打字只发生在第 1 帧；之后（包括播放结束后切换语言）始终显示完整提示词。
const typedPrompt = computed(() =>
  frame.value >= 2 ? promptChars.value.join('') : promptChars.value.slice(0, typed.value).join('')
)
const status = computed(() => {
  if (frame.value < 2) return 'idle'
  return frame.value < FINAL_FRAME ? 'spinning' : 'done'
})
const tiltStyle = computed(() => ({
  transform: `perspective(1400px) rotateX(${tilt.value.x}deg) rotateY(${tilt.value.y}deg)`,
  '--mx': `${tilt.value.mx}%`,
  '--my': `${tilt.value.my}%`,
}))

const formatCount = (value: number) => value.toLocaleString('en-US')

// 工具调用进行中圆点闪烁，下一拍开始即视为完成、变绿。
function bulletClass(at: number): string {
  return frame.value === at ? 'replay-running text-dark-300' : 'text-emerald-400'
}

function onPointerMove(event: PointerEvent) {
  if (event.pointerType !== 'mouse' || reducedMotion.value === 'reduce') return
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const px = (event.clientX - rect.left) / rect.width
  const py = (event.clientY - rect.top) / rect.height
  tilt.value = { x: (0.5 - py) * TILT_X, y: (px - 0.5) * TILT_Y, mx: px * 100, my: py * 100 }
}

function resetTilt() {
  tilt.value = { x: 0, y: 0, mx: 50, my: 50 }
}

// 所有定时器都登记在这里；清掉后挂起的 await 不会再恢复，旧的一轮自然作废。
const timers = new Set<ReturnType<typeof setTimeout>>()

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    const id = setTimeout(() => {
      timers.delete(id)
      resolve()
    }, ms)
    timers.add(id)
  })
}

function stop() {
  timers.forEach(clearTimeout)
  timers.clear()
}

async function play(delay = TYPE_DELAY * 10) {
  stop()
  if (reducedMotion.value === 'reduce') {
    frame.value = FINAL_FRAME
    typed.value = promptChars.value.length
    tokens.value = BEATS[BEATS.length - 1].tokens
    return
  }
  frame.value = 0
  typed.value = 0
  tokens.value = [0, 0]
  await sleep(delay)
  frame.value = 1
  for (let i = 1; i <= promptChars.value.length; i++) {
    await sleep(TYPE_DELAY)
    typed.value = i
  }
  await sleep(AFTER_PROMPT)
  for (const beat of BEATS) {
    frame.value = beat.frame
    tokens.value = beat.tokens
    if (beat.hold) await sleep(beat.hold)
  }
}

onMounted(() => play(START_DELAY))
onBeforeUnmount(stop)
</script>

<style scoped>
.replay-cursor {
  display: inline-block;
  width: 0.55em;
  height: 1.1em;
  margin-left: 1px;
  vertical-align: text-bottom;
  background: theme('colors.gray.100');
}

.replay-line.is-hidden {
  opacity: 0;
  transform: translateY(6px);
  filter: blur(4px);
}

.replay-spinner::before {
  content: '✻';
}

.replay-shimmer {
  color: theme('colors.primary.400');
}

.replay-sheen {
  opacity: 0;
  background: radial-gradient(
    420px circle at var(--mx) var(--my),
    theme('colors.white / 7%'),
    transparent 45%
  );
  transition: opacity 0.3s ease;
}

@media (prefers-reduced-motion: no-preference) {
  .replay-stage {
    animation: replay-stage-in 0.9s cubic-bezier(0.34, 1.4, 0.64, 1) 0.45s backwards;
  }

  .replay-window {
    transition: transform 0.3s ease-out;
  }

  .replay-window.is-done {
    animation: replay-done 1.4s ease-out;
  }

  .replay-glow {
    transition: opacity 1s ease;
  }

  .replay-line {
    transition:
      opacity 0.4s ease,
      transform 0.5s cubic-bezier(0.34, 1.4, 0.64, 1),
      filter 0.4s ease;
  }

  .replay-delay:not(.is-hidden) {
    transition-delay: 0.35s;
  }

  .replay-cursor,
  .replay-running {
    animation: replay-blink 0.9s step-end infinite;
  }

  .replay-spinner::before {
    animation: replay-glyph 1.2s step-end infinite;
  }

  .replay-shimmer {
    background: linear-gradient(
      90deg,
      theme('colors.primary.400') 35%,
      theme('colors.primary.100') 50%,
      theme('colors.primary.400') 65%
    );
    background-size: 250% 100%;
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
    animation: replay-shimmer 1.8s linear infinite;
  }
}

@media (prefers-reduced-motion: no-preference) and (hover: hover) {
  .replay-stage:hover .replay-sheen {
    opacity: 1;
  }
}

@keyframes replay-stage-in {
  from {
    opacity: 0;
    transform: translateY(28px) scale(0.97);
  }
}

@keyframes replay-done {
  from {
    box-shadow:
      0 0 0 1px theme('colors.primary.400'),
      0 0 40px theme('colors.primary.500 / 35%');
  }
}

@keyframes replay-blink {
  50% {
    opacity: 0;
  }
}

@keyframes replay-glyph {
  0% {
    content: '·';
  }
  16.66% {
    content: '✢';
  }
  33.33% {
    content: '✳';
  }
  50% {
    content: '✶';
  }
  66.66% {
    content: '✻';
  }
  83.33% {
    content: '✽';
  }
}

@keyframes replay-shimmer {
  from {
    background-position: 150% 0;
  }
  to {
    background-position: -100% 0;
  }
}
</style>
