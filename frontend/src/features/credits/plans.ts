export type CreditPlanId = 'month' | 'year'

export interface CreditPlan {
  id: CreditPlanId
  name: string
  priceYuan: number
  credits: number
  validityLabel: string
  description: string
}

// Fallback display while loading, and mock preview only. Live prices/credits come from the server catalog.
export const creditPlans: CreditPlan[] = [
  { id: 'month', name: '月卡', priceYuan: 4.8, credits: 4, validityLabel: '1个月', description: '陪毛孩记录一个月的小剧场' },
  { id: 'year', name: '年卡', priceYuan: 58, credits: 48, validityLabel: '1年', description: '把一整年的可爱慢慢收藏' },
]
