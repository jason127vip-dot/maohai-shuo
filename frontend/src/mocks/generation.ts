import { createDemoImage } from '../lib/image'
import type { GenerationApi, GenerationTask } from '../features/generation/types'

const tasks = new Map<string, { task: GenerationTask; readyAt: number }>()
const requests = new Map<string, string>()
let remainingCredits = 2
let creating = false

export async function mockCurrentUser() { return { remainingCredits } }

// All demo data stays in memory. Refreshing resets demo credits and tasks.
export const mockGenerationApi: GenerationApi = {
  async create(input) {
    const previous = requests.get(input.idempotencyKey)
    if (previous) return { ...tasks.get(previous)!.task }
    if (creating) throw new Error('上一张还在提交，请稍等一下。')
    if (remainingCredits <= 0) throw new Error('体验额度已用完，本地预览可刷新页面重新体验。')
    creating = true
    try {
      const imageUrl = await createDemoImage(input.photo)
      const task: GenerationTask = {
        id: crypto.randomUUID(), status: 'queued', scene: input.scene, style: input.style, imageUrl, isMock: true,
      }
      tasks.set(task.id, { task, readyAt: Date.now() + 3200 })
      requests.set(input.idempotencyKey, task.id)
      remainingCredits -= 1
      return { ...task, imageUrl: undefined }
    } finally { creating = false }
  },
  async get(taskId, signal) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    const item = tasks.get(taskId)
    if (!item) throw new Error('本地预览记录已失效，请返回制作页重新选择照片。')
    const status = Date.now() >= item.readyAt ? 'succeeded' : 'processing'
    return { ...item.task, status, imageUrl: status === 'succeeded' ? item.task.imageUrl : undefined }
  },
}
