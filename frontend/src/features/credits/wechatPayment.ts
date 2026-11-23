import type { PayParams } from './api'

interface Bridge { invoke(method: string, params: PayParams, callback: (result: {err_msg?: string}) => void): void }
declare global { interface Window { WeixinJSBridge?: Bridge } }

export function invokeWechatPayment(params: PayParams): Promise<'returned' | 'cancelled'> {
  return new Promise((resolve, reject) => {
    if (!/MicroMessenger/i.test(navigator.userAgent)) { reject(new Error('请在微信内打开本页面完成付款。')); return }
    let finished = false
    const finish = (error?: Error, cancelled = false) => {
      if (finished) return
      finished = true
      window.clearTimeout(timeout)
      document.removeEventListener('WeixinJSBridgeReady', invoke)
      if (error) reject(error)
      else resolve(cancelled ? 'cancelled' : 'returned')
    }
    const invoke = () => {
      if (!window.WeixinJSBridge || finished) return
      document.removeEventListener('WeixinJSBridgeReady', invoke)
      try {
        window.WeixinJSBridge.invoke('getBrandWCPayRequest', params, result => {
          if (result.err_msg === 'get_brand_wcpay_request:cancel') finish(undefined, true)
          else if (result.err_msg === 'get_brand_wcpay_request:ok') finish()
          else finish(new Error('付款结果尚未确认，请查询原订单，避免重复购买。'))
        })
      } catch { finish(new Error('微信收银台暂时无法打开，请稍后查询原订单。')) }
    }
    // A bank/WeChat UI may remain open for minutes. Timeout means unknown, never paid.
    const timeout = window.setTimeout(() => finish(new Error('付款结果尚未确认，请查询原订单。')), 180_000)
    if (window.WeixinJSBridge) invoke()
    else document.addEventListener('WeixinJSBridgeReady', invoke)
  })
}
