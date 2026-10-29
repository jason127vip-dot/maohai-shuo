import { useCallback, useEffect, useState } from 'react'
import { generationApi } from './api'
import type { GenerationTask } from './types'

export function useGenerationTask(taskId: string) {
  const [snapshot, setSnapshot] = useState<{ taskId: string; task: GenerationTask | null; error: string | null }>({ taskId, task: null, error: null })
  const [attempt, setAttempt] = useState(0)
  const retry = useCallback(() => {
    setSnapshot({ taskId, task: null, error: null })
    setAttempt(value => value + 1)
  }, [taskId])

  useEffect(() => {
    const controller = new AbortController()
    let timer: ReturnType<typeof setTimeout> | undefined
    const startedAt = Date.now()
    async function poll() {
      try {
        const next = await generationApi.get(taskId, controller.signal)
        if (controller.signal.aborted) return
        if (next.status === 'succeeded' && !next.imageUrl) throw new Error('图片地址暂时不可用，请重新查询。')
        setSnapshot({ taskId, task: next, error: null })
        if (next.status === 'queued' || next.status === 'processing') {
          if (Date.now() - startedAt > 180_000) {
            setSnapshot({ taskId, task: next, error: '这次生成等待较久，你可以继续查询。请不要重复提交，以免多占额度。' })
            return
          }
          timer = setTimeout(() => { void poll() }, 1500)
        }
      } catch (reason) {
        if (!controller.signal.aborted) setSnapshot({ taskId, task: null, error: reason instanceof Error ? reason.message : '查询失败，请重试。' })
      }
    }
    void poll()
    return () => { controller.abort(); clearTimeout(timer) }
  }, [taskId, attempt])

  return { task: snapshot.taskId === taskId ? snapshot.task : null, error: snapshot.taskId === taskId ? snapshot.error : null, retry }
}
