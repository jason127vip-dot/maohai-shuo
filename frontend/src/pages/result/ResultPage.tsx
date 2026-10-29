import './ResultPage.css'
import { useState } from 'react'
import { ArrowLeft, ArrowRight, Download, Expand, ImageOff } from 'lucide-react'
import { Link, useParams } from 'react-router'
import { Button } from '../../components/Button'
import { Modal } from '../../components/Modal'
import { GenerationStatus } from '../../features/generation/components/GenerationStatus'
import { scenes, artStyles } from '../../features/generation/options'
import { useGenerationTask } from '../../features/generation/useGenerationTask'
import { useCreationDraft } from '../../features/generation/useCreationDraft'

export function ResultPage() {
  const { taskId = '' } = useParams()
  const { task, error, retry } = useGenerationTask(taskId)
  const { startNextCreation } = useCreationDraft()
  const [showImage, setShowImage] = useState(false)
  const scene = scenes.find(value => value.id === task?.scene)
  const style = artStyles.find(value => value.id === task?.style)
  const failed = task?.status === 'failed'

  return (
    <main id="main-content" className="result-page">
      <Link className="back-link" to="/"><ArrowLeft size={16} /> 返回制作页</Link>
      <section className="result-card">
        {error || failed ? (
          <div className="result-error" role="alert"><ImageOff size={38} /><h1>{failed ? '这次没有生成成功' : '暂时没找到这份可爱'}</h1><p>{error || task?.error || '请稍后重试，额度以后台记录为准。'}</p>
            {!failed && <Button onClick={retry}>重新查询</Button>}<Link className="text-link" to="/">回到制作页 <ArrowRight size={15} /></Link></div>
        ) : task?.status === 'succeeded' && task.imageUrl ? (
          <>
            <span className="eyebrow"><span /> 毛孩的小剧场</span>
            <h1>{task.isMock ? '流程跑通啦，可爱就位' : '这份可爱，属于你'}</h1>
            <p className="result-subtitle">{scene?.title} <span>×</span> {style?.title}</p>
            <button className="result-image-button" onClick={() => setShowImage(true)} type="button" aria-label="查看大图"><img src={task.imageUrl} alt={task.isMock ? '模拟结果：上传的原图和演示标识，非 AI 生成成品' : '生成的宠物表情图'} /><span><Expand size={14} /> 查看大图</span></button>
            {task.isMock && <p className="mock-result-note">这是原图的流程预览，未经过 AI 改图，也没有产生模型费用。</p>}
            <Button className="result-save" onClick={() => setShowImage(true)}><Download size={18} /> 保存这份可爱</Button>
            <Link className="text-link" to="/" onClick={startNextCreation}>再做一张 <ArrowRight size={15} /></Link>
          </>
        ) : <GenerationStatus task={task} />}
      </section>
      {showImage && task?.imageUrl && <Modal title="把这份可爱带走" onClose={() => setShowImage(false)}><img className="modal-image" src={task.imageUrl} alt="可长按保存的图片" /><p className="save-instructions">微信内请长按图片，选择「保存图片」，再从相册发送给朋友。</p><a className="button button--secondary" href={task.imageUrl} download={`maohai-${task.id}.jpg`}>下载图片（浏览器）</a></Modal>}
    </main>
  )
}
