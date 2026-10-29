import './generation.css'
import { useEffect, useRef, useState } from 'react'
import { Camera, Check, ImagePlus, RefreshCw, Upload, X } from 'lucide-react'
import { preparePhoto } from '../../../lib/image'
import type { SelectedPhoto } from '../../../lib/image'

export function PhotoUploader({ value, onChange, onProcessingChange, disabled }: {
  value: SelectedPhoto | null; onChange(value: SelectedPhoto | null): void; onProcessingChange(value: boolean): void; disabled: boolean
}) {
  const input = useRef<HTMLInputElement>(null)
  const requestId = useRef(0)
  const [processing, setProcessing] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => () => { requestId.current += 1 }, [])

  async function select(file: File | undefined) {
    if (!file || disabled || processing) return
    const current = ++requestId.current
    setProcessing(true)
    onProcessingChange(true)
    setError(null)
    try {
      const photo = await preparePhoto(file)
      if (current !== requestId.current) { URL.revokeObjectURL(photo.previewUrl); return }
      onChange(photo)
    } catch (reason) {
      if (current === requestId.current) setError(reason instanceof Error ? reason.message : '照片读取失败，请重试。')
    } finally {
      if (current === requestId.current) { setProcessing(false); onProcessingChange(false) }
    }
  }

  return (
    <div>
      <input ref={input} id="pet-photo" type="file" accept="image/jpeg,image/png,image/webp" className="visually-hidden"
        tabIndex={-1} disabled={disabled || processing} onChange={event => {
          void select(event.target.files?.[0]); event.target.value = ''
        }} />
      {value ? (
        <div className="photo-preview">
          <img src={value.previewUrl} alt="已选择的宠物照片" />
          <span className="photo-ready"><Check size={13} /> 照片准备好了</span>
          <button className="remove-photo" type="button" aria-label="移除照片" disabled={disabled || processing} onClick={() => onChange(null)}><X size={17} /></button>
          <button className="replace-photo" type="button" disabled={disabled || processing} onClick={() => input.current?.click()}><RefreshCw size={14} /> {processing ? '正在整理照片…' : '换一张照片'}</button>
        </div>
      ) : (
        <button className={`upload-zone ${dragging ? 'upload-zone--dragging' : ''}`} type="button"
          disabled={disabled || processing} aria-describedby="photo-hint" onClick={() => input.current?.click()}
          onDragOver={event => { event.preventDefault(); if (!disabled) setDragging(true) }}
          onDragLeave={() => setDragging(false)} onDrop={event => {
            event.preventDefault(); setDragging(false); void select(event.dataTransfer.files[0])
          }}>
          <span className="upload-art"><ImagePlus size={31} strokeWidth={1.6} /><span className="upload-plus"><Upload size={12} /></span></span>
          <strong>{processing ? '正在整理照片…' : '点这里，把主角请上场'}</strong>
          <span className="upload-description">选择相册照片，或用相机拍一张</span>
          <span className="upload-format">JPEG / PNG / WebP · 不超过 10 MB</span>
        </button>
      )}
      <p className="field-hint" id="photo-hint"><Camera size={13} /> 选一张脸部清晰、只有一只猫或狗的照片，效果更好哦</p>
      {error && <p className="error-message" role="alert">{error}</p>}
    </div>
  )
}
