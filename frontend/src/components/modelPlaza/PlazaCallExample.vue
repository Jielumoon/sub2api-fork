<template>
  <div v-if="examples.length" class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
    <div
      role="tablist"
      :aria-label="t('modelPlaza.example.title')"
      class="flex gap-1 overflow-x-auto border-b border-gray-200 bg-gray-50/70 px-2 dark:border-dark-700 dark:bg-dark-800/40"
    >
      <button
        v-for="(example, index) in examples"
        :id="tabId(example.kind)"
        :key="example.kind"
        ref="tabRefs"
        type="button"
        role="tab"
        :aria-selected="index === activeIndex"
        :aria-controls="panelId"
        :tabindex="index === activeIndex ? 0 : -1"
        class="-mb-px shrink-0 border-b-2 px-3 py-2.5 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500"
        :class="
          index === activeIndex
            ? 'border-primary-600 text-gray-900 dark:border-primary-400 dark:text-white'
            : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200'
        "
        @click="active = example.kind"
        @keydown="onKeydown($event, index)"
      >
        {{ example.label }}
      </button>
    </div>
    <div :id="panelId" role="tabpanel" :aria-labelledby="tabId(current.kind)">
      <div class="flex items-center justify-between gap-3 px-4 pt-3">
        <span class="truncate font-code text-xs text-gray-500 dark:text-dark-400">curl</span>
        <button
          type="button"
          class="btn btn-ghost btn-sm shrink-0 gap-1.5"
          :class="{ 'text-emerald-600 dark:text-emerald-400': copied }"
          @click="copy"
        >
          <Icon :name="copied ? 'check' : 'copy'" size="sm" />
          {{ copied ? t('modelPlaza.example.copied') : t('modelPlaza.example.copy') }}
        </button>
      </div>
      <pre
        class="overflow-x-auto px-4 pb-4 pt-2 font-code text-[12.5px] leading-6 text-gray-800 [font-variant-ligatures:none] dark:text-gray-200"
        data-example-code
      ><code><template v-for="(line, i) in current.lines" :key="i"><span
        v-for="(part, j) in line"
        :key="j"
        :class="PART_CLASS[part.kind]"
      >{{ part.text }}</span>{{ i < current.lines.length - 1 ? '\n' : '' }}</template></code></pre>
    </div>
    <p class="border-t border-gray-100 px-4 py-2.5 text-xs text-gray-500 dark:border-dark-800 dark:text-dark-400">
      {{ t('modelPlaza.example.keyHint', { key: KEY_PLACEHOLDER }) }}
    </p>
  </div>
</template>

<script lang="ts">
let exampleIdCounter = 0
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { BILLING_MODE_IMAGE, BILLING_MODE_TOKEN } from '@/constants/channel'
import type { PlazaCatalogEntry } from '@/utils/plazaCatalog'
import { billingMode } from '@/utils/plazaPricing'

const props = defineProps<{
  entry: PlazaCatalogEntry
  /** 站点 API 根地址，已去掉结尾的 /v1 和 /。 */
  baseUrl: string
}>()

type ExampleKind = 'anthropic' | 'openai-chat' | 'gemini' | 'antigravity-claude' | 'antigravity-gemini' | 'images'
type PartKind = 'plain' | 'url' | 'key'

const KEY_PLACEHOLDER = 'sk-xxx'
const PART_CLASS: Record<PartKind, string> = {
  plain: '',
  url: 'font-semibold text-primary-700 dark:text-primary-300',
  key: 'text-gray-500 underline decoration-dotted underline-offset-4 dark:text-dark-400'
}
/** 标签页顺序固定，同一模型多平台时展示稳定。 */
const KIND_ORDER: ExampleKind[] = ['anthropic', 'openai-chat', 'gemini', 'antigravity-claude', 'antigravity-gemini', 'images']
const KIND_LABEL: Record<ExampleKind, string> = {
  anthropic: 'Anthropic Messages',
  'openai-chat': 'OpenAI Chat Completions',
  gemini: 'Gemini generateContent',
  'antigravity-claude': 'Antigravity · Messages',
  'antigravity-gemini': 'Antigravity · Gemini',
  images: 'OpenAI Images'
}

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const uid = ++exampleIdCounter
const panelId = `plaza-example-panel-${uid}`
const tabRefs = ref<HTMLButtonElement[]>([])
const active = ref<ExampleKind | null>(null)

/**
 * 端点按报价的具体平台选（路由见 backend/internal/server/routes/gateway.go）：
 * anthropic → /v1/messages；gemini → /v1beta generateContent；antigravity 走 /antigravity 前缀；
 * 其余 OpenAI 兼容平台 → /v1/chat/completions。
 * 图片计费：/v1/images/generations 只分发 OpenAI / Grok（gateway.go imagesHandler，其余 404）；
 * Gemini / Antigravity 生图走 generateContent 并按回图计费；别的平台没有可用的生图入口，不给示例。
 * 按次计费的工具类模型没有通用调用形态，也不给示例。
 */
