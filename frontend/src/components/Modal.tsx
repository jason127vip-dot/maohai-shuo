import './Modal.css'
import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import type { ReactNode } from 'react'

export function Modal({ title, children, onClose }: { title: string; children: ReactNode; onClose(): void }) {
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    const dialog = ref.current
    dialog?.showModal()
    return () => dialog?.close()
  }, [])
  return (
    <dialog className="modal" ref={ref} aria-label={title} onCancel={event => { event.preventDefault(); onClose() }}
      onClick={event => { if (event.target === event.currentTarget) onClose() }}>
      <div className="modal-content"><button type="button" className="modal-close" aria-label="关闭" onClick={onClose}><X size={21} /></button><h2>{title}</h2>{children}</div>
    </dialog>
  )
}
