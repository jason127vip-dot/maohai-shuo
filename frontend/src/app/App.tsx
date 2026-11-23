import './App.css'
import { Heart, PawPrint, Images, Sprout } from 'lucide-react'
import { Link, NavLink } from 'react-router'
import { AppRouter } from './router'
import { IS_MOCK } from '../lib/config'
import { CreationDraftProvider } from '../features/generation/CreationDraftProvider'
import { RouteScroll } from './RouteScroll'
import { WechatAccess } from '../features/user/WechatAccess'

export function App() {
  return (
    <>
      <RouteScroll />
      <a className="skip-link" href="#main-content">跳到主要内容</a>
      <header className="site-header">
        <Link className="brand" to="/" aria-label="毛孩儿说首页">
          <span className="brand-mark"><PawPrint size={24} fill="currentColor" /></span>
          <span>毛孩儿说<span className="brand-dot">.</span></span>
        </Link>
        <div className="header-note"><Heart size={14} /> 每只毛孩，都有自己的小剧场</div>
        <nav className="header-links" aria-label="我的毛孩"><NavLink className="header-history" to="/growth" aria-label="成长轨迹"><Sprout size={18} /><span className="header-label-full">成长轨迹</span><span className="header-label-mobile">成长</span></NavLink><NavLink className="header-history" to="/history" aria-label="我的作品"><Images size={18} /><span className="header-label-full">我的作品</span><span className="header-label-mobile">作品</span></NavLink></nav>
      </header>
      {IS_MOCK && <div className="demo-banner">体验预览版 · 生成流程使用模拟数据，暂不调用 AI</div>}
      <CreationDraftProvider><WechatAccess><AppRouter /></WechatAccess></CreationDraftProvider>
      <footer className="site-footer">
        <span>一点小表情，一份大偏爱。</span>
        <div><Link to="/privacy">隐私说明</Link><span className="footer-divider">·</span><span>图片由 AI 生成，请理性使用</span></div>
      </footer>
    </>
  )
}
