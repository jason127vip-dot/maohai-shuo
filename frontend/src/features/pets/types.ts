export interface PetInput {
  name: string
  species: '' | 'cat' | 'dog'
  avatar: 'paw' | 'cat' | 'dog'
  birthDate: string
  approximate: boolean
  isDefault: boolean
}

export interface Pet extends PetInput {
  id: string
  createdAt: string
  updatedAt: string
  ageLabel: string
  ageMonths: number
}

export const petAvatars = { paw: '🐾', cat: '🐱', dog: '🐶' }
