export type SceneId = 'mischief' | 'snack' | 'sleep' | 'door'
export type ArtStyleId = 'sketch' | 'cartoon' | 'watercolor'
export type TaskStatus = 'queued' | 'processing' | 'succeeded' | 'failed'

export interface CreateGenerationInput {
  photo: File
  scene: SceneId
  style: ArtStyleId
  idempotencyKey: string
  petId?: string
  showPetLabel?: boolean
}

export interface GenerationTask {
	createdAt?: string
  id: string
  status: TaskStatus
  scene: SceneId
  style: ArtStyleId
  imageUrl?: string
  error?: string
  isMock?: boolean
  petId?: string
  petName?: string
  petAge?: string
  showPetLabel?: boolean
  note?: string
}

export interface GenerationApi {
	list(cursor?: string, signal?: AbortSignal, petId?: string): Promise<GenerationHistory>
  assignPet(taskId: string, petId: string): Promise<GenerationTask>
  updateNote(taskId: string, note: string): Promise<GenerationTask>
  create(input: CreateGenerationInput): Promise<GenerationTask>
  get(taskId: string, signal?: AbortSignal): Promise<GenerationTask>
}

export interface GenerationHistory {
  items: GenerationTask[]
  nextCursor: string
}
