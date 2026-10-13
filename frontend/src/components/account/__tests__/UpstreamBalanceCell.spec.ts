import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import UpstreamBalanceCell from '../UpstreamBalanceCell.vue'
import type { Account, UpstreamBalanceProbeSnapshot } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
    n: (value: number, options?: Intl.NumberFormatOptions) => new Intl.NumberFormat('en-US', options).format(value),
    locale: ref('en-US')
  })
}))

const now = Date.parse('2026-10-13T10:00:00Z')
const snapshot: UpstreamBalanceProbeSnapshot = {
  type: 'wallet', status: 'ok', data: { remaining: 0, unit: 'USD' },
  received_at: '2026-10-13T09:45:00Z', fresh_until: '2026-10-13T10:45:00Z',
  last_attempt_at: '2026-10-13T09:45:00Z', next_probe_at: '2026-10-13T10:15:00Z'
}
const account = (value?: UpstreamBalanceProbeSnapshot): Account => ({
  id: 1, type: 'apikey', extra: { upstream_balance_probe_type: 'wallet', upstream_balance_probe: value }
} as Account)

describe('上游余额单元格', () => {
  it('区分零、缺失和无限制，保留额度类型与刷新动作', async () => {
    const wrapper = mount(UpstreamBalanceCell, { props: { account: account(snapshot), now } })
    expect(wrapper.get('[data-testid="balance-amount"]').text()).toBe('$0.00')
    expect(wrapper.text()).toContain('types.wallet')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('probe')).toHaveLength(1)
    await wrapper.setProps({ account: account() })
    expect(wrapper.get('[data-testid="balance-amount"]').text()).toBe('—')
    expect(wrapper.text()).toContain('upstreamBalance.pending')
    await wrapper.setProps({ account: account({ ...snapshot, data: { unit: 'USD', unlimited: true } }) })
    expect(wrapper.text()).toContain('upstreamBalance.unlimited')
  })

  it('失败时标记上次成功，未提供类型时隐藏旧金额，更改配置后不显示旧值', async () => {
    const wrapper = mount(UpstreamBalanceCell, {
      props: { account: account({ ...snapshot, status: 'failed' }), now, globalProbeEnabled: false }
    })
    expect(wrapper.text()).toContain('$0.00')
    expect(wrapper.text()).toContain('upstreamBalance.lastSuccess')
    expect(wrapper.text()).toContain('status.failed')
    expect(wrapper.get('button').attributes('disabled')).toBeUndefined()
    await wrapper.setProps({ account: account({ ...snapshot, status: 'unavailable' }) })
    expect(wrapper.get('[data-testid="balance-amount"]').text()).toBe('—')
    await wrapper.setProps({ account: { ...account(snapshot), extra: { upstream_balance_probe_type: 'key_total', upstream_balance_probe: snapshot } } })
    expect(wrapper.text()).not.toContain('$0.00')
    expect(wrapper.text()).toContain('types.key_total')
  })

  it('使用列表时钟标记过期，默认关闭时不请求上游', async () => {
    const wrapper = mount(UpstreamBalanceCell, { props: { account: account(snapshot), now } })
    expect(wrapper.text()).toContain('upstreamBalance.updated')
    await wrapper.setProps({ now: now + 3600_000 })
    expect(wrapper.text()).toContain('upstreamBalance.lastSuccess')
    await wrapper.setProps({ account: { ...account(), extra: {} } })
    expect(wrapper.find('button').exists()).toBe(false)
  })
})
