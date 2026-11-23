import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { useLocation } from 'react-router'
import { IS_MOCK, API_BASE_URL } from '../../lib/config'
import { ApiError } from '../../lib/request'
import { getCurrentUser } from './api'
import './WechatAccess.css'

const attemptKey = 'maohai-wechat-login-started'

function forgetAttempt() {
  try { sessionStorage.removeItem(attemptKey) } catch { /* Login also works without browser storage. */ }
}

function startLogin() {
  try { sessionStorage.setItem(attemptKey, String(Date.now())) } catch { /* Fall back to the manual login button. */ }
  const query = new URLSearchParams({ return_to: window.location.pathname })
  window.location.assign(`${API_BASE_URL}/auth/wechat/start?${query.toString()}`)
}

function mayAutoLogin() {
  if (!/MicroMessenger/i.test(navigator.userAgent)) return false
  try {
    const last = Number(sessionStorage.getItem(attemptKey))
    return !last || Date.now() - last > 60_000
  } catch { return false }
}

/** Authenticate before users select a photo, so the OAuth redirect loses no draft. */
export function WechatAccess({ children }: { children: ReactNode }) {
  const { pathname } = useLocation()
  const publicPage = pathname === '/privacy' || IS_MOCK
  const [status, setStatus] = useState<'loading' | 'ready' | 'login' | 'error'>('loading')
  const [message, setMessage] = useState('')
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    if (publicPage) return
    const controller = new AbortController()
    getCurrentUser(controller.signal).then(() => {
      if (!controller.signal.aborted) { forgetAttempt(); setStatus('ready') }
    }).catch((error: unknown) => {
      if (controller.signal.aborted) return
      if (error instanceof ApiError && error.code === 'WECHAT_LOGIN_REQUIRED') {
        setStatus('login')
        if (mayAutoLogin()) startLogin()
      } else {
        setMessage(error instanceof Error ? error.message : '账号暂时无法读取，请稍后重试。')
        setStatus('error')
      }
    })
    const expired = () => setStatus('login')
    window.addEventListener('maohai-login-required', expired)
    return () => { controller.abort(); window.removeEventListener('maohai-login-required', expired) }
  }, [publicPage, attempt])

  if (publicPage || status === 'ready') return children
  const inWechat = /MicroMessenger/i.test(navigator.userAgent)
  return <main id="main-content" className="wechat-access" aria-live="polite">
    {status === 'loading' ? <p>正在读取你的账号…</p> : status === 'login' ? <>
      <h1>登录后，继续毛孩的小剧场</h1>
      <p>用微信登录，保留你的免费次数和生成记录。</p>
      {inWechat ? <button type="button" className="button button--primary" onClick={startLogin}>微信登录</button> : <p>请复制当前链接，在微信中打开后继续。</p>}
    </> : <>
      <h1>账号暂时无法读取</h1><p role="alert">{message}</p>
      <button type="button" className="button button--primary" onClick={() => { setStatus('loading'); setAttempt(value => value + 1) }}>重新读取</button>
    </>}
  </main>
}
