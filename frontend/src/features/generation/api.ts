import { IS_MOCK } from '../../lib/config'
import { request } from '../../lib/request'
import type { CreateGenerationInput, GenerationApi, GenerationTask } from './types'

const backendApi: GenerationApi = {
  create(input: CreateGenerationInput) {
    const body = new FormData()
    body.append('photo', input.photo)
    body.append('scene', input.scene)
    body.append('style', input.style)
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
  async create(input) {
    if (IS_MOCK) return (await import('../../mocks/generation')).mockGenerationApi.create(input)
    return backendApi.create(input)
  },
  async get(taskId, signal) {
    if (IS_MOCK) return (await import('../../mocks/generation')).mockGenerationApi.get(taskId, signal)
    return backendApi.get(taskId, signal)
  },
}
