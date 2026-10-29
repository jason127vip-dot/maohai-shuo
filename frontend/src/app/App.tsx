import './App.css'
import { Heart, PawPrint } from 'lucide-react'
import { Link } from 'react-router'
import { AppRouter } from './router'
import { IS_MOCK } from '../lib/config'
import { CreationDraftProvider } from '../features/generation/CreationDraftProvider'
import { RouteScroll } from './RouteScroll'

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
        <span className="header-tag">宠物表情包制造所</span>
      </header>
      {IS_MOCK && <div className="demo-banner">体验预览版 · 生成流程使用模拟数据，暂不调用 AI</div>}
      <CreationDraftProvider><AppRouter /></CreationDraftProvider>
      <footer className="site-footer">
        <span>一点小表情，一份大偏爱。</span>
        <div><Link to="/privacy">隐私说明</Link><span className="footer-divider">·</span><span>图片由 AI 生成，请理性使用</span></div>
      </footer>
    </>
  )
}
