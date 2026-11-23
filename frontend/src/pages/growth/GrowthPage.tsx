import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, ArrowRight, CalendarDays, ImageOff, NotebookPen, Plus } from 'lucide-react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { Button } from '../../components/Button'
import { generationApi } from '../../features/generation/api'
import type { GenerationTask } from '../../features/generation/types'
import { scenes } from '../../features/generation/options'
import { useCreationDraft } from '../../features/generation/useCreationDraft'
import { usePets } from '../../features/pets/usePets'
import { petAvatars, type Pet } from '../../features/pets/types'
import '../pets/PetsPage.css'
import './GrowthPage.css'

function GrowthEntry({ task, pets, onAssigned }: { task: GenerationTask; pets: Pet[]; onAssigned(): void }) {
  const [note, setNote] = useState(task.note || '')
  const [draft, setDraft] = useState(note)
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [failedImage, setFailedImage] = useState(false)
  const [assigning, setAssigning] = useState('')
  const [petId, setPetId] = useState('')

  async function saveNote() {
    if (saving) return
    setSaving(true); setError('')
    try { const saved = await generationApi.updateNote(task.id, draft.trim()); setNote(saved.note || ''); setEditing(false) }
    catch (reason) { setError(reason instanceof Error ? reason.message : '保存失败，请重试。') }
    finally { setSaving(false) }
  }
  async function assign() {
    if (!petId || assigning) return
    setAssigning(petId); setError('')
    try { await generationApi.assignPet(task.id, petId); onAssigned() }
    catch (reason) { setError(reason instanceof Error ? reason.message : '关联失败，请重试。') }
    finally { setAssigning('') }
  }

  return <article className="growth-entry">
    <div className="growth-entry-date"><span className="growth-dot" /><time dateTime={task.createdAt}>{task.createdAt ? new Date(task.createdAt).toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', timeZone: 'UTC' }) : '日期未记录'}</time><span>{task.petAge || (task.petId ? '当时年龄未设置' : '未关联宠物')}</span></div>
    <div className="growth-entry-card">
      <Link className="growth-image" to={`/generations/${encodeURIComponent(task.id)}`} aria-label="查看这张成长作品">{task.imageUrl && !failedImage ? <img src={task.imageUrl} loading="lazy" alt={`${task.petName || '毛孩'}的成长作品`} onError={() => setFailedImage(true)} /> : <span><ImageOff size={28} />图片暂时无法读取，点击查看</span>}</Link>
      <div className="growth-entry-detail"><span className="growth-scene">{scenes.find(scene => scene.id === task.scene)?.title || '毛孩的小剧场'}</span><h3>{task.petName ? `${task.petName}的这一天` : '给这份可爱找个归属'}</h3>
        {editing ? <div className="growth-note-editor"><label htmlFor={`note-${task.id}`}>写一句成长记录</label><textarea id={`note-${task.id}`} value={draft} maxLength={200} rows={3} disabled={saving} placeholder="比如：今天第一次去海边" onChange={event => setDraft(event.target.value)} /><div><Button busy={saving} onClick={() => { void saveNote() }}>保存记录</Button><button type="button" className="text-link" disabled={saving} onClick={() => { setEditing(false); setDraft(note) }}>取消</button></div></div>
          : <><p className={`growth-note ${note ? '' : 'is-empty'}`}>{note || '留下一句话，记住这一刻的小事。'}</p><button className="growth-note-button" type="button" onClick={() => { setDraft(note); setEditing(true) }}><NotebookPen size={14} />{note ? '编辑记录' : '写一句记录'}</button></>}
        {!task.petId && (pets.length ? <div className="growth-assign"><label className="visually-hidden" htmlFor={`assign-${task.id}`}>选择作品所属宠物</label><select id={`assign-${task.id}`} value={petId} disabled={Boolean(assigning)} onChange={event => setPetId(event.target.value)}><option value="">选择所属宠物</option>{pets.map(pet => <option key={pet.id} value={pet.id}>{pet.name}</option>)}</select><Button variant="secondary" disabled={!petId} busy={Boolean(assigning)} onClick={() => { void assign() }}>关联</Button><p>关联后进入该宠物的成长轨迹，原图片保持原样。</p></div> : <Link className="text-link" to="/pets">先添加宠物 <ArrowRight size={14} /></Link>)}
        {error && <p className="error-message" role="alert">{error}</p>}
      </div>
    </div>
  </article>
}

export function GrowthPage() {
  const { pets, loading: petsLoading, error: petsError, reload: reloadPets } = usePets()
  const [search, setSearch] = useSearchParams()
  const requestedPet = search.get('pet')
  const selected = requestedPet || pets.find(pet => pet.isDefault)?.id || pets[0]?.id || 'unassigned'
  const pet = pets.find(item => item.id === selected)
  const missingPet = !petsLoading && !petsError && selected !== 'unassigned' && !pet
  const [attempt, setAttempt] = useState(0)
  const key = `${selected}:${attempt}`
  const [snapshot, setSnapshot] = useState<{ key: string; items: GenerationTask[]; cursor: string; error: string }>({ key: '', items: [], cursor: '', error: '' })
  const [moreLoadingKey, setMoreLoadingKey] = useState<string | null>(null)
  const moreLoading = moreLoadingKey === key
  const moreRequest = useRef<AbortController | null>(null)
  const navigate = useNavigate()
  const { updateDraft } = useCreationDraft()
  const current = snapshot.key === key
  const items = current ? snapshot.items : []
  const loading = !current

  useEffect(() => {
    if (petsLoading || petsError || missingPet) return
    const controller = new AbortController()
    generationApi.list(undefined, controller.signal, selected).then(page => {
      if (!controller.signal.aborted) setSnapshot({ key, items: page.items, cursor: page.nextCursor, error: '' })
    }).catch((reason: unknown) => {
      if (!controller.signal.aborted) setSnapshot({ key, items: [], cursor: '', error: reason instanceof Error ? reason.message : '成长记录暂时无法读取' })
    })
    return () => { controller.abort(); moreRequest.current?.abort(); moreRequest.current = null }
  }, [key, selected, petsLoading, petsError, missingPet])

  async function loadMore() {
    if (!current || !snapshot.cursor || moreRequest.current) return
    const controller = new AbortController(); moreRequest.current = controller; setMoreLoadingKey(key)
    try {
      const page = await generationApi.list(snapshot.cursor, controller.signal, selected)
      if (!controller.signal.aborted) setSnapshot(old => old.key !== key ? old : { ...old, items: [...old.items, ...page.items.filter(item => !old.items.some(previous => previous.id === item.id))], cursor: page.nextCursor, error: '' })
    } catch (reason) {
      if (!controller.signal.aborted) setSnapshot(old => old.key !== key ? old : { ...old, error: reason instanceof Error ? reason.message : '更多记录暂时无法读取' })
    } finally { setMoreLoadingKey(previous => previous === key ? null : previous); if (moreRequest.current === controller) moreRequest.current = null }
  }

  const groups = new Map<string, GenerationTask[]>()
  for (const item of items) { const month = item.createdAt?.slice(0, 7) || '未记录日期'; groups.set(month, [...(groups.get(month) || []), item]) }

  return <main id="main-content" className="growth-page">
    <Link className="back-link" to="/"><ArrowLeft size={16} /> 返回制作页</Link>
    <div className="pets-heading"><div><span className="eyebrow"><span /> 把一起长大的日子，慢慢收藏</span><h1>成长轨迹</h1><p>每一张小剧场，都是毛孩生命里的一个片段。</p></div><Link className="button button--secondary" to="/pets">管理宠物</Link></div>
    {petsLoading ? <p className="history-status" role="status">正在读取宠物资料…</p> : petsError ? <div className="history-error" role="alert"><p>{petsError}</p><Button onClick={reloadPets}>重新读取宠物资料</Button></div> : <>
      <label className="growth-filter" htmlFor="growth-pet">查看谁的成长？<select id="growth-pet" value={selected} onChange={event => setSearch({ pet: event.target.value })}>{missingPet && <option value={selected}>宠物不存在或无权访问</option>}{pets.map(item => <option key={item.id} value={item.id}>{item.name}{item.isDefault ? ' · 默认宠物' : ''}</option>)}<option value="unassigned">未关联宠物的作品</option></select></label>
      {missingPet ? <p className="error-message" role="alert">宠物不存在或无权访问，请重新选择。</p> : <>
        <section className="growth-profile" aria-label="当前宠物"><span className="growth-avatar" aria-hidden="true">{pet ? petAvatars[pet.avatar] : '🐾'}</span><div><h2>{pet?.name || '待归档的可爱'}</h2><p>{pet ? pet.ageLabel || '年龄未设置' : '把旧作品关联给宠物，就能串进它的成长故事。'}</p>{pet?.birthDate && <small>{pet.approximate ? '估计生日' : '生日'} {pet.birthDate}</small>}</div><Button onClick={() => { updateDraft({ petId: pet?.id || '' }); navigate('/') }}><Plus size={16} />记录这一刻</Button></section>
        {loading ? <p className="history-status" role="status">正在翻看成长故事…</p> : items.length ? <div className="growth-timeline">{[...groups.entries()].map(([month, entries]) => <section key={month} className="growth-month"><h2><CalendarDays size={17} />{month.includes('-') ? `${month.slice(0, 4)}年${Number(month.slice(5))}月` : month}</h2>{entries.map(task => <GrowthEntry key={task.id} task={task} pets={pets} onAssigned={() => setAttempt(value => value + 1)} />)}</section>)}</div>
          : !snapshot.error && <section className="pets-empty"><CalendarDays size={40} /><h2>{pet ? `${pet.name}的故事，从今天开始` : '暂时没有待归档的作品'}</h2><p>{pet ? '选择它生成第一张图片，就会自动出现在这里。' : '以前未关联宠物的成功作品会显示在这里。'}</p>{!pets.length && <Link className="button button--secondary" to="/pets">添加第一只宠物</Link>}</section>}
        {current && snapshot.error && <div className="history-error" role="alert"><p>{snapshot.error}</p><Button variant="secondary" onClick={() => { if (items.length) { void loadMore() } else setAttempt(value => value + 1) }}>重新读取成长记录</Button></div>}
        {current && snapshot.cursor && <div className="history-more"><Button variant="secondary" busy={moreLoading} onClick={() => { void loadMore() }}>查看更多成长记录</Button></div>}
      </>}
    </>}
  </main>
}
