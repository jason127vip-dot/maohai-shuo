import { IS_MOCK } from '../../lib/config'
import { request } from '../../lib/request'
import type { CreateGenerationInput, GenerationApi, GenerationTask, GenerationHistory } from './types'

const backendApi: GenerationApi = {
  list(cursor, signal, petId) {
    const query = new URLSearchParams()
    if (cursor) query.set('cursor', cursor)
    if (petId) query.set('petId', petId)
    return request<GenerationHistory>(`/generations?${query.toString()}`, { signal })
  },
  assignPet(taskId, petId) { return request(`/generations/${encodeURIComponent(taskId)}/pet`, { method: 'PATCH', body: JSON.stringify({ petId }) }) },
  updateNote(taskId, note) { return request(`/generations/${encodeURIComponent(taskId)}/note`, { method: 'PATCH', body: JSON.stringify({ note }) }) },
  create(input: CreateGenerationInput) {
    const body = new FormData()
    body.append('photo', input.photo)
    body.append('scene', input.scene)
    body.append('style', input.style)
    if (input.petId) { body.append('petId', input.petId); body.append('showPetLabel', String(Boolean(input.showPetLabel))) }
    return request<GenerationTask>('/generations', {
      method: 'POST', body, headers: { 'Idempotency-Key': input.idempotencyKey },
    })
  },
  get(taskId, signal) {
    return request<GenerationTask>(`/generations/${encodeURIComponent(taskId)}`, { signal })
  },
}

// Pages never import a model vendor. The same contract serves mock and Go APIs.
export const generationApi: GenerationApi = {
  async list(cursor, signal, petId) {
    if (IS_MOCK) return (await import('../../mocks/generation')).mockGenerationApi.list(cursor, signal, petId)
    return backendApi.list(cursor, signal, petId)
  },
  async assignPet(taskId, petId) {
    if (IS_MOCK) return (await import('../../mocks/generation')).mockGenerationApi.assignPet(taskId, petId)
    return backendApi.assignPet(taskId, petId)
  },
  async updateNote(taskId, note) {
    if (IS_MOCK) return (await import('../../mocks/generation')).mockGenerationApi.updateNote(taskId, note)
    return backendApi.updateNote(taskId, note)
  },
  async create(input) {
    if (IS_MOCK) return (await import('../../mocks/generation')).mockGenerationApi.create(input)
    return backendApi.create(input)
  },
  async get(taskId, signal) {
    if (IS_MOCK) return (await import('../../mocks/generation')).mockGenerationApi.get(taskId, signal)
    return backendApi.get(taskId, signal)
  },
}
