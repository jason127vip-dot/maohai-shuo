import { IS_MOCK } from '../../lib/config'
import { request } from '../../lib/request'
import type { Pet, PetInput } from './types'

export const petsApi = {
  async list(signal?: AbortSignal): Promise<{ items: Pet[] }> {
    if (IS_MOCK) return (await import('../../mocks/pets')).mockPetsApi.list(signal)
    return request('/pets', { signal })
  },
  async save(input: PetInput, id?: string): Promise<Pet> {
    if (IS_MOCK) return (await import('../../mocks/pets')).mockPetsApi.save(input, id)
    return request(id ? `/pets/${encodeURIComponent(id)}` : '/pets', { method: id ? 'PATCH' : 'POST', body: JSON.stringify(input) })
  },
}
