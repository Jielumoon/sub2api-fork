import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ProxiesView from '../ProxiesView.vue'

const { list, create, update, getAllWithCount, showError } = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  getAllWithCount: vi.fn(),
  showError: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { proxies: { list, create, update, getAllWithCount } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
// 带上插值参数，才能断言占位符是以参数形式传给文案的（花括号直接写进文案会被 vue-i18n 当成插值）
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => (params ? `${key}(${Object.values(params).join(' ')})` : key),
  }),
}))
const mountView = () => shallowMount(ProxiesView, {
  global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
    DataTable: { props: ['data'], template: '<div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
  } },
})
const TEMPLATE_PASSWORD_REQUIRED = 'admin.proxies.templatePasswordRequired({account_id} Default.{account_id})'

let wrapper: ReturnType<typeof mountView>
beforeEach(() => {
  vi.clearAllMocks()
  list.mockResolvedValue({ items: [{ id: 9, name: 'resin', protocol: 'http', host: 'resin', port: 2260, username: 'Default', password: 'token', status: 'active' }], total: 1, pages: 1 })
  getAllWithCount.mockResolvedValue([])
  create.mockResolvedValue({})
  update.mockResolvedValue({})
})
afterEach(() => wrapper?.unmount())

async function editUsername(value: string) {
  wrapper = mountView(); await flushPromises()
  await wrapper.findAll('button').find(button => button.text() === 'common.edit')!.trigger('click')
  await wrapper.findAll<HTMLInputElement>('#edit-proxy-form input').find(input => input.element.value === 'Default')!.setValue(value)
}
async function fillCreate(username: string, password: string) {
  wrapper = mountView(); await flushPromises()
  await wrapper.findAll('button').find(button => button.text().includes('admin.proxies.createProxy'))!.trigger('click')
  const form = wrapper.get('#create-proxy-form')
  const textInputs = form.findAll<HTMLInputElement>('input[type="text"]')
  await textInputs[0].setValue('resin')
  await textInputs[1].setValue('resin')
  await form.get('input[type="number"]').setValue(2260)
  await form.findAll<HTMLInputElement>('input').find(input => input.attributes('placeholder') === 'admin.proxies.optionalAuth' && input.attributes('type') === 'text')!.setValue(username)
  await form.get('input[type="password"]').setValue(password)
}
const submit = async (id: string) => { await wrapper.get(id).trigger('submit'); await flushPromises() }

describe('proxy username template', () => {
  it('passes the placeholders to the hint as literal params', async () => {
    await editUsername('Default')
    expect(wrapper.get('#edit-proxy-form').text()).toContain('admin.proxies.usernameTemplateHint({account_id} Default.{account_id})')
  })

  it('keeps the existing password when switching to a template', async () => {
    await editUsername('Default.{account_id}')
    await submit('#edit-proxy-form')
    expect(update).toHaveBeenCalledTimes(1)
    expect(update.mock.calls[0][1].username).toBe('Default.{account_id}')
  })

  it('blocks editing to a template username without password', async () => {
    await editUsername('user-session-{account_id}')
    await wrapper.get('#edit-proxy-form input[type="password"]').setValue('')
    await submit('#edit-proxy-form')
    expect(update).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith(TEMPLATE_PASSWORD_REQUIRED)
  })

  it('blocks creating a template username without password', async () => {
    await fillCreate('Default.{account_id}', '')
    await submit('#create-proxy-form')
    expect(create).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith(TEMPLATE_PASSWORD_REQUIRED)
  })

  it('creates a template proxy when a password is set', async () => {
    await fillCreate('Default.{account_id}', 'token')
    await submit('#create-proxy-form')
    expect(create).toHaveBeenCalledTimes(1)
    expect(create.mock.calls[0][0]).toMatchObject({ username: 'Default.{account_id}', password: 'token' })
  })
})
