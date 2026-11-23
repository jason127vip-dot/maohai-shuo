import { request } from '../../lib/request'
import { IS_MOCK } from '../../lib/config'
import { creditPlans } from './plans'

export interface Plan { id: string; name: string; amountFen: number; credits: number; months: number }
export interface Catalog { enabled: boolean; plans: Plan[] }
export interface PaymentOrder {
  id: string; planId: string; planName: string; amountFen: number; credits: number; months: number
  status: 'pending' | 'paid' | 'closed'; createdAt: string; expiresAt: string; paidAt?: string
}
export interface Entitlement { id: string; planName: string; total: number; remaining: number; startsAt: string; expiresAt: string }
export interface PayParams { appId: string; timeStamp: string; nonceStr: string; package: string; signType: string; paySign: string }
export const paymentApi = {
  catalog: (signal?: AbortSignal): Promise<Catalog> => IS_MOCK ? Promise.resolve({ enabled: false, plans: creditPlans.map(p => ({ id: p.id, name: p.name, amountFen: Math.round(p.priceYuan * 100), credits: p.credits, months: p.id === 'month' ? 1 : 12 })) }) : request('/payments/catalog', { signal }),
  orders: (signal?: AbortSignal): Promise<{items: PaymentOrder[]}> => IS_MOCK ? Promise.resolve({items: []}) : request('/payments/orders', { signal }),
  entitlements: (signal?: AbortSignal): Promise<{items: Entitlement[]; serverTime: string}> => IS_MOCK ? Promise.resolve({items: [], serverTime: new Date().toISOString()}) : request('/payments/entitlements', { signal }),
  create: (planId: string, key: string) => request<PaymentOrder>('/payments/orders', {method: 'POST', headers: {'Idempotency-Key': key}, body: JSON.stringify({planId})}),
  pay: (id: string) => request<{order: PaymentOrder; payParams: PayParams | null}>(`/payments/orders/${encodeURIComponent(id)}/pay`, {method: 'POST'}),
  refresh: (id: string) => request<PaymentOrder>(`/payments/orders/${encodeURIComponent(id)}/refresh`, {method: 'POST'}),
}
