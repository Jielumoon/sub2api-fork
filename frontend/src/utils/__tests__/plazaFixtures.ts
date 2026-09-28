import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import type { UserPricingInterval, UserSupportedModelPricing } from '@/api/channels'

/** Claude Sonnet 口径：输入 $3 / 输出 $15 / 缓存写 $3.75 / 缓存读 $0.30（官方另有 1h 写 $6）。 */
export function tokenModel(overrides: Partial<PlazaModel> = {}): PlazaModel {
  return {
    name: 'claude-sonnet',
    platform: 'anthropic',
    pricing: {
      billing_mode: 'token',
      input_price: 3e-6,
      output_price: 1.5e-5,
      cache_write_price: 3.75e-6,
      cache_read_price: 3e-7,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: []
    },
    official_pricing: {
      input_price: 3e-6,
      output_price: 1.5e-5,
      cache_write_price: 3.75e-6,
      cache_write_1h_price: 6e-6,
      cache_read_price: 3e-7
    },
    ...overrides
  }
}

export function requestPricing(overrides: Partial<UserSupportedModelPricing> = {}): UserSupportedModelPricing {
  return {
    billing_mode: 'per_request',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: [],
    ...overrides
  }
}

export function requestTier(label: string, price: number): UserPricingInterval {
  return {
    min_tokens: 0,
    max_tokens: null,
    tier_label: label,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    per_request_price: price
  }
}

/** 两档长上下文阶梯（≤272K / >272K）。 */
export function ladderIntervals(): UserPricingInterval[] {
  return [
    {
      min_tokens: 0,
      max_tokens: 272000,
      tier_label: '≤272K',
      input_price: 5e-6,
      output_price: 3e-5,
      cache_write_price: 6.25e-6,
      cache_read_price: 5e-7,
      per_request_price: null
    },
    {
      min_tokens: 272000,
      max_tokens: null,
      tier_label: '>272K',
      input_price: 1e-5,
      output_price: 4.5e-5,
      cache_write_price: 1.25e-5,
      cache_read_price: 1e-6,
      per_request_price: null
    }
  ]
}

export function ladderModel(overrides: Partial<PlazaModel> = {}): PlazaModel {
  return tokenModel({
    name: 'gpt-5.6-sol',
    platform: 'openai',
    pricing: {
      billing_mode: 'token',
      input_price: 5e-6,
      output_price: 3e-5,
      cache_write_price: 6.25e-6,
      cache_read_price: 5e-7,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: ladderIntervals()
    },
    official_pricing: {
      input_price: 5e-6,
      output_price: 3e-5,
      cache_write_price: 6.25e-6,
      cache_read_price: 5e-7,
      intervals: ladderIntervals()
    },
    long_context_basis: 'whole_request',
    ...overrides
  })
}

/** 夜间 0.5 倍、晚高峰 1.2 倍的分时模型。 */
export function timePricedModel(): PlazaModel {
  return tokenModel({
    name: 'deepseek-chat',
    platform: 'deepseek',
    time_pricing: {
      timezone: 'Asia/Shanghai',
      periods: [
        { start_time: '00:30', end_time: '08:30:00', multiplier: 0.5 },
        { start_time: '18:00', end_time: '22:00', multiplier: 1.2 }
      ]
    }
  })
}

export function plazaGroup(overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id: 1,
    name: 'default',
    description: '',
    platform: 'anthropic',
    subscription_type: 'standard',
    rate_multiplier: 1,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    long_context_pricing_enabled: true,
    models: [tokenModel()],
    ...overrides
  }
}
