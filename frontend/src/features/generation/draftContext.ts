import { createContext } from 'react'
import type { SelectedPhoto } from '../../lib/image'
import type { ArtStyleId, SceneId } from './types'

export interface CreationDraft {
  photo: SelectedPhoto | null
  scene: SceneId | null
  style: ArtStyleId | null
  consent: boolean
}

interface DraftContextValue {
  draft: CreationDraft
  updateDraft(patch: Partial<CreationDraft>): void
  getSubmissionKey(): string
  startNextCreation(): void
}

export const DraftContext = createContext<DraftContextValue | null>(null)