function exampleKind(platform: string, mode: string, name: string): ExampleKind | null {
  if (mode === BILLING_MODE_IMAGE) {
    switch (platform) {
      case 'openai':
      case 'grok':
        return 'images'
      case 'gemini':
        return 'gemini'
      case 'antigravity':
        return 'antigravity-gemini'
      default:
        return null
    }
  }
  if (mode !== BILLING_MODE_TOKEN) return null
  switch (platform) {
    case 'anthropic':
      return 'anthropic'
    case 'gemini':
      return 'gemini'
    case 'antigravity':
      return name.toLowerCase().startsWith('gemini') ? 'antigravity-gemini' : 'antigravity-claude'
    default:
      return 'openai-chat'
  }
}

/**
 * 把任意字符串包成一个 shell 单引号参数：单引号内除 ' 外没有特殊字符，' 写成 '\''。
 * 模型名、站点地址都来自配置，不能靠 JSON 转义代替 shell 转义，否则复制执行时可能被截断或注入命令。
 */
function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`
}

/**
 * 请求体每个字段一行、字段值保持紧凑，避免 messages 这类数组撑出十几行。
 * 每个参数整体做 shell 引用；多行的请求体续行只在 JSON 结构位置缩进，不改变内容。
 */
function curl(url: string, headers: string[], body: Record<string, unknown>): string[] {
  const fields = Object.entries(body).map(
    ([key, value], i, all) => `  ${JSON.stringify(key)}: ${JSON.stringify(value)}${i < all.length - 1 ? ',' : ''}`
  )
  const [first, ...rest] = shellQuote(['{', ...fields, '}'].join('\n')).split('\n')
  return [
    `curl ${shellQuote(url)} \\`,
    ...headers.map((h) => `  -H ${shellQuote(h)} \\`),
    `  -d ${first}`,
    ...rest.map((line) => `  ${line}`)
  ]
}

function exampleCode(kind: ExampleKind, root: string, model: string): string[] {
  const messages = [{ role: 'user', content: 'Hello' }]
  const bearer = [`Authorization: Bearer ${KEY_PLACEHOLDER}`, 'Content-Type: application/json']
  const anthropic = [`x-api-key: ${KEY_PLACEHOLDER}`, 'anthropic-version: 2023-06-01', 'Content-Type: application/json']
  const google = [`x-goog-api-key: ${KEY_PLACEHOLDER}`, 'Content-Type: application/json']
  const geminiBody = { contents: [{ parts: [{ text: 'Hello' }] }] }
  // 模型名进 URL 路径要编码，进请求体由 JSON 转义；两者再统一做 shell 引用
  const modelPath = encodeURIComponent(model)
  switch (kind) {
    case 'anthropic':
      return curl(`${root}/v1/messages`, anthropic, { model, max_tokens: 1024, messages })
    case 'antigravity-claude':
      return curl(`${root}/antigravity/v1/messages`, anthropic, { model, max_tokens: 1024, messages })
    case 'gemini':
      return curl(`${root}/v1beta/models/${modelPath}:generateContent`, google, geminiBody)
    case 'antigravity-gemini':
      return curl(`${root}/antigravity/v1beta/models/${modelPath}:generateContent`, google, geminiBody)
    case 'images':
      return curl(`${root}/v1/images/generations`, bearer, { model, prompt: 'A cute cat', size: '1024x1024' })
    case 'openai-chat':
      return curl(`${root}/v1/chat/completions`, bearer, { model, messages })
  }
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/** 只把站点地址和密钥占位符标出来。 */
function splitParts(line: string, root: string) {
  const marks: Array<[string, PartKind]> = [
    [root, 'url'],
    [KEY_PLACEHOLDER, 'key']
  ]
  const pattern = new RegExp(`(${marks.map(([text]) => escapeRegExp(text)).join('|')})`)
  return line
    .split(pattern)
    .filter(Boolean)
    .map((text) => ({ text, kind: marks.find(([mark]) => mark === text)?.[1] ?? ('plain' as PartKind) }))
}

const examples = computed(() => {
  const kinds = new Set<ExampleKind>()
  for (const { model } of props.entry.offers) {
    const kind = exampleKind(model.platform, billingMode(model), model.name)
    if (kind) kinds.add(kind)
  }
  return KIND_ORDER.filter((kind) => kinds.has(kind)).map((kind) => {
    const code = exampleCode(kind, props.baseUrl, props.entry.name)
    return {
      kind,
      label: KIND_LABEL[kind],
      text: code.join('\n'),
      lines: code.map((line) => splitParts(line, props.baseUrl))
    }
  })
})

const activeIndex = computed(() => Math.max(0, examples.value.findIndex((e) => e.kind === active.value)))
const current = computed(() => examples.value[activeIndex.value])

// 换模型时回到第一个标签页。
watch(
  () => props.entry.key,
  () => {
    active.value = null
  }
)

function tabId(kind: ExampleKind): string {
  return `plaza-example-tab-${uid}-${kind}`
}

function onKeydown(event: KeyboardEvent, index: number) {
  const last = examples.value.length - 1
  const target: Record<string, number> = {
    ArrowRight: index === last ? 0 : index + 1,
    ArrowLeft: index === 0 ? last : index - 1,
    Home: 0,
    End: last
  }
  const next = target[event.key]
  if (next === undefined) return
  event.preventDefault()
  active.value = examples.value[next].kind
  tabRefs.value[next]?.focus()
}

function copy() {
  copyToClipboard(current.value.text, t('modelPlaza.example.copySuccess'))
}
</script>
