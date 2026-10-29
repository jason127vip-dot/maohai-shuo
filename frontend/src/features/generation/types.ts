export type SceneId = 'mischief' | 'snack' | 'sleep' | 'door'
export type ArtStyleId = 'sketch' | 'cartoon' | 'watercolor'
export type TaskStatus = 'queued' | 'processing' | 'succeeded' | 'failed'

export interface CreateGenerationInput {
  photo: File
  scene: SceneId
  style: ArtStyleId
  idempotencyKey: string
}

export interface GenerationTask {
  id: string
  status: TaskStatus
  scene: SceneId
  style: ArtStyleId
  imageUrl?: string
  error?: string
  isMock?: boolean
}

export interface GenerationApi {
  create(input: CreateGenerationInput): Promise<GenerationTask>
  get(taskId: string, signal?: AbortSignal): Promise<GenerationTask>
}
