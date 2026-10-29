import { useCallback, useEffect, useState } from 'react'
import { getCurrentUser } from './api'
import type { CurrentUser } from './types'

export function useCurrentUser() {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    const controller = new AbortController()
    getCurrentUser(controller.signal).then(value => {
      if (!controller.signal.aborted) { setUser(value); setError(null) }
    }).catch((reason: unknown) => {
      if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : '额度读取失败，请重试。')
    })
    return () => controller.abort()
  }, [attempt])
  const reload = useCallback(() => setAttempt(value => value + 1), [])
  return { user, error, reload }
}
