<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    data-testid="default-home"
    class="relative flex min-h-screen flex-col overflow-hidden bg-gray-50 dark:bg-dark-950"
  >
    <!-- 仅保留一层静态的暖色背景。 -->
    <div class="pointer-events-none absolute inset-0 bg-mesh-gradient"></div>

    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <!-- Logo -->
        <div class="flex min-w-0 items-center gap-3">
          <div class="h-10 w-10 shrink-0 overflow-hidden rounded-xl">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="hidden truncate text-base font-semibold text-gray-900 dark:text-white sm:inline">
            {{ siteName }}
          </span>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-3">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Model Plaza Link -->
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="inline-flex items-center gap-1.5 rounded-lg p-2 text-sm text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-full bg-gray-900 py-1 pl-1 pr-2.5 transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-primary-600 text-[10px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-medium text-white">{{ t('home.dashboard') }}</span>
            <svg
              class="h-3 w-3 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-gray-900 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6 pb-20 pt-10 sm:pt-16">
      <div class="mx-auto max-w-6xl">
        <!-- 首屏：标题、入口和会话回放 -->
        <section class="mx-auto max-w-3xl text-center">
          <h1
            class="font-display text-5xl font-[750] tracking-tight text-gray-900 [overflow-wrap:anywhere] dark:text-white sm:text-6xl lg:text-7xl lg:leading-[1.05]"
            :aria-label="headlineWords ? siteSubtitle : undefined"
          >
            <!-- 逐字弹入；拉丁单词包成一个行内块，放得下时不从中间断开，比行宽还长时仍会折行 -->
            <span v-if="headlineWords" aria-hidden="true">
              <template v-for="(word, w) in headlineWords" :key="w">
                {{ w ? ' ' : '' }}<span :class="{ 'inline-block': word.group }"
                  ><span
                    v-for="(char, c) in word.chars"
                    :key="c"
                    class="home-char inline-block"
                    :style="{ '--i': word.start + c }"
                    >{{ char }}</span
                  ></span
                >
              </template>
            </span>
            <template v-else>{{ siteSubtitle }}</template>
          </h1>
          <p
            class="home-rise mx-auto mt-5 max-w-3xl text-lg text-gray-600 dark:text-dark-300"
            style="--d: 0.35s"
          >
            <!-- 两句各自成块，只在句间换行，避免末尾掉下一两个字 -->
            <span class="inline-block">{{ t('home.hero.description', { siteName }) }}</span>
            {{ ' ' }}
            <span class="inline-block">{{ t('home.hero.hint') }}</span>
          </p>
          <div class="home-rise mt-8 flex flex-wrap items-center justify-center gap-3" style="--d: 0.5s">
            <router-link v-if="isAuthenticated" :to="dashboardPath" class="btn btn-primary px-6 py-2.5 text-base">
              {{ t('home.goToDashboard') }}
            </router-link>
            <template v-else>
              <router-link v-if="registrationEnabled" to="/register" class="btn btn-primary px-6 py-2.5 text-base">
                {{ t('home.register') }}
              </router-link>
              <router-link
                to="/login"
                class="btn px-6 py-2.5 text-base"
                :class="registrationEnabled ? 'btn-secondary' : 'btn-primary'"
              >
                {{ t('home.login') }}
              </router-link>
            </template>
          </div>
        </section>

        <HomeSessionReplay class="mx-auto mt-12 max-w-3xl" :base-url="apiBaseUrl" />

        <!-- 三步接入与客户端配置 -->
        <section class="mt-24 grid grid-cols-1 items-start gap-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:gap-16">
          <div>
            <h2 class="font-display text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">
              {{ t('home.steps.title') }}
            </h2>
            <!-- 滚动到这里时依次点亮编号、画出连线：这是一个有先后顺序的流程 -->
            <ol ref="stepsList" class="home-steps mt-8 space-y-8" :class="{ 'is-shown': stepsShown }">
              <li
                v-for="(step, index) in steps"
                :key="step.key"
                class="relative flex gap-4"
                :style="{ '--i': index }"
              >
                <span
                  v-if="index < steps.length - 1"
                  class="step-line absolute -bottom-8 left-4 top-9 w-px bg-primary-300 dark:bg-primary-800"
                  aria-hidden="true"
                ></span>
                <span
                  class="step-num relative grid h-8 w-8 shrink-0 place-items-center rounded-full border border-primary-600 bg-primary-600 font-code text-sm font-semibold text-white"
                >
                  {{ index + 1 }}
                </span>
                <div class="step-body min-w-0 pt-1">
                  <h3 class="font-semibold text-gray-900 dark:text-white">{{ step.title }}</h3>
                  <p class="mt-1 text-sm leading-relaxed text-gray-600 dark:text-dark-400">{{ step.description }}</p>
                  <router-link
                    v-if="step.action"
                    :to="step.action.to"
                    class="mt-2 inline-flex text-sm font-medium text-primary-600 hover:text-primary-700 hover:underline dark:text-primary-400 dark:hover:text-primary-300"
                  >
                    {{ step.action.label }}
                  </router-link>
                </div>
              </li>
            </ol>
          </div>
          <div>
            <HomeConnectPanel :base-url="apiBaseUrl" />
            <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">{{ t('home.connect.fullConfig') }}</p>
          </div>
        </section>
      </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/50 px-6 py-8 dark:border-dark-800/50">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useIntersectionObserver } from '@vueuse/core'
