import { useEffect, useState } from 'react'
import { petsApi } from './api'
import type { Pet } from './types'

export function usePets() {
  const [attempt, setAttempt] = useState(0)
  const [snapshot, setSnapshot] = useState<{ attempt: number; items: Pet[]; error: string }>({ attempt: -1, items: [], error: '' })
  useEffect(() => {
    const controller = new AbortController()
    petsApi.list(controller.signal).then(({ items }) => {
      if (!controller.signal.aborted) setSnapshot({ attempt, items, error: '' })
    }).catch((reason: unknown) => {
      if (!controller.signal.aborted) setSnapshot({ attempt, items: [], error: reason instanceof Error ? reason.message : '宠物资料暂时无法读取' })
    })
    return () => controller.abort()
  }, [attempt])
  return { pets: snapshot.items, error: snapshot.error, loading: snapshot.attempt !== attempt, reload: () => setAttempt(value => value + 1) }
}
