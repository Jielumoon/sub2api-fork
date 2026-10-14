import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import UpstreamsView from '../UpstreamsView.vue'
import type { Upstream } from '@/types'

const { list, create, update, probe, getAll, showError, showSuccess } = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  probe: vi.fn(),
  getAll: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))
vi.mock('@/api/admin', () => ({
  adminAPI: { upstreams: { list, create, update, probe, delete: vi.fn() }, proxies: { getAll } }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const row: Upstream = {
  id: 7,
  name: 'site-a',
  base_url: 'https://a.example.com',
  api_key_masked: 'sk-a***',
  api_key_configured: true,
  proxy_id: 3,
  enabled: true,
  interval_minutes: 15,
  next_probe_at: null,
  last_attempt_at: null,
  failure_count: 0,
  last_error: '',
  created_at: '2026-10-13T09:00:00Z',
  updated_at: '2026-10-13T09:00:00Z'
}

const mountView = () => shallowMount(UpstreamsView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
      DataTable: {
        props: ['data'],
        template: '<div><div v-for="r in data" :key="r.id" class="row"><slot name="cell-balance" :row="r" /><slot name="cell-actions" :row="r" /></div></div>'
      },
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
      UpstreamBalanceSummary: { props: ['snapshot'], template: '<output>{{ snapshot ? snapshot.status : "none" }}</output>' }
    }
  }
})

let wrapper: ReturnType<typeof mountView>
beforeEach(() => {
  vi.clearAllMocks()
  list.mockResolvedValue({ items: [{ ...row }], total: 1, page: 1, page_size: 20, pages: 1 })
  getAll.mockResolvedValue([])
  update.mockImplementation(async (id: number, payload: Partial<Upstream>) => ({ ...row, ...payload, id }))
})
afterEach(() => wrapper?.unmount())

async function openEdit() {
  wrapper = mountView()
  await flushPromises()
  await wrapper.get('[data-testid="upstream-edit"]').trigger('click')
}

describe('UpstreamsView', () => {
  it('编辑时 Key 留空不提交 api_key，未变更的代理也不提交', async () => {
    await openEdit()
    const keyInput = wrapper.get<HTMLInputElement>('#upstream-api-key')
    expect(keyInput.element.value).toBe('')
    expect(keyInput.attributes('placeholder')).toBe('sk-a***')
    await wrapper.get('#upstream-name').setValue('site-a2')
    await wrapper.get('#upstream-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledTimes(1)
    expect(update.mock.calls[0][0]).toBe(7)
    expect(update.mock.calls[0][1]).toEqual({ name: 'site-a2' })
  })

  it('编辑时填写新 Key 才提交 api_key', async () => {
    await openEdit()
    await wrapper.get('#upstream-api-key').setValue('  sk-new  ')
    await wrapper.get('#upstream-form').trigger('submit')
    await flushPromises()
    expect(update.mock.calls[0][1]).toEqual({ api_key: 'sk-new' })
  })

  it('手动刷新调用探测接口并更新该行快照', async () => {
    let resolveProbe: (value: unknown) => void = () => {}
    probe.mockReturnValue(new Promise((resolve) => { resolveProbe = resolve }))
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('output').text()).toBe('none')

    const button = wrapper.get('[data-testid="upstream-probe"]')
    await button.trigger('click')
    expect(probe).toHaveBeenCalledWith(7)
    expect(button.attributes('disabled')).toBeDefined()

    resolveProbe({
      upstream_id: 7,
      snapshot: {
        status: 'failed',
        last_error: 'proxy_unavailable',
        last_attempt_at: '2026-10-13T10:00:00Z',
        next_probe_at: '2026-10-13T10:15:00Z',
        failure_count: 1
      }
    })
    await flushPromises()
    expect(wrapper.get('output').text()).toBe('failed')
    expect(wrapper.get('[data-testid="upstream-probe"]').attributes('disabled')).toBeUndefined()
    expect(showError).not.toHaveBeenCalled()
  })

  it('手动刷新失败时提示错误', async () => {
    probe.mockRejectedValue({ status: 409, code: 409, reason: 'UPSTREAM_PROBE_BUSY', message: 'busy' })
    wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="upstream-probe"]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('admin.upstreams.errors.busy')
  })
})
