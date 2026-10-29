import './PrivacyPage.css'
import { ArrowLeft, ShieldCheck } from 'lucide-react'
import { Link } from 'react-router'
import { IS_MOCK } from '../../lib/config'

export function PrivacyPage() {
  return (
    <main id="main-content" className="privacy-page">
      <Link className="back-link" to="/"><ArrowLeft size={16} /> 返回制作页</Link>
      <article className="privacy-document">
        <ShieldCheck size={30} className="privacy-icon" /><h1>你的照片，我们认真对待</h1>
        <p className="privacy-date">体验版说明 · 2026 年 10 月 29 日</p>
        {IS_MOCK ? <><h2>当前是本地流程预览</h2><p>照片仅在当前浏览器内进行压缩和预览，不会上传到服务器或第三方 AI 服务。示例结果展示原始照片和演示标识，并非 AI 生成成品。</p><h2>照片与记录如何保存</h2><p>当前版本的照片、生成记录及体验额度只保留在页面运行内存中。刷新或关闭页面后，这些演示记录会失效。</p></> : <><h2>正式服务说明尚待确认</h2><p>正式隐私指引需要补全运营主体、联系方式、模型服务商、照片保留期限及删除方式。当前页面仅供开发预览，不代表正式上线版本。</p><h2>第三方图片生成</h2><p>真实生成功能接通后，点击生成会将照片和创作选项交给后端及所选 AI 服务处理。正式上线前应明确实际接收方及处理方式，并取得用户同意。</p></>}
        <h2>请只上传有权使用的宠物照片</h2><p>请上传你拥有或已获得授权的猫狗照片，避免包含人脸、证件或其他个人信息。请尊重他人权益。</p>
        <h2>关于图片示例</h2><p>页面中的宠物插画用于说明场景与画风，不是模型效果承诺。正式 AI 成品将带有「毛孩儿说」及「AI 生成」标识。</p>
        <Link className="button button--primary" to="/">知道啦，继续制作</Link>
      </article>
    </main>
  )
}
