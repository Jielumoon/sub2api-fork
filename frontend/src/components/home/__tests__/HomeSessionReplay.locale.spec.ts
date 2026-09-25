// mock 的 useI18n 和真实的一样返回 locale ref；测试里改它就等同于用户切换语言。
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const { ref } = await import('vue')
  const locale = ref<'zh' | 'en'>('zh')
  const messages: Record<string, Record<string, string>> = {
    zh: { 'home.replay.prompt': '改首页' },
    en: { 'home.replay.prompt': 'make the home page look better' },
  }
  return { ...actual, useI18n: () => ({ locale, t: (key: string) => messages[locale.value][key] ?? key }) }
})

import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import HomeSessionReplay from '../HomeSessionReplay.vue'

describe('HomeSessionReplay 切换语言', () => {
  // 全局 setup 的 matchMedia 视为"减少动效"，挂载即终帧。
  it('播放结束后切换语言，提示词完整显示而不是按旧长度截断', async () => {
    const wrapper = mount(HomeSessionReplay, {
      props: { baseUrl: 'https://api.example.com' },
      global: { stubs: { Icon: true, CountUp: true } },
    })
    await nextTick()
    expect(wrapper.get('[data-line="prompt"]').text()).toContain('改首页')

    useI18n().locale.value = 'en'
    await nextTick()
    expect(wrapper.get('[data-line="prompt"]').text()).toContain('make the home page look better')
  })
})
