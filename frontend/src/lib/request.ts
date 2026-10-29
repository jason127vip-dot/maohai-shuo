import { API_BASE_URL } from './config'

export class ApiError extends Error {
  constructor(message: string, public readonly code: string, public readonly status: number) {
    super(message)
    this.name = 'ApiError'
  }
}

/** All business APIs return JSON; HTTP errors share { code, message }. */
export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), 20_000)
  const abort = () => controller.abort()
  if (init.signal?.aborted) controller.abort()
  else init.signal?.addEventListener('abort', abort, { once: true })
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      ...init,
      headers,
      credentials: 'include',
      signal: controller.signal,
    })
    const body: unknown = await response.json().catch(() => null)
    if (!response.ok) {
      const error = body as { code?: string; message?: string } | null
      throw new ApiError(error?.message || '请求没有成功，请稍后再试。', error?.code || 'HTTP_ERROR', response.status)
    }
    if (body === null) throw new Error('服务器返回的数据格式不正确，请稍后再试。')
    return body as T
  } catch (error) {
    if (error instanceof ApiError) throw error
    if (init.signal?.aborted) throw error
    if (controller.signal.aborted) throw new Error('网络响应较慢，请稍后再试。')
    if (error instanceof TypeError) throw new Error('连接失败，请检查网络后重试。')
    throw error
  } finally {
    window.clearTimeout(timeout)
    init.signal?.removeEventListener('abort', abort)
  }
}
