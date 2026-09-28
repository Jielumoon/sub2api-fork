import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { computed } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import AppSidebar from '../AppSidebar.vue'

const shopUrl = 'https://shop.example.com/s/abc'

const { appStore, authStore, adminSettingsStore } = vi.hoisted(() => ({
  appStore: {
    backendModeEnabled: false,
    publicSettingsLoaded: true,
    mobileOpen: false,
    sidebarCollapsed: false,
    sidebarScrollTop: 0,
    siteLogo: '',
    siteName: 'Site',
    siteVersion: '',
    setMobileOpen: () => {},
    toggleSidebar: () => {},
    cachedPublicSettings: {
      custom_menu_items: [
        { id: 'docs', label: 'Docs', icon_svg: '', url: 'https://docs.example.com/', visibility: 'user', sort_order: 0 },
        { id: 'shop', label: 'Shop', icon_svg: '', url: 'https://shop.example.com/s/abc', visibility: 'user', sort_order: 1, open_mode: 'new_tab' },
      ],
    },
  },
  authStore: { isAdmin: false, isSimpleMode: false, token: 'secret-token', user: { id: 7 } },
  adminSettingsStore: {
    customMenuItems: [
      { id: 'ops-shop', label: 'Ops Shop', icon_svg: '', url: 'https://shop.example.com/s/abc', visibility: 'admin', sort_order: 0, open_mode: 'new_tab' },
    ],
    opsMonitoringEnabled: false,
    paymentEnabled: false,
    fetch: () => Promise.resolve(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
  useAdminSettingsStore: () => adminSettingsStore,
  useOnboardingStore: () => ({ isCurrentStep: () => false, nextStep: () => {} }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: computed(() => false), refreshBatchImageAccess: () => Promise.resolve() }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }),
}))

async function mountSidebar(isAdmin: boolean) {
  authStore.isAdmin = isAdmin
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  await router.push('/dashboard')
  await router.isReady()
  const wrapper = mount(AppSidebar, {
    global: { plugins: [router], stubs: { VersionBadge: true, Icon: true } },
  })
  await flushPromises()
  return wrapper
}

function linkByLabel(wrapper: Awaited<ReturnType<typeof mountSidebar>>, label: string) {
  const link = wrapper.findAll('a.sidebar-link').find((a) => a.text().trim() === label)
  if (!link) throw new Error(`sidebar link ${label} not found`)
  return link.element as HTMLAnchorElement
}

describe('AppSidebar custom menu open modes (mounted)', () => {
  it('renders new_tab items as external links with the raw URL and keeps embed items on the router', async () => {
    const wrapper = await mountSidebar(false)
    const shop = linkByLabel(wrapper, 'Shop')
    expect(shop.getAttribute('href')).toBe(shopUrl)
    expect(shop.target).toBe('_blank')
    expect(shop.rel).toBe('noopener noreferrer')
    expect(shop.href).not.toContain('token')

    const docs = linkByLabel(wrapper, 'Docs')
    expect(docs.getAttribute('href')).toBe('/custom/docs')
    expect(docs.target).toBe('')
    wrapper.unmount()
  })

  it('renders new_tab items as external links in both admin sections', async () => {
    const wrapper = await mountSidebar(true)
    const shop = linkByLabel(wrapper, 'Ops Shop')
    expect(shop.getAttribute('href')).toBe(shopUrl)
    expect(shop.target).toBe('_blank')
    // 管理员「我的账户」区里的用户菜单项同样走外链
    expect(linkByLabel(wrapper, 'Shop').getAttribute('href')).toBe(shopUrl)
    expect(linkByLabel(wrapper, 'Docs').getAttribute('href')).toBe('/custom/docs')
    wrapper.unmount()
  })
})

