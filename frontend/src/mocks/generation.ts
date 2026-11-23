import { createRandomId } from '../lib/uuid'
import { createDemoImage } from '../lib/image'
import type { GenerationApi, GenerationTask } from '../features/generation/types'
import { findMockPet } from './pets'
import { ageAt } from '../features/pets/age'

const tasks = new Map<string, { task: GenerationTask; readyAt: number }>()
const requests = new Map<string, string>()
let remainingCredits = 2
let creating = false

export async function mockCurrentUser() { return { remainingCredits } }

// All demo data stays in memory. Refreshing resets demo credits and tasks.
export const mockGenerationApi: GenerationApi = {
  async list(cursor, signal, petId) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    const items = [...tasks.values()].filter(item => Date.now() >= item.readyAt && (!petId || (petId === 'unassigned' ? !item.task.petId : item.task.petId === petId)))
      .map(item => ({ ...item.task, status: 'succeeded' as const }))
      .sort((a, b) => (b.createdAt || '').localeCompare(a.createdAt || '') || b.id.localeCompare(a.id))
    const start = cursor ? items.findIndex(item => item.id === cursor) + 1 : 0
    const page = items.slice(start, start + 20)
    return { items: page, nextCursor: start + 20 < items.length ? page[page.length - 1].id : '' }
  },
  async assignPet(taskId, petId) {
    const item = tasks.get(taskId), pet = findMockPet(petId)
    if (!item || Date.now() < item.readyAt) throw new Error('作品不存在。')
    if (item.task.petId && item.task.petId !== petId) throw new Error('作品已经关联另一只宠物。')
    if (!item.task.petId) item.task = { ...item.task, petId, petName: pet.name, petAge: ageAt(pet.birthDate, pet.approximate, item.task.createdAt?.slice(0, 10)).ageLabel }
    return { ...item.task, status: 'succeeded' }
  },
  async updateNote(taskId, note) {
    const item = tasks.get(taskId)
    if (!item || Date.now() < item.readyAt) throw new Error('作品不存在。')
    if ([...note].length > 200) throw new Error('记录文字最多200个字。')
    item.task.note = note
    return { ...item.task, status: 'succeeded' }
  },
  async create(input) {
    const previous = requests.get(input.idempotencyKey)
    if (previous) return { ...tasks.get(previous)!.task }
    if (creating) throw new Error('上一张还在提交，请稍等一下。')
    if (remainingCredits <= 0) throw new Error('体验额度已用完，本地预览可刷新页面重新体验。')
    creating = true
    try {
      const pet = input.petId ? findMockPet(input.petId) : undefined
      const createdAt = new Date().toISOString()
      const caption = pet && input.showPetLabel ? [pet.name, pet.ageLabel, createdAt.slice(0, 10)].filter(Boolean).join(' · ') : ''
      const imageUrl = await createDemoImage(input.photo, caption)
      const task: GenerationTask = {
        id: createRandomId(), createdAt, status: 'queued', scene: input.scene, style: input.style, imageUrl, isMock: true, petId: pet?.id, petName: pet?.name, petAge: pet?.ageLabel, showPetLabel: Boolean(pet && input.showPetLabel),
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
