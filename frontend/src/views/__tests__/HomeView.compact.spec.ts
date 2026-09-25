import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import HomeView from '../HomeView.vue'

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback site',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    user: null as { email?: string } | null,
    checkAuth: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountHome(settings: Record<string, unknown> = {}) {
  appStore.cachedPublicSettings = {
    site_name: 'Test site',
    site_subtitle: 'Test subtitle',
    ...settings,
  }

  return mount(HomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div data-testid="locale-switcher" />' },
        Icon: { template: '<span data-testid="icon" />' },
        HomeSessionReplay: { props: ['baseUrl'], template: '<div data-testid="replay" />' },
        HomeConnectPanel: { props: ['baseUrl'], template: '<div data-testid="connect" />' },
      },
    },
  })
}

function compactDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper.get('[data-testid="compact-home"]').findComponent(RouterLinkStub).props('to')
}

function modelPlazaDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.props('to') === '/model-plaza')
    ?.props('to')
}

describe('HomeView compact mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.user = null
    authStore.checkAuth.mockClear()
    appStore.fetchPublicSettings.mockClear()
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  it('renders custom HTML ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: '<section id="custom-home">Custom home</section>',
    })

    expect(wrapper.get('#custom-home').text()).toBe('Custom home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('renders custom URL content ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: ' https://example.com/home ',
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('https://example.com/home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('treats whitespace-only custom content as empty and selects compact mode', () => {
    const wrapper = mountHome({ compact_home_enabled: true, home_content: ' \n\t ' })

    expect(wrapper.get('[data-testid="compact-home"]').text()).toContain('Test site')
  })

  it.each([undefined, false])('selects the default home when compact mode is %s', (enabled) => {
    const settings = enabled === undefined ? {} : { compact_home_enabled: enabled }
    const wrapper = mountHome(settings)

    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="default-home"]').exists()).toBe(true)
  })

  it('links unauthenticated visitors to login', () => {
    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/login')
  })

  it('links authenticated users to their dashboard', () => {
    authStore.isAuthenticated = true

    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/dashboard')
  })

  it('links administrators to the admin dashboard', () => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true

    const wrapper = mountHome({ compact_home_enabled: true })
    expect(compactDestination(wrapper)).toBe('/admin/dashboard')
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('shows the model plaza link to anonymous visitors when public access is enabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link from anonymous visitors when sign-in is required', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it('shows the model plaza link to authenticated visitors when sign-in is required', () => {
    authStore.isAuthenticated = true

    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('shows the model plaza link in the default home header', () => {
    const wrapper = mountHome({
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link when the feature is disabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: false,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })
})

describe('HomeView default mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.user = null
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  function linkTargets(wrapper: ReturnType<typeof mountHome>) {
    return wrapper
      .get('main')
      .findAllComponents(RouterLinkStub)
      .map((link) => link.props('to'))
  }

  it('offers sign-up and login to anonymous visitors when registration is open', () => {
    const targets = linkTargets(mountHome({ registration_enabled: true }))

    expect(targets).toContain('/register')
    expect(targets).toContain('/login')
  })

  it('hides every sign-up entry when registration is closed', () => {
    const wrapper = mountHome({ registration_enabled: false })
    const targets = linkTargets(wrapper)

    expect(targets).not.toContain('/register')
    expect(targets).toContain('/login')
    // 第 1 步整步改成登录，不留注册字样
    expect(wrapper.get('main').text()).toContain('home.steps.account.loginTitle')
    expect(wrapper.get('main').text()).not.toContain('home.steps.account.title')
  })

  it('sends signed-in users to their dashboard instead of login', () => {
    authStore.isAuthenticated = true

    const targets = linkTargets(mountHome({ registration_enabled: true }))
    expect(targets).toContain('/dashboard')
    expect(targets).not.toContain('/login')
    expect(targets).not.toContain('/register')
  })

  it.each([
    ['https://api.example.com/v1/', 'https://api.example.com'],
    ['https://api.example.com/', 'https://api.example.com'],
    ['https://api.example.com/v1//', 'https://api.example.com'],
    ['', window.location.origin],
  ])('passes the API root for %j to the replay and connect panel', (configured, expected) => {
    const wrapper = mountHome({ api_base_url: configured })

    expect(wrapper.getComponent('[data-testid="replay"]').props('baseUrl')).toBe(expected)
    expect(wrapper.getComponent('[data-testid="connect"]').props('baseUrl')).toBe(expected)
  })
})

describe('HomeView headline', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  it('names the heading once and keeps each latin word together', () => {
    const h1 = mountHome({ site_subtitle: 'Happy Vibe Coding!' }).get('h1')

    expect(h1.attributes('aria-label')).toBe('Happy Vibe Coding!')
    // 文本只出现一次：复制、页内查找、爬虫都不会读到重复内容
    expect(h1.element.textContent?.replace(/\s+/g, ' ').trim()).toBe('Happy Vibe Coding!')
    expect(h1.findAll('.home-char')).toHaveLength('HappyVibeCoding!'.length)
    // 单词是 inline-block 但不 nowrap：放得下时整体，超长时仍能折行
    expect(h1.findAll('.inline-block:not(.home-char)').map((word) => word.text())).toEqual(['Happy', 'Vibe', 'Coding!'])
    expect(h1.find('.whitespace-nowrap').exists()).toBe(false)
  })

  it('lets a CJK subtitle wrap between characters', () => {
    const h1 = mountHome({ site_subtitle: '一个很长很长的中文副标题' }).get('h1')

    expect(h1.findAll('.inline-block:not(.home-char)')).toHaveLength(0)
    expect(h1.findAll('.home-char')).toHaveLength(12)
  })

  it('splits by grapheme so emoji and combining marks stay intact', () => {
    const h1 = mountHome({ site_subtitle: 'Cafe\u0301 👩‍💻' }).get('h1')

    expect(h1.findAll('.home-char').map((char) => char.text())).toEqual(['C', 'a', 'f', 'e\u0301', '👩‍💻'])
  })

  it('shows shaped scripts such as Arabic as plain text', () => {
    const h1 = mountHome({ site_subtitle: 'مرحبا بالبرمجة' }).get('h1')

    expect(h1.find('.home-char').exists()).toBe(false)
    expect(h1.attributes('aria-label')).toBeUndefined()
    expect(h1.text()).toBe('مرحبا بالبرمجة')
  })
})
