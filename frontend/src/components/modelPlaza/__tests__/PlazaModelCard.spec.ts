import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PlazaModelCard from '../PlazaModelCard.vue'
import PlazaOfferPricingTable from '../PlazaOfferPricingTable.vue'
import { EMPTY_FILTERS, buildPlazaCatalog } from '@/utils/plazaCatalog'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import { plazaGroup, requestPricing, requestTier, tokenModel } from '@/utils/__tests__/plazaFixtures'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}(${Object.values(params).join(',')})` : key)
    })
  }
})
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: null }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: ref(false), copyToClipboard: vi.fn() }) }))

const STUBS = { transition: true, 'transition-group': true }

function mountCard(groups: ModelPlazaGroup[]) {
  const [entry] = buildPlazaCatalog(groups)
  return mount(PlazaModelCard, { props: { entry, filters: EMPTY_FILTERS }, global: { stubs: STUBS } })
}

const originals = (wrapper: ReturnType<typeof mountCard>) =>
  wrapper.findAll('[data-original-price]').map((el) => el.text().replace('modelPlaza.price.original', ''))

describe('PlazaModelCard 划线与印章', () => {
  it('输入打折、输出加价：只划掉输入原价，输出原价不出现，也不盖统一折数', () => {
    const wrapper = mountCard([
      plazaGroup({ models: [tokenModel({ pricing: { ...tokenModel().pricing!, input_price: 1.5e-6, output_price: 3e-5 } })] })
    ])
    expect(wrapper.findAll('[data-paid-price]').map((el) => el.text())).toEqual(['$1.50', '$30.00'])
    expect(originals(wrapper)).toEqual(['$3.00'])
    expect(wrapper.find('[data-stamp]').exists()).toBe(false)
  })

  it('折扣一致时两项都划线并盖章', () => {
    const wrapper = mountCard([plazaGroup({ rate_multiplier: 0.5, models: [tokenModel()] })])
    expect(originals(wrapper)).toEqual(['$3.00', '$15.00'])
    expect(wrapper.get('[data-stamp]').text()).toContain('modelPlaza.discount.off(5,50)')
  })
})

describe('PlazaModelCard 分组数', () => {
  it('Composite 分组里同名模型的两个平台报价只算一个分组，不显示「+1 个分组」', () => {
    const wrapper = mountCard([
      plazaGroup({ id: 9, platform: 'composite', models: [tokenModel({ name: 'shared' }), tokenModel({ name: 'shared', platform: 'openai' })] })
    ])
    expect(wrapper.text()).not.toContain('modelPlaza.card.moreGroups')
  })

  it('真的有别的分组时照常显示', () => {
    const wrapper = mountCard([
      plazaGroup({ id: 1, rate_multiplier: 0.5, models: [tokenModel()] }),
      plazaGroup({ id: 2, name: 'b', models: [tokenModel()] })
    ])
    expect(wrapper.text()).toContain('modelPlaza.card.moreGroups(1)')
  })
})

describe('按次 / 按图单价档展示', () => {
  const offerTable = (pricing: ReturnType<typeof requestPricing>) => {
    const model = tokenModel({ name: 'img', official_pricing: null, pricing })
    return mount(PlazaOfferPricingTable, {
      props: { offers: [{ group: plazaGroup({ models: [model] }), model }] },
      global: { stubs: { GroupBadge: true } }
    })
  }

  it('尺寸档全覆盖时不列默认价；卡片最低价也不用它', () => {
    const pricing = requestPricing({
      billing_mode: 'image',
      per_request_price: 0.01,
      intervals: [requestTier('1K', 0.1), requestTier('2K', 0.2), requestTier('4K', 0.3)]
    })
    const chips = offerTable(pricing).findAll('[data-unit-tier]').map((el) => el.text())
    expect(chips).toHaveLength(3)
    expect(chips.join(' ')).not.toContain('$0.01')
    expect(mountCard([plazaGroup({ models: [tokenModel({ name: 'img', official_pricing: null, pricing })] })]).get('[data-paid-price]').text()).toBe('$0.10')
  })

  it('尺寸档没配全时默认价记为「其他」一档，和卡片最低价对得上', () => {
    const pricing = requestPricing({ billing_mode: 'image', per_request_price: 0.01, intervals: [requestTier('1K', 0.1)] })
    const chips = offerTable(pricing).findAll('[data-unit-tier]').map((el) => el.text())
    expect(chips).toEqual(['1K $0.10modelPlaza.table.perUnitImage', 'modelPlaza.table.otherTier $0.01modelPlaza.table.perUnitImage'])
  })
})
