import './CreateForm.css'
import { useEffect, useRef, useState } from 'react'
import { ArrowRight, Gift, ShieldCheck, Sparkles } from 'lucide-react'
import { Link, useNavigate } from 'react-router'
import { Button } from '../../components/Button'
import { generationApi } from '../../features/generation/api'
import { PhotoUploader } from '../../features/generation/components/PhotoUploader'
import { ScenePicker } from '../../features/generation/components/ScenePicker'
import { StylePicker } from '../../features/generation/components/StylePicker'
import type { ArtStyleId, SceneId } from '../../features/generation/types'
import { useCurrentUser } from '../../features/user/useCurrentUser'
import { useCreationDraft } from '../../features/generation/useCreationDraft'
import { Showcase } from './Showcase'
import styles from './CreatePage.module.css'

export function CreatePage() {
  const navigate = useNavigate()
  const { user, error: userError, reload } = useCurrentUser()
  const { draft, updateDraft, getSubmissionKey } = useCreationDraft()
  const { photo, scene, style, consent } = draft
  const [processingPhoto, setProcessingPhoto] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const submitLock = useRef(false)
  const mounted = useRef(false)
  const available = user !== null && user.remainingCredits > 0
  const completed = [Boolean(photo), Boolean(scene), Boolean(style)].filter(Boolean).length

  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])

  function formChanged() { setError(null) }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitLock.current || processingPhoto || !photo || !scene || !style || !consent || !available) return
    submitLock.current = true
    setSubmitting(true)
    setError(null)
    try {
      const task = await generationApi.create({ photo: photo.file, scene, style, idempotencyKey: getSubmissionKey() })
      if (mounted.current) navigate(`/generations/${encodeURIComponent(task.id)}`)
    } catch (reason) {
      if (mounted.current) {
        setError(reason instanceof Error ? reason.message : '提交失败，请稍后再试。')
        reload()
      }
    } finally {
      submitLock.current = false
      if (mounted.current) setSubmitting(false)
    }
  }

  return (
    <main className={styles.layout} id="main-content">
      <Showcase />
      <section className={styles.workspace} aria-labelledby="create-title">
        <div className={styles.cardHeader}>
          <div><span className="small-kicker">LET’S MAKE SOMETHING CUTE</span><h2 id="create-title">今天，毛孩想说什么？</h2></div>
          <span className="credit-badge"><Gift size={14} /> {user ? <>剩余 <b>{user.remainingCredits}</b> 次免费</> : userError ? '额度读取失败' : '读取额度中'}</span>
        </div>
        <form onSubmit={event => { void submit(event) }}>
          <fieldset disabled={submitting} className={styles.section}>
            <legend><span className="step-number">01</span><span>上传毛孩照片</span><span className="section-note">让可爱有迹可循</span></legend>
            <PhotoUploader value={photo} disabled={submitting} onProcessingChange={setProcessingPhoto} onChange={value => { updateDraft({ photo: value }); formChanged() }} />
          </fieldset>
          <fieldset disabled={submitting} className={styles.section}>
            <legend><span className="step-number">02</span><span>选一个小剧场</span><span className="section-note">它一定干过这些事</span></legend>
            <ScenePicker value={scene} disabled={submitting} onChange={(value: SceneId) => { updateDraft({ scene: value }); formChanged() }} />
          </fieldset>
          <fieldset disabled={submitting} className={styles.section}>
            <legend><span className="step-number">03</span><span>挑一种画风</span><span className="section-note">插画仅作风格示意</span></legend>
            <StylePicker value={style} scene={scene} disabled={submitting} onChange={(value: ArtStyleId) => { updateDraft({ style: value }); formChanged() }} />
          </fieldset>
          <div className={styles.submitArea}>
            <label className="consent"><input type="checkbox" checked={consent} disabled={submitting} onChange={event => updateDraft({ consent: event.target.checked })} /><span>照片为本人所有或已获授权，同意<Link to="/privacy">隐私说明</Link></span></label>
            {userError && <div className="inline-error" role="alert"><span>{userError}</span><button type="button" onClick={reload}>重新读取</button></div>}
            {user && !available && <p className="error-message" role="status">免费额度已用完，谢谢你带毛孩来玩。</p>}
            {error && <p className="error-message" role="alert">{error}</p>}
            <Button type="submit" className="generate-button" busy={submitting} disabled={processingPhoto || completed < 3 || !consent || !available}>
              {!submitting && <Sparkles size={19} />}<span>{submitting ? '正在提交，请稍等' : '让毛孩出个表情'}</span>{!submitting && <ArrowRight size={19} />}
            </Button>
            <p className="submit-hint" aria-live="polite">{completed < 3 ? `完成上面的 ${3 - completed} 个选择，就可以开演啦` : !consent ? '阅读并同意隐私说明后，就可以开演啦' : '准备好啦，把这份可爱变成表情吧'}</p>
            <div className="privacy-footnote"><ShieldCheck size={13} /><span>仅用于本次创作 · 成品标注「AI 生成」</span></div>
          </div>
        </form>
      </section>
    </main>
  )
}
