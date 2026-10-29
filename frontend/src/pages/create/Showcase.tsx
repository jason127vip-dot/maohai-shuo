import './Showcase.css'
import { ArrowDownRight, Heart, Sparkle } from 'lucide-react'
import { PetArt } from '../../components/PetArt'

export function Showcase() {
  return (
    <aside className="showcase" aria-label="产品介绍和插画示意">
      <span className="eyebrow"><span /> 给毛孩一点戏份</span>
      <h1>它的每个小表情，<br />都值得<span className="headline-emphasis">被收藏<svg viewBox="0 0 230 14" preserveAspectRatio="none" aria-hidden="true"><path d="M4 10Q95-4 225 6" /></svg></span>。</h1>
      <p className="hero-description">上传一张照片，把毛孩的小脾气、小心思，<br className="desktop-break" />变成只有你才懂的专属表情包。</p>
      <div className="sticker-stage">
        <span className="handwritten stage-note">今天的可爱，有图为证 <ArrowDownRight size={28} strokeWidth={1.3} /></span>
        <div className="sample-card sample-card--back"><span className="paper-tape" /><PetArt scene="sleep" style="watercolor" /><span className="sample-caption">电量不足，先睡了</span><span className="sample-brand">毛孩儿说 · 插画示意</span></div>
        <div className="sample-card sample-card--front"><span className="paper-tape" /><PetArt scene="mischief" /><span className="sample-caption">不是我，真的。</span><span className="sample-brand">毛孩儿说 · 插画示意</span></div>
        <span className="floating-heart"><Heart size={26} fill="currentColor" strokeWidth={1.3} /></span>
        <Sparkle className="floating-sparkle" size={31} strokeWidth={1.4} />
        <span className="sticker-caption handwritten">毛色、花纹，还有它的小神态 ♡</span>
      </div>
      <div className="hero-points"><span><span className="point-dot point-dot--orange" /> 4 个生活小剧场</span><span><span className="point-dot point-dot--green" /> 3 种手绘风格</span><span><span className="point-dot point-dot--yellow" /> 一份专属可爱</span></div>
      <p className="sample-disclaimer">以上为界面插画示意，实际效果以模型生成结果为准</p>
    </aside>
  )
}
