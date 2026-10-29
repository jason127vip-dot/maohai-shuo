import { useContext } from 'react'
import { DraftContext } from './draftContext'

export function useCreationDraft() {
  const context = useContext(DraftContext)
  if (!context) throw new Error('useCreationDraft must be used within CreationDraftProvider')
  return context
}
