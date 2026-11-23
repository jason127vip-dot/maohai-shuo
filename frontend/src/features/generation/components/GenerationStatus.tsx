import './generation.css'
import { useEffect, useState } from 'react'
import { Heart, LoaderCircle, PawPrint, Sparkles } from 'lucide-react'
import { PetArt } from '../../../components/PetArt'
import type { GenerationTask, SceneId } from '../types'

const sceneLines: Record<SceneId, string[]> = {
  mischief: [
    '先说好，沙发上的那个洞不是我弄的。',
    '正在练习一个无辜的眼神。',
    '你看我这么可爱，应该不会生气吧？',
    '家里只是换了一种装修风格。',
    '等会儿记得拍到我乖巧的那一面。',
  ],
  snack: [
    '听说拍完这张，就有小零食？',
    '正在练习一个让你无法拒绝的眼神。',
    '就一口，我保证只吃一小口。',
    '等会儿记得把我发给饭搭子。',
    '耳朵已经听见零食袋的声音了。',
  ],
  sleep: [
    '困了，但可爱这件事我可以梦里完成。',
    '这个睡姿，我可是研究了好久。',
    '梦里有小鱼干，先不醒了。',
    '拍好了吗？我要翻个身继续睡啦。',
    '呼噜声是我的拍摄背景音乐。',
  ],
  door: [
    '你怎么还不回来呀？',
    '门口的脚步声，每一个我都认真听了。',
    '等你回来，先摸摸我再放包。',
    '今天的尾巴，专门为你摇。',
    '见到你的第一件事，当然是扑过去。',
  ],
}

const cheerReplies = ['收到！这就认真营业。', '有你打气，尾巴都翘起来啦。', '这颗爱心，我先收进小口袋。']

export function GenerationStatus({ task }: { task: GenerationTask | null }) {
  return <WaitingScene key={task?.id ?? 'pending'} task={task} />
}

function WaitingScene({ task }: { task: GenerationTask | null }) {
  const [startedAt] = useState(() => Date.now())
  const [now, setNow] = useState(startedAt)
  const [cheerCount, setCheerCount] = useState(0)
  const [cheer, setCheer] = useState<number | null>(null)
  const createdAt = task?.createdAt ? Date.parse(task.createdAt) : NaN
  const elapsed = Math.max(0, now - (Number.isFinite(createdAt) ? createdAt : startedAt))
  const lines = sceneLines[task?.scene ?? 'mischief']
  const lineIndex = Math.floor(elapsed / 7000) % lines.length
  const line = cheer === null ? lines[lineIndex] : cheerReplies[(cheer - 1) % cheerReplies.length]
  const waitingLong = elapsed >= 60_000

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [])

  useEffect(() => {
    if (cheer === null) return
    const timer = window.setTimeout(() => setCheer(null), 3000)
    return () => window.clearTimeout(timer)
  }, [cheer])

  function giveCheer() {
    const next = cheerCount + 1
    setCheerCount(next)
    setCheer(next)
  }

  return (
    <div className="generation-waiting">
      <div className="waiting-art">
        <PetArt scene={task?.scene} style={task?.style} />
        <Sparkles className="waiting-sparkle" size={24} aria-hidden="true" />
      </div>
      <h1>你的毛孩，正在准备登场</h1>
      <p className="waiting-estimate" role="status">
        {waitingLong ? '这次准备得稍久一点，图片仍在生成中。' : '通常需要约 1 分钟，请给它一点时间。'}
      </p>
      <div className="waiting-speech">
        <span className="waiting-speech-label">毛孩的内心戏</span>
        <p key={cheer === null ? `line-${lineIndex}` : `cheer-${cheer}`} className="waiting-line">“{line}”</p>
        <span className="waiting-reply" role="status">{cheer === null ? '' : line}</span>
      </div>
      <div className="waiting-cheer">
        <button className="waiting-cheer-button" type="button" onClick={giveCheer}>
          <PawPrint size={19} aria-hidden="true" /> 给它打气
        </button>
        {cheer !== null && <span key={cheer} className="waiting-hearts" aria-hidden="true">
          <Heart className="waiting-heart" size={17} />
          <Heart className="waiting-heart" size={21} />
          <Heart className="waiting-heart" size={15} />
        </span>}
      </div>
      <span className="waiting-pill" role="status">
        <LoaderCircle size={15} className="spin" aria-hidden="true" />
        {!task ? '正在查询任务…' : task.status === 'queued' ? '正在排队…' : '图片生成中…'}
      </span>
    </div>
  )
}
