import { IS_MOCK } from '../../lib/config'
import { request } from '../../lib/request'
import type { CurrentUser } from './types'

export async function getCurrentUser(signal?: AbortSignal): Promise<CurrentUser> {
  if (IS_MOCK) {
    const { mockCurrentUser } = await import('../../mocks/generation')
    return mockCurrentUser()
  }
  return request<CurrentUser>('/me', { signal })
}
