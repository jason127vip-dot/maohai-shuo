import './Button.css'
import type { ButtonHTMLAttributes } from 'react'
import { LoaderCircle } from 'lucide-react'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  busy?: boolean
  variant?: 'primary' | 'secondary'
}

export function Button({ busy, disabled, children, className = '', variant = 'primary', ...props }: ButtonProps) {
  return (
    <button type="button" {...props} className={`button button--${variant} ${className}`} disabled={disabled || busy} aria-busy={busy || undefined}>
      {busy && <LoaderCircle className="spin" size={19} aria-hidden="true" />}{children}
    </button>
  )
}
