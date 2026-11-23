import './HistoryPage.css'
import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, ArrowUpRight, Gift, ImageOff, Images, UserRound } from 'lucide-react'
import { Link, useSearchParams } from 'react-router'
import { Button } from '../../components/Button'
import { Modal } from '../../components/Modal'
import { generationApi } from '../../features/generation/api'
import type { GenerationTask } from '../../features/generation/types'
import { scenes, artStyles } from '../../features/generation/options'
import { useCurrentUser } from '../../features/user/useCurrentUser'
import { API_BASE_URL } from '../../lib/config'

function WorkImage({ task }: { task: GenerationTask }) {
  const [failed, setFailed] = useState(false)
  return <div className="work-image">{failed || !task.imageUrl
    ? <span><ImageOff size={25} />图片暂时无法读取，点击重试</span>
    : <img src={task.imageUrl} alt="生成的宠物表情图" loading="lazy" onError={() => setFailed(true)} />}</div>
}

function ProfileAvatar({ src }: { src?: string }) {
  const [failed, setFailed] = useState(false)
  return <span className="profile-avatar">{src && !failed
    ? <img src={src} alt="微信头像" referrerPolicy="no-referrer" onError={() => setFailed(true)} />
    : <UserRound size={28} />}</span>
}

export function HistoryPage() {
  const { user, error: userError, reload } = useCurrentUser()
  const [search] = useSearchParams()
  const [items, setItems] = useState<GenerationTask[]>([])
  const [cursor, setCursor] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)
  const [moreLoading, setMoreLoading] = useState(false)
  const [confirmSync, setConfirmSync] = useState(false)
  const moreRequest = useRef<AbortController | null>(null)
  const syncStatus = search.get('wechat_profile')
  const inWechat = /MicroMessenger/i.test(navigator.userAgent)

  useEffect(() => {
    const controller = new AbortController()
    generationApi.list(undefined, controller.signal).then(page => {
      if (!controller.signal.aborted) { setItems(page.items); setCursor(page.nextCursor); setError(''); setLoading(false) }
    }).catch((reason: unknown) => {
      if (!controller.signal.aborted) { setError(reason instanceof Error ? reason.message : '作品暂时无法读取'); setLoading(false) }
    })
    return () => { controller.abort(); moreRequest.current?.abort() }
  }, [attempt])

  async function loadMore() {
    if (!cursor || moreRequest.current) return
    const controller = new AbortController()
    moreRequest.current = controller
    setMoreLoading(true)
    setError('')
    try {
      const page = await generationApi.list(cursor, controller.signal)
      if (!controller.signal.aborted) {
        setItems(previous => [...previous, ...page.items.filter(item => !previous.some(old => old.id === item.id))])
        setCursor(page.nextCursor)
      }
    } catch (reason) {
      if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : '作品暂时无法读取')
    } finally {
      if (!controller.signal.aborted) setMoreLoading(false)
      if (moreRequest.current === controller) moreRequest.current = null
    }
  }

  return <main id="main-content" className="history-page">
    <Link className="back-link" to="/"><ArrowLeft size={16} /> 返回制作页</Link>
    <section className="profile-card" aria-label="我的账号">
      <ProfileAvatar key={user?.avatarUrl || 'default'} src={user?.avatarUrl} />
      <div className="profile-detail"><h2>{user?.nickname || '微信用户'}</h2>
        <p>{user ? <><Gift size={14} /> 剩余 {user.remainingCredits} 次 · <Link className="profile-credits" to="/credits">购买额度</Link></> : userError ? '账号暂时无法读取' : '正在读取账号…'}</p>
        {userError && <button type="button" className="profile-sync" onClick={reload}>重新读取账号</button>}
      </div>
      {user?.canSyncWechatProfile && <button type="button" className="profile-sync" onClick={() => setConfirmSync(true)}>{user.nickname || user.avatarUrl ? '更新微信资料' : '同步微信资料'}</button>}
    </section>
    <div className="history-pet-links"><Link to="/pets">我的宠物 <ArrowUpRight size={14} /></Link><Link to="/growth">成长轨迹 <ArrowUpRight size={14} /></Link></div>
    {syncStatus === 'synced' && <p className="profile-notice" role="status">微信昵称和头像已同步。</p>}
    {syncStatus === 'skipped' && <p className="profile-notice" role="status">已跳过资料授权，你仍可正常使用。</p>}
    {syncStatus === 'failed' && <p className="profile-notice" role="status">资料暂未同步，原账号和作品已保留，可以稍后再试。</p>}
    <div className="history-heading"><div><span className="eyebrow"><span /> 把可爱收藏起来</span><h1>我的作品</h1><p>每一次小剧场，都在这里。</p></div><Link className="button button--primary" to="/">再做一张 <ArrowUpRight size={17} /></Link></div>
    {loading ? <p className="history-status" role="status">正在翻找毛孩的小剧场…</p>
      : items.length > 0 ? <div className="history-grid">{items.map(task => <Link className="work-card" key={task.id} to={`/generations/${encodeURIComponent(task.id)}`}>
        <WorkImage task={task} />
        <div className="work-caption"><h2>{scenes.find(scene => scene.id === task.scene)?.title || '毛孩的小剧场'}</h2><p>{artStyles.find(style => style.id === task.style)?.title || '宠物表情图'}</p>
          {task.petName && <p>{task.petName}{task.petAge ? ` · ${task.petAge}` : ''}</p>}
          {task.createdAt && <time dateTime={task.createdAt}>{new Date(task.createdAt).toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</time>}</div>
      </Link>)}</div>
        : !error && <section className="history-empty"><Images size={42} /><h2>第一份可爱，等你来收藏</h2><p>成功生成的图片会自动出现在这里。</p><Link className="button button--secondary" to="/">去制作第一张</Link></section>}
    {error && <div className="history-error" role="alert"><p>{error}</p><Button variant="secondary" onClick={() => { if (items.length) { void loadMore() } else { setLoading(true); setAttempt(value => value + 1) } }}>重新读取</Button></div>}
    {!loading && cursor && <div className="history-more"><Button variant="secondary" busy={moreLoading} onClick={() => { void loadMore() }}>查看更多作品</Button></div>}
    {!loading && !error && items.length > 0 && !cursor && <p className="history-status">已经看完所有可爱啦</p>}
    {confirmSync && <Modal title="同步微信资料" onClose={() => setConfirmSync(false)}>
      <p className="profile-consent">同意后将获取并保存你的微信昵称和头像，用于账号展示与用户管理。取消授权不影响生成图片、免费次数和历史作品。</p>
      {inWechat ? <a className="button button--primary" href={`${API_BASE_URL}/auth/wechat/profile/start?return_to=%2Fhistory`}>前往微信授权</a> : <p>请在微信内打开当前页面后同步。</p>}
      <button type="button" className="text-link profile-skip" onClick={() => setConfirmSync(false)}>暂不同步，继续使用</button>
      <Link className="text-link" to="/privacy">查看隐私说明</Link>
    </Modal>}
  </main>
}
