import './generation.css'
import { LoaderCircle, Sparkles } from 'lucide-react'
import { PetArt } from '../../../components/PetArt'
import type { GenerationTask } from '../types'

export function GenerationStatus({ task }: { task: GenerationTask | null }) {
  return (
    <div className="generation-waiting" role="status" aria-live="polite">
      <div className="waiting-art"><PetArt scene={task?.scene} style={task?.style} /><Sparkles className="waiting-sparkle" size={28} /></div>
      <h1>毛孩的小剧场，正在开演</h1>
      <p>给它一点时间，酝酿一个可爱表情。</p>
      <span className="waiting-pill"><LoaderCircle size={17} className="spin" /> {task?.status === 'queued' ? '正在排队' : '正在准备图片'}…</span>
    </div>
  )
}
