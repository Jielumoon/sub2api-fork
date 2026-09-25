<template>
  <div class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
    <div
      role="tablist"
      :aria-label="t('home.connect.clients')"
      class="relative flex gap-1 overflow-x-auto border-b border-gray-200 px-2 dark:border-dark-700"
    >
      <button
        v-for="(client, index) in clients"
        :id="tabId(client.id)"
        :key="client.id"
        ref="tabRefs"
        type="button"
        role="tab"
        :aria-selected="index === active"
        :aria-controls="PANEL_ID"
        :tabindex="index === active ? 0 : -1"
        class="shrink-0 rounded-t-lg px-3 py-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500"
        :class="
          index === active
            ? 'text-gray-900 dark:text-white'
            : 'text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200'
        "
        @click="active = index"
        @keydown="onKeydown($event, index)"
      >
        {{ client.label }}
      </button>
      <!-- 同一条下划线在标签之间滑动，而不是每个标签各自出现/消失 -->
      <span
        class="tab-indicator absolute -bottom-px left-0 h-0.5 rounded-full bg-primary-600 dark:bg-primary-400"
        :style="indicatorStyle"
        aria-hidden="true"
      ></span>
    </div>

    <!-- 每个文件单独一块、单独复制；换标签时整块重建，代码逐行滑入 -->
    <div
      :id="PANEL_ID"
      role="tabpanel"
      :aria-labelledby="tabId(current.id)"
      class="divide-y divide-gray-100 dark:divide-dark-800"
    >
      <section v-for="file in current.files" :key="file.key">
        <div class="flex items-center justify-between gap-3 px-4 pt-3">
          <span class="truncate font-code text-xs text-gray-500 dark:text-dark-400">{{ file.path }}</span>
          <button
            type="button"
            class="btn btn-ghost btn-sm shrink-0 gap-1.5"
            :class="{ 'text-emerald-600 dark:text-emerald-400': isCopied(file.key) }"
            @click="copy(file)"
          >
            <Icon
              :key="isCopied(file.key) ? 'check' : 'copy'"
              class="copy-icon"
              :name="isCopied(file.key) ? 'check' : 'copy'"
              size="sm"
            />
            {{ isCopied(file.key) ? t('home.connect.copied') : t('home.connect.copy') }}
            <span class="sr-only">{{ file.path }}</span>
          </button>
        </div>
        <!-- 只把本站地址和密钥占位符标出来 -->
        <pre
          class="overflow-x-auto px-4 pb-4 pt-2 font-code text-[13px] leading-6 text-gray-800 [font-variant-ligatures:none] dark:text-gray-200"
        ><code><span
          v-for="(line, i) in file.lines"
          :key="i"
          class="snippet-line block min-h-6"
          :style="{ '--i': file.offset + i }"
        ><span v-for="(part, j) in line" :key="j" :class="PART_CLASS[part.kind]">{{ part.text }}</span></span></code></pre>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useEventListener } from '@vueuse/core'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'

const props = defineProps<{
  /** 站点 API 根地址，已去掉结尾的 /v1 和 /。 */
  baseUrl: string
}>()

type PartKind = 'plain' | 'url' | 'key'

const PANEL_ID = 'home-connect-panel'
const PART_CLASS: Record<PartKind, string> = {
  plain: '',
  url: 'font-semibold text-primary-700 dark:text-primary-300',
  key: 'text-gray-500 underline decoration-dotted underline-offset-4 dark:text-dark-400',
}
// 下划线比标签文字两侧各缩进 12px（与按钮的 px-3 对齐）。
const INDICATOR_INSET = 12

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const active = ref(0)
// 最近一次复制的是哪个文件；"已复制"只显示在它上面，换标签后不会误导。
const lastCopied = ref<string | null>(null)
const tabRefs = ref<HTMLButtonElement[]>([])
const indicator = ref({ left: 0, width: 0 })

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function splitParts(line: string, marks: Array<[string, PartKind]>) {
  const pattern = new RegExp(`(${marks.map(([text]) => escapeRegExp(text)).join('|')})`)
  return line
    .split(pattern)
    .filter(Boolean)
    .map((text) => ({ text, kind: marks.find(([mark]) => mark === text)?.[1] ?? ('plain' as PartKind) }))
}

