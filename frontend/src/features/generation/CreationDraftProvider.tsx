import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { DraftContext } from './draftContext'
import type { CreationDraft } from './draftContext'

/** Keeps the current creation when navigating to privacy/results; never persists photos. */
export function CreationDraftProvider({ children }: { children: ReactNode }) {
  const [draft, setDraft] = useState<CreationDraft>({ photo: null, scene: null, style: null, consent: false })
  const submissionKey = useRef<string | null>(null)

  useEffect(() => {
    const previewUrl = draft.photo?.previewUrl
    return () => { if (previewUrl) URL.revokeObjectURL(previewUrl) }
  }, [draft.photo])

  const updateDraft = useCallback((patch: Partial<CreationDraft>) => {
    if ('photo' in patch || 'scene' in patch || 'style' in patch) submissionKey.current = null
    setDraft(previous => ({ ...previous, ...patch }))
  }, [])

  const getSubmissionKey = useCallback(() => {
    submissionKey.current ??= crypto.randomUUID()
    return submissionKey.current
  }, [])

  // Only an explicit next creation or changed inputs begins a new billable request.
  const startNextCreation = useCallback(() => { submissionKey.current = null }, [])

  return <DraftContext.Provider value={{ draft, updateDraft, getSubmissionKey, startNextCreation }}>{children}</DraftContext.Provider>
}
