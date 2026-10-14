import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UpstreamBalanceSummary from '../UpstreamBalanceSummary.vue'
import type { UpstreamProbeSnapshot } from '@/types'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({
    t: (key: string) => key,
    n: (value: number, options?: Intl.NumberFormatOptions) => new Intl.NumberFormat('en-US', options).format(value)
  })
}))

const now = Date.parse('2026-10-13T10:00:00Z')
const base: UpstreamProbeSnapshot = {
  status: 'ok',
  data: { unit: 'USD', items: { wallet: { remaining: 0 } } },
  received_at: '2026-10-13T09:45:00Z',
  fresh_until: '2026-10-13T10:15:00Z',
  last_attempt_at: '2026-10-13T09:45:00Z',
  next_probe_at: '2026-10-13T10:00:00Z'
}
const render = (snapshot?: UpstreamProbeSnapshot | null, at = now) =>
  mount(UpstreamBalanceSummary, { props: { snapshot, now: at } })

describe('UpstreamBalanceSummary', () => {
  it('余额为 0 显示 $0.00，未返回的额度项不显示', () => {
    const wrapper = render(base)
    expect(wrapper.get('[data-testid="upstream-item-wallet"]').text()).toContain('$0.00')
    expect(wrapper.find('[data-testid="upstream-item-key_total"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="upstream-status"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="upstream-received"]').text()).toContain('admin.upstreams.balance.updated')
  })

  it('全部返回的额度项按顺序展示，无限制与限额明细可区分', () => {
    const wrapper = render({
      ...base,
      data: {
        unit: 'USD',
        items: {
          key_7d: { remaining: 3, limit: 10, used: 7 },
          subscription_monthly: { unlimited: true },
          wallet: { remaining: -1.5 }
        }
      }
    })
    const items = wrapper.findAll('[data-testid^="upstream-item-"]').map((item) => item.attributes('data-testid'))
    expect(items).toEqual(['upstream-item-wallet', 'upstream-item-key_7d', 'upstream-item-subscription_monthly'])
    expect(wrapper.get('[data-testid="upstream-item-wallet"]').text()).toContain('-$1.50')
    expect(wrapper.get('[data-testid="upstream-item-key_7d"]').text()).toContain('admin.upstreams.balance.limit $10.00')
    expect(wrapper.get('[data-testid="upstream-item-key_7d"]').text()).toContain('admin.upstreams.balance.used $7.00')
    expect(wrapper.get('[data-testid="upstream-item-subscription_monthly"]').text()).toContain('admin.upstreams.balance.unlimited')
  })

  it('没有快照时显示等待首次探测，而不是 0', () => {
    const wrapper = render(undefined)
    expect(wrapper.text()).toContain('admin.upstreams.balance.pending')
    expect(wrapper.text()).not.toContain('$')
  })

  it('失败时保留上次成功值并标记为上次成功', () => {
    const wrapper = render({ ...base, status: 'failed', last_error: 'proxy_unavailable' })
    expect(wrapper.get('[data-testid="upstream-status"]').text()).toContain('admin.upstreams.balance.status.failed')
    expect(wrapper.get('[data-testid="upstream-status"]').text()).toContain('proxy_unavailable')
    expect(wrapper.get('[data-testid="upstream-item-wallet"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-testid="upstream-received"]').text()).toContain('admin.upstreams.balance.lastSuccess')
  })

  it('ok 但超过 fresh_until 同样标记为陈旧', () => {
    const wrapper = render(base, now + 3600_000)
    expect(wrapper.get('[data-testid="upstream-received"]').text()).toContain('admin.upstreams.balance.lastSuccess')
  })

  it('unsupported / unavailable 显示状态文字且隐藏金额', () => {
    for (const status of ['unsupported', 'unavailable'] as const) {
      const wrapper = render({ ...base, status })
      expect(wrapper.get('[data-testid="upstream-status"]').text()).toContain(`admin.upstreams.balance.status.${status}`)
      expect(wrapper.find('[data-testid="upstream-amount"]').exists()).toBe(false)
      expect(wrapper.text()).not.toContain('$')
    }
  })
})
