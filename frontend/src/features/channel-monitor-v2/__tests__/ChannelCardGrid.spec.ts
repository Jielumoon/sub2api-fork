const i18nT = (key: string, params?: Record<string, unknown>) => {
  const map: Record<string, string> = {
    'channelMonitorV2.cards.recent': '近 {n} 个区间',
    'channelMonitorV2.cards.rate': '倍率 {rate}x',
    'channelMonitorV2.cards.health.healthy': '健康',
    'channelMonitorV2.cards.health.warning': '需关注',
    'channelMonitorV2.cards.health.critical': '异常',
    'channelMonitorV2.cards.health.unknown': '无流量',
    'channelMonitorV2.metrics.successRate': '成功率',
    'channelMonitorV2.metrics.cacheRate': '缓存率',
    'channelMonitorV2.metrics.ttft': '首 Token',
    'channelMonitorV2.metrics.successRateValue': '成功率 {value}',
    'channelMonitorV2.metrics.cacheRateValue': '缓存率 {value}',
    'channelMonitorV2.metrics.ttftValue': '首 Token {value}',
    'channelMonitorV2.matrix.noTraffic': '无流量',
    'monitorCommon.nextUpdateIn': '{n}s 后刷新',
    'monitorCommon.providers.anthropic': 'Anthropic',
    'monitorCommon.providers.openai': 'OpenAI',
  }
  const template = map[key] || key
  return template.replace(/\{(\w+)\}/g, (_, name) => String(params?.[name] ?? ''))
}

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: i18nT, te: () => true, locale: { value: 'zh' } }) }
})

import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ChannelCardGrid from '../ChannelCardGrid.vue'
import ChannelCard from '../ChannelCard.vue'
import { alignRows, bucketStartsForCoverage } from '../monitorBuckets'
import type { MonitorCoverage, MonitorHealth, MonitorMatrixRow, MonitorMetric } from '@/api/channelMonitorV2'

const health: MonitorHealth = {
  overall: 'healthy',
  error_rate: 'healthy',
  ttft: 'healthy',
  cache: 'healthy',
  score: 95,
  error_rate_score: 100,
  ttft_score: 90,
  cache_score: 80,
  minimum_sample: 50,
}

function metrics(errorRate: number): MonitorMetric {
  return {
    success_requests: 0,
    error_requests: 0,
    request_count: 0,
    token_count: 0,
    rpm: 0,
    tpm: 0,
    error_rate: errorRate,
    cache_rate: 0.8,
    cache_rate_numerator: 0,
    cache_rate_denominator: 0,
    ttft: { sample_count: 0, p50_ms: 1500, p95_ms: 3000, avg_ms: 1600 },
    duration: { sample_count: 0, p50_ms: 3000, p95_ms: 6000, avg_ms: 3200 },
  }
}

// 90m 档：5 分钟一块，requested_start 对齐后正好 18 块。
const coverage: MonitorCoverage = {
  requested_start: '2026-09-25T07:35:00Z',
  requested_end: '2026-09-25T09:05:00Z',
  coverage_start: '2026-09-25T07:35:00Z',
  data_through: '2026-09-25T09:02:00Z',
  computed_at: '2026-09-25T09:02:00Z',
  aggregation_lag_seconds: 0,
  coverage_complete: true,
  bucket_seconds: 300,
}

const rows: MonitorMatrixRow[] = [
  {
    platform: 'openai',
    group_id: 7,
    group_name: 'ChatGPT-Plus',
    metrics: metrics(0.02),
    health,
    buckets: [{ bucket_start: '2026-09-25T08:55:00Z', metrics: metrics(0.02), health }],
  },
  {
    platform: 'anthropic',
    group_id: 8,
    group_name: 'Claude-Max',
    metrics: metrics(0),
    health: { ...health, overall: 'unknown', score: null },
    buckets: [],
  },
  {
    platform: 'openai',
    group_id: 9,
    group_name: 'ChatGPT-Pro',
    metrics: metrics(0.1),
    health,
    buckets: [{ bucket_start: '2026-09-25T07:35:00Z', metrics: metrics(0.1), health }],
  },
]

describe('monitorBuckets', () => {
  it('按 coverage 补齐 90m 档的 18 个时间槽，并把稀疏桶对位', () => {
    const starts = bucketStartsForCoverage(coverage)
    expect(starts).toHaveLength(18)
    expect(starts[0]).toBe('2026-09-25T07:35:00.000Z')
    expect(starts[17]).toBe('2026-09-25T09:00:00.000Z')

    const [{ slots }] = alignRows([rows[0]], starts)
    expect(slots.filter((slot) => slot.bucket)).toHaveLength(1)
    expect(slots[16].bucket?.bucket_start).toBe('2026-09-25T08:55:00Z')
  })
})

describe('ChannelCardGrid', () => {
  function cardByName(name: string) {
    const card = mountGrid().findAll('article').find((item) => item.find('h3').text() === name)
    if (!card) throw new Error(`card ${name} not found`)
    return card
  }

  function mountGrid() {
    return mount(ChannelCardGrid, {
      props: { rows, coverage, healthMode: 'overall', rates: { 7: 0.05 }, countdownSeconds: 42 },
    })
  }

  it('按平台分组，保持后端顺序并显示数量', () => {
    const headers = mountGrid().findAll('section > header')
    expect(headers.map((h) => h.text())).toEqual(['OpenAI2', 'Anthropic1'])
  })

  it('每张卡片 18 个色块，倍率只在拿到时显示，倒计时进入色块条标题', () => {
    expect(mountGrid().findAll('article')).toHaveLength(3)
    const plus = cardByName('ChatGPT-Plus')
    expect(plus.findAll('.strip-block')).toHaveLength(18)
    expect(plus.text()).toContain('倍率 0.05x')
    expect(plus.text()).toContain('近 18 个区间')
    expect(plus.text()).toContain('42s 后刷新')
    expect(cardByName('ChatGPT-Pro').text()).not.toContain('倍率')
  })

  it('倒计时每秒变化时不重新对位时间槽', async () => {
    const wrapper = mountGrid()
    const before = wrapper.findComponent(ChannelCard).props('slots')
    await wrapper.setProps({ countdownSeconds: 41 })
    expect(wrapper.findComponent(ChannelCard).props('slots')).toBe(before)
    expect(wrapper.text()).toContain('41s 后刷新')
  })

  it('色块使用共享健康色板类，并给读屏器汇总状态', () => {
    const plus = cardByName('ChatGPT-Plus')
    const classes = plus.findAll('.strip-block').map((block) => block.classes().find((c) => c.startsWith('health-')))
    expect(classes.filter((c) => c === 'health-unknown')).toHaveLength(17)
    expect(classes[16]).toMatch(/^health-score\d+$/)
    expect(plus.find('[role="img"]').attributes('aria-label')).toBe('近 18 个区间 (无流量 17, 健康 1)')
  })

  it('整个窗口没有流量时指标显示 -，徽章为无流量', () => {
    const card = cardByName('Claude-Max')
    expect(card.text()).toContain('无流量')
    const values = card.findAll('.font-mono.text-lg').map((cell) => cell.text())
    expect(values).toEqual(['-', '-', '-'])
  })
})