import '@fontsource-variable/bricolage-grotesque'
import '@fontsource-variable/jetbrains-mono'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import HomeConnectPanel from '@/components/home/HomeConnectPanel.vue'
import HomeSessionReplay from '@/components/home/HomeSessionReplay.vue'
import { sanitizeUrl, toApiRoot } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const registrationEnabled = computed(() => appStore.cachedPublicSettings?.registration_enabled === true)
// 与密钥页一致：优先后台配置的 API 地址，否则用当前域名。
const apiBaseUrl = computed(() => toApiRoot(appStore.cachedPublicSettings?.api_base_url || window.location.origin))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// 中日韩文字逐字换行，不把整段包成一个块。
const CJK = /[\u3000-\u9fff\uff00-\uffef]/
// 连写或会重排字形的文字（希伯来、阿拉伯、天城文、泰文等）拆成独立行内块会断开字形，这类标题直接显示整句。
const SHAPED_SCRIPT = /[\u0590-\u08ff\u0900-\u0dff\u0e00-\u0eff\u1000-\u109f\u1780-\u17ff\ufb1d-\ufdff\ufe70-\ufeff]/

// 按字素拆分，保证 emoji、组合重音不被拆开。
function graphemes(text: string): string[] {
  if (typeof Intl.Segmenter !== 'function') return Array.from(text)
  return Array.from(new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(text), (part) => part.segment)
}

const headlineWords = computed(() => {
  if (SHAPED_SCRIPT.test(siteSubtitle.value)) return null
  let start = 0
  return siteSubtitle.value
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => {
      const chars = graphemes(word)
      const item = { chars, start, group: !CJK.test(word) }
      start += chars.length
      return item
    })
})

const stepsList = ref<HTMLElement | null>(null)
const stepsShown = ref(false)
const { isSupported: stepsObservable, stop: stopStepsObserver } = useIntersectionObserver(
  stepsList,
  ([entry]) => {
    if (!entry?.isIntersecting) return
    stepsShown.value = true
    stopStepsObserver()
  },
  { threshold: 0.35 }
)
if (!stepsObservable.value) stepsShown.value = true

const steps = computed(() => [
  {
    key: 'account',
    // 关闭注册时整步改成"登录"，不留任何注册字样。
    title: registrationEnabled.value ? t('home.steps.account.title') : t('home.steps.account.loginTitle'),
    description: registrationEnabled.value
      ? t('home.steps.account.description')
      : t('home.steps.account.loginDescription'),
    action: isAuthenticated.value
      ? null
      : registrationEnabled.value
        ? { to: '/register', label: t('home.steps.account.register') }
        : { to: '/login', label: t('home.steps.account.login') },
  },
  {
    key: 'key',
    title: t('home.steps.key.title'),
    description: t('home.steps.key.description'),
    action: { to: '/keys', label: t('home.steps.key.action') },
  },
  {
    key: 'paste',
    title: t('home.steps.paste.title'),
    description: t('home.steps.paste.description'),
    action: null,
  },
])

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
/* 首屏唯一一段编排动效：标题逐字弹入 → 说明与按钮 → 终端窗口（在 HomeSessionReplay 里）。 */
@media (prefers-reduced-motion: no-preference) {
  .home-char {
    animation: home-char-in 0.8s cubic-bezier(0.34, 1.56, 0.64, 1) backwards;
    animation-delay: calc(var(--i) * 40ms + 80ms);
    transition:
      transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1),
      color 0.2s ease;
  }

  .home-char:hover {
    transform: translateY(-0.12em) rotate(-5deg);
    color: theme('colors.primary.600');
  }

  /* 不能写 :global(.dark)：Vue 会把整条选择器编译成裸的 .dark。 */
  .dark .home-char:hover {
    color: theme('colors.primary.400');
  }

  .home-rise {
    animation: home-rise 0.7s ease-out backwards;
    animation-delay: var(--d);
  }

  /* 默认即最终样子；只有支持动效且还没滚到时才处于"未点亮"状态。 */
  .home-steps .step-num,
  .home-steps .step-line,
  .home-steps .step-body {
    transition:
      background-color 0.4s ease,
      border-color 0.4s ease,
      color 0.4s ease,
      opacity 0.5s ease,
      transform 0.5s cubic-bezier(0.34, 1.4, 0.64, 1);
    transition-delay: calc(var(--i) * 350ms);
  }

  .home-steps .step-line {
    transform-origin: top;
    transition-delay: calc(var(--i) * 350ms + 200ms);
  }

  .home-steps:not(.is-shown) .step-num {
    border-color: theme('colors.gray.300');
    background-color: theme('colors.white');
    color: theme('colors.gray.500');
    transform: scale(0.85);
  }

  .dark .home-steps:not(.is-shown) .step-num {
    border-color: theme('colors.dark.600');
    background-color: theme('colors.dark.900');
    color: theme('colors.dark.400');
  }

  .home-steps:not(.is-shown) .step-line {
    transform: scaleY(0);
  }

  .home-steps:not(.is-shown) .step-body {
    opacity: 0;
    transform: translateX(-8px);
  }
}

@keyframes home-char-in {
  from {
    opacity: 0;
    transform: translateY(0.45em) rotate(8deg);
    filter: blur(8px);
  }
}

@keyframes home-rise {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
}
</style>
