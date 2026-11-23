import type { Pet, PetInput } from '../features/pets/types'
import { ageAt } from '../features/pets/age'
import { createRandomId } from '../lib/uuid'

const pets = new Map<string, Pet>()
export function findMockPet(id: string) {
  const pet = pets.get(id)
  if (!pet) throw new Error('宠物资料不存在，请重新选择。')
  return { ...pet, ...ageAt(pet.birthDate, pet.approximate) }
}
export const mockPetsApi = {
  async list(signal?: AbortSignal) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    return { items: [...pets.values()].map(pet => findMockPet(pet.id)).sort((a, b) => Number(b.isDefault) - Number(a.isDefault) || a.createdAt.localeCompare(b.createdAt) || a.id.localeCompare(b.id)) }
  },
  async save(input: PetInput, id?: string) {
    const old = id ? findMockPet(id) : undefined
    const pet: Pet = { ...input, id: id || createRandomId(), createdAt: old?.createdAt || new Date().toISOString(), updatedAt: new Date().toISOString(), ...ageAt(input.birthDate, input.approximate), isDefault: input.isDefault || Boolean(old?.isDefault) || pets.size === 0 }
    if (pet.isDefault) for (const [key, other] of pets) pets.set(key, { ...other, isDefault: false })
    pets.set(pet.id, pet)
    return { ...pet }
  },
}