// 取自 UseKeyModal 各客户端配置的核心几行，是精简预览；
// 完整配置（Windows、settings.json、模型等可选项）以登录后的密钥页为准。
const clients = computed(() => {
  const root = props.baseUrl
  const key = t('home.connect.keyPlaceholder')
  const marks: Array<[string, PartKind]> = [
    [root, 'url'],
    [key, 'key'],
  ]
  return [
    {
      id: 'claude',
      label: t('keys.useKeyModal.cliTabs.claudeCode'),
      files: [
        {
          path: 'bash / zsh',
          code: [
            `export ANTHROPIC_BASE_URL="${root}"`,
            `export ANTHROPIC_AUTH_TOKEN="${key}"`,
            'export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1',
          ],
        },
      ],
    },
    {
      id: 'codex',
      label: t('keys.useKeyModal.cliTabs.codexCli'),
      files: [
        {
          path: '~/.codex/config.toml',
          code: [
            'model_provider = "OpenAI"',
            '',
            '[model_providers.OpenAI]',
            'name = "OpenAI"',
            `base_url = "${root}/v1"`,
            'wire_api = "responses"',
            'requires_openai_auth = true',
          ],
        },
        {
          path: '~/.codex/auth.json',
          code: JSON.stringify({ OPENAI_API_KEY: key }, null, 2).split('\n'),
        },
      ],
    },
    {
      id: 'gemini',
      label: t('keys.useKeyModal.cliTabs.geminiCli'),
      files: [
        {
          path: 'bash / zsh',
          code: [`export GOOGLE_GEMINI_BASE_URL="${root}"`, `export GEMINI_API_KEY="${key}"`],
        },
      ],
    },
  ].map((client) => {
    // offset 让多个文件的代码行接着错峰滑入，而不是每块都从头开始。
    let offset = 0
    const files = client.files.map((file, index) => {
      const item = {
        key: `${client.id}:${index}`,
        path: file.path,
        text: file.code.join('\n'),
        lines: file.code.map((line) => splitParts(line, marks)),
        offset,
      }
      offset += file.code.length
      return item
    })
    return { ...client, files }
  })
})

const current = computed(() => clients.value[active.value])

const indicatorStyle = computed(() => ({
  width: `${indicator.value.width}px`,
  transform: `translateX(${indicator.value.left}px)`,
}))

function measureIndicator() {
  const tab = tabRefs.value[active.value]
  if (!tab) return
  indicator.value = {
    left: tab.offsetLeft + INDICATOR_INSET,
    width: Math.max(0, tab.offsetWidth - INDICATOR_INSET * 2),
  }
}

// 切换标签、换语言（标签宽度变化）、窗口缩放时重新量一次。
watch([active, clients], () => nextTick(measureIndicator))
onMounted(measureIndicator)
useEventListener(window, 'resize', measureIndicator)

function tabId(id: string): string {
  return `home-connect-tab-${id}`
}

function onKeydown(event: KeyboardEvent, index: number) {
  const last = clients.value.length - 1
  const target: Record<string, number> = {
    ArrowRight: index === last ? 0 : index + 1,
    ArrowLeft: index === 0 ? last : index - 1,
    Home: 0,
    End: last,
  }
  const next = target[event.key]
  if (next === undefined) return
  event.preventDefault()
  active.value = next
  tabRefs.value[next]?.focus()
}

function isCopied(fileKey: string): boolean {
  return copied.value && lastCopied.value === fileKey
}

function copy(file: { key: string; text: string }) {
  lastCopied.value = file.key
  copyToClipboard(file.text, t('home.connect.copySuccess'))
}
</script>

<style scoped>
@media (prefers-reduced-motion: no-preference) {
  .tab-indicator {
    transition:
      transform 0.4s cubic-bezier(0.34, 1.4, 0.64, 1),
      width 0.4s cubic-bezier(0.34, 1.4, 0.64, 1);
  }

  .snippet-line {
    animation: snippet-in 0.35s ease-out backwards;
    animation-delay: calc(var(--i) * 30ms);
  }

  .copy-icon {
    animation: copy-pop 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
  }
}

@keyframes snippet-in {
  from {
    opacity: 0;
    transform: translateX(-6px);
  }
}

@keyframes copy-pop {
  from {
    transform: scale(0.4);
  }
}
</style>
