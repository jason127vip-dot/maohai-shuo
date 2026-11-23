import './CreditsPage.css'
import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowLeft, ArrowRight, CalendarDays, Check, Gift, Images, PawPrint } from 'lucide-react'
import { Link } from 'react-router'
import { Button } from '../../components/Button'
import { creditPlans } from '../../features/credits/plans'
import { paymentApi, type Catalog, type Entitlement, type PaymentOrder } from '../../features/credits/api'
import { invokeWechatPayment } from '../../features/credits/wechatPayment'
import { createRandomId } from '../../lib/uuid'
import { useCurrentUser } from '../../features/user/useCurrentUser'

export function CreditsPage() {
  const { user, error, reload } = useCurrentUser()
  const [selectedPlan, setSelectedPlan] = useState('month')
  const [catalog, setCatalog] = useState<Catalog | null>(null)
  const [packages, setPackages] = useState<Entitlement[]>([])
  const [orders, setOrders] = useState<PaymentOrder[]>([])
  const [shopError, setShopError] = useState('')
  const [historyError, setHistoryError] = useState('')
  const [historyReady, setHistoryReady] = useState(false)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [serverOffset, setServerOffset] = useState(0)
  const [now, setNow] = useState(() => Date.now())
  const creating = useRef(false)
  const keys = useRef<Record<string, string>>({})
  const alive = useRef(true)
  const plans = catalog?.plans.map(plan => ({...plan, priceYuan: plan.amountFen / 100, validityLabel: plan.months === 12 ? '1年' : '1个月', description: creditPlans.find(p => p.id === plan.id)?.description || ''})) || creditPlans
  const selected = plans.find(plan => plan.id === selectedPlan) || plans[0]
  const loadHistory = useCallback(async () => {
    const [orderResult, packageResult] = await Promise.all([paymentApi.orders(), paymentApi.entitlements()])
    if (!alive.current) return
    setOrders(orderResult.items)
    setPackages(packageResult.items)
    setServerOffset(Date.parse(packageResult.serverTime) - Date.now())
    setHistoryError('')
    setHistoryReady(true)
  }, [])
  const loadCatalog = useCallback(() => {
    paymentApi.catalog().then(value => {if (alive.current) {setCatalog(value); setShopError('')}}).catch((e: unknown) => {if (alive.current) {setCatalog(null); setShopError(e instanceof Error ? e.message : '套餐读取失败')}})
  }, [])
  useEffect(() => {
    alive.current = true
    loadCatalog()
    loadHistory().catch(() => {if (alive.current) {setHistoryReady(false); setHistoryError('订单与套餐暂时无法读取，请重试。')}})
    return () => {alive.current = false}
  }, [loadCatalog, loadHistory])
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 10_000)
    return () => window.clearInterval(timer)
  }, [])
  const syncHistory = useCallback(async () => {
    reload()
    try {await loadHistory()} catch {if (alive.current) {setHistoryReady(false); setHistoryError('订单与套餐暂时无法读取，请重试。')}}
  }, [reload, loadHistory])
  const showOrder = useCallback((order: PaymentOrder) => {
    if (!alive.current) return
    setOrders(items => [order, ...items.filter(item => item.id !== order.id)].slice(0, 20))
    if (order.status === 'paid') setMessage('付款已确认，套餐已到账，可以继续创作了。')
    else if (order.status === 'closed') setMessage('订单已关闭，可以重新选择套餐。')
    else setMessage('订单尚未确认付款。若已扣款，请稍后查询原订单，避免重复购买。')
  }, [])
  const checkOrder = useCallback(async (id: string) => {
    if (creating.current) return
    creating.current = true; setBusy(true); setMessage('正在向微信查询付款结果…')
    try {showOrder(await paymentApi.refresh(id)); await syncHistory()}
    catch (e) {setMessage(e instanceof Error ? e.message : '付款结果暂时无法确认，请稍后查询。')}
    finally {creating.current = false; if (alive.current) setBusy(false)}
  }, [showOrder, syncHistory])
  const buy = async (existing?: PaymentOrder) => {
    if (creating.current || !catalog?.enabled || !selected) return
    if (!/MicroMessenger/i.test(navigator.userAgent)) {setMessage('请在微信内打开本页面完成付款。'); return}
    creating.current = true; setBusy(true); setMessage('正在准备微信付款…')
    let order = existing || orders.find(item => item.planId === selected.id && item.status === 'pending' && Date.parse(item.expiresAt) > Date.now() + serverOffset)
    try {
      if (!order) {
        const storageKey = `maohai-payment-request:${selected.id}`
        let key = keys.current[selected.id]
        try {key ||= sessionStorage.getItem(storageKey) || ''} catch { /* Storage can be disabled. */ }
        key ||= createRandomId()
        keys.current[selected.id] = key
        try {sessionStorage.setItem(storageKey, key)} catch { /* In-memory retry remains available. */ }
        order = await paymentApi.create(selected.id, key)
        showOrder(order)
      }
      // Also clear a recovered create request after finding its durable order in history.
      delete keys.current[order.planId]
      try {sessionStorage.removeItem(`maohai-payment-request:${order.planId}`)} catch { /* No storage requirement. */ }
      const result = await paymentApi.pay(order.id)
      order = result.order
      showOrder(order)
      if (result.payParams && order.status === 'pending') {
        const returned = await invokeWechatPayment(result.payParams)
        if (!alive.current) return
        if (returned === 'cancelled') setMessage('已取消付款，订单仍保留，可以稍后继续支付或查询。')
        else {setMessage('正在确认到账，请稍候…'); showOrder(await paymentApi.refresh(order.id))}
      }
      await syncHistory()
    } catch (e) {if (alive.current) setMessage(e instanceof Error ? e.message : '付款结果尚未确认，请查询原订单。')}
    finally {creating.current = false; if (alive.current) setBusy(false)}
  }
  // Poll only pending orders and only while visible; callbacks never credit the account.
  useEffect(() => {
    if (!catalog?.enabled || !orders.some(order => order.status === 'pending')) return
    const timer = window.setInterval(() => {
      if (document.visibilityState !== 'visible' || creating.current) return
      const pending = orders.find(order => order.status === 'pending')
      if (pending) void checkOrder(pending.id)
    }, 15_000)
    return () => window.clearInterval(timer)
  }, [catalog?.enabled, orders, checkOrder])

  return <main id="main-content" className="credits-page">
    <Link className="back-link" to="/"><ArrowLeft size={16} /> 返回制作页</Link>
    <div className="credits-heading">
      <span className="credits-paw" aria-hidden="true"><PawPrint size={28} /></span>
      <span className="eyebrow"><span /> 给可爱多一点空间</span>
      <h1>让毛孩的小剧场继续</h1>
      <p>选一张月卡或年卡，继续记录毛孩的可爱。</p>
    </div>
    <section className="credits-balance" aria-label="当前额度">
      <Gift size={20} aria-hidden="true" />
      <div>{user ? <><strong>剩余 {user.remainingCredits} 次</strong><span>{user.remainingCredits <= 0 ? '当前额度已用完，可以在这里查看套餐。' : '还可以继续创作，也可以提前看看套餐。'}</span></>
        : error ? <><strong>额度暂时无法读取</strong><span role="alert">{error}</span></> : <span role="status">正在读取额度…</span>}</div>
      {error && <button type="button" className="text-link" onClick={reload}>重新读取</button>}
    </section>
    <section className="credits-shop" aria-labelledby="credits-shop-title">
      <h2 id="credits-shop-title">选择你的套餐</h2>
      <fieldset className="credits-plans">
        <legend className="visually-hidden">选择月卡或年卡</legend>
        {plans.map(plan => <label key={plan.id} className={`credits-plan ${selectedPlan === plan.id ? 'is-selected' : ''}`}>
          <input type="radio" name="credit-plan" value={plan.id} checked={selectedPlan === plan.id} onChange={() => setSelectedPlan(plan.id)} />
          <span className="credits-plan-top"><strong>{plan.name}</strong><span className="credits-plan-check" aria-hidden="true">{selectedPlan === plan.id && <Check size={14} />}</span></span>
          <span className="credits-price"><span>¥</span>{plan.priceYuan}<small>/ 张</small></span>
          <span className="credits-allowance"><Images size={17} aria-hidden="true" /><strong>{plan.credits} 次生成</strong></span>
          <span className="credits-validity"><CalendarDays size={14} aria-hidden="true" />购买后{plan.validityLabel}内有效</span>
          <span className="credits-plan-description">{plan.description}</span>
        </label>)}
      </fieldset>
      <div className="credits-checkout" aria-live="polite">
        <div className="credits-order"><div><span>已选套餐</span><strong>{selected.name}</strong><small>{selected.credits}次生成 · {selected.validityLabel}内有效</small></div>
          <strong className="credits-total"><small>¥</small>{selected.priceYuan}</strong></div>
        <Button disabled={!catalog?.enabled || !user || !!error || !historyReady || !!historyError} busy={busy} className="credits-buy" onClick={() => void buy()}>{catalog?.enabled ? '微信支付' : '微信支付 · 暂未开放'}</Button>
        {shopError ? <p role="alert" className="credits-payment-note">{shopError} <button type="button" className="text-link" onClick={loadCatalog}>重新读取套餐</button></p> : <p className="credits-payment-note">{catalog?.enabled ? '请在微信内完成付款，到账后自动增加套餐次数。' : '微信支付暂未开放，敬请期待。'}</p>}
        {message && <p role="status" className="credits-payment-status">{message}</p>}
      </div>
    </section>
    <section className="credits-owned" aria-label="我的套餐和订单">
      <h2>我的套餐</h2>
      {historyError && <p role="alert">{historyError} <button type="button" className="text-link" disabled={busy} onClick={() => void syncHistory()}>重新读取</button></p>}
      {!historyReady && !historyError && <p role="status">正在读取套餐和订单…</p>}
      {historyReady && !historyError && packages.length === 0 && <p>购买成功后，会在这里显示次数和到期时间。</p>}
      {packages.map(pack => <article key={pack.id}><div><strong>{pack.planName}</strong><span>{new Date(pack.expiresAt).toLocaleString('zh-CN', {timeZone: 'Asia/Shanghai', hour12: false})} 到期（北京时间）</span></div><strong>{Date.parse(pack.expiresAt) <= now + serverOffset ? '已到期' : `剩余 ${pack.remaining} / ${pack.total} 次`}</strong></article>)}
      {orders.length > 0 && <><h2>最近订单</h2>{orders.map(order => <article className="credits-payment-order" key={order.id}>
        <div><strong>{order.planName} · ¥{(order.amountFen / 100).toFixed(2)} · {order.status === 'paid' ? '已到账' : order.status === 'closed' ? '已关闭' : '待确认付款'}</strong><span>订单号：{order.id}</span></div>
        {order.status === 'pending' && <div className="credits-order-actions"><button type="button" disabled={busy || !catalog?.enabled} onClick={() => void checkOrder(order.id)}>查询结果</button>{Date.parse(order.expiresAt) > now + serverOffset && <button type="button" disabled={busy || !catalog?.enabled} onClick={() => void buy(order)}>继续支付</button>}</div>}
      </article>)}</>}
    </section>
    <section className="credits-guide" aria-labelledby="credits-guide-title">
      <h2 id="credits-guide-title">套餐怎么使用？</h2>
      <div><Images size={19} aria-hidden="true" /><p><strong>每次生成，使用1次额度</strong><span>月卡包含{plans.find(plan => plan.id === 'month')?.credits}次，年卡包含{plans.find(plan => plan.id === 'year')?.credits}次，可在各自有效期内使用。</span></p></div>
      <div><CalendarDays size={19} aria-hidden="true" /><p><strong>付款成功后开始计算有效期</strong><span>月卡有效期为1个月，年卡有效期为1年；每次购买独立计算，到期后未使用的次数失效。按北京时间计算自然月／年，月末取对应月最后一天。</span></p></div>
      <div><Images size={19} aria-hidden="true" /><p><strong>先用快到期的套餐，再用免费次数</strong><span>生成失败退回原套餐；套餐已到期时，退回的次数也不能继续使用。</span></p></div>
    </section>
    <Link className="credits-history" to="/history">先去看看收藏的可爱 <ArrowRight size={15} /></Link>
  </main>
}
