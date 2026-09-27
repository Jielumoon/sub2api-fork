/** 模型广场各视图共用的文案型标签（依赖 i18n，所以不放进纯计价工具）。 */
import { BILLING_MODE_IMAGE } from '@/constants/channel'
import type { PlazaModel } from '@/api/modelPlaza'
import type { PlazaOffer } from '@/utils/plazaCatalog'
import { billingMode, isTokenBilled, reasoningEffortMultipliers, timePeriods, tokenIntervals } from '@/utils/plazaPricing'

type Translate = (key: string, params?: Record<string, unknown>) => string

/** 非 token 计费的模式标签：按图片计费 / 按次计费。 */
export function billingModeLabel(model: PlazaModel, t: Translate): string {
  return billingMode(model) === BILLING_MODE_IMAGE ? t('modelPlaza.table.perImage') : t('modelPlaza.table.perRequest')
}

/** 非 token 计费的单位后缀：按图片 → “/ 张”，按次 → “/ 次”。 */
export function plazaUnitSuffix(offer: PlazaOffer, t: Translate): string {
  return billingMode(offer.model) === BILLING_MODE_IMAGE
    ? t('modelPlaza.table.perUnitImage')
    : t('modelPlaza.table.perUnitRequest')
}

/** 卡片 / 表格上的概要徽章：计费模式（非 token）、阶梯、分时、思考等级倍率。 */
export function plazaBadges(offers: PlazaOffer[], t: Translate): string[] {
  const models = offers.map((o) => o.model)
  const badges: string[] = []
  const nonToken = models.find((m) => !isTokenBilled(m))
  if (nonToken && models.every((m) => !isTokenBilled(m))) badges.push(billingModeLabel(nonToken, t))
  if (models.some((m) => isTokenBilled(m) && tokenIntervals(m).length > 1)) badges.push(t('modelPlaza.badges.tiered'))
  if (models.some((m) => timePeriods(m).length > 0)) badges.push(t('modelPlaza.badges.timePricing'))
  if (models.some((m) => reasoningEffortMultipliers(m).length > 0)) badges.push(t('modelPlaza.badges.reasoning'))
  return badges
}

/**
 * 折扣文案：中文习惯「3.5折」，英文习惯「65% off」。两个值都传进去，由各语言文案挑自己要的那个。
 * 保底 0.1 折 / 最多 99% off，避免极端比值显示成「0折」「100% off」。
 */
export function discountText(ratio: number, t: Translate): string {
  const zhe = Math.max(0.1, Math.round(ratio * 100) / 10)
  const percent = Math.min(99, Math.max(1, Math.round((1 - ratio) * 100)))
  return t('modelPlaza.discount.off', { zhe, percent })
}
