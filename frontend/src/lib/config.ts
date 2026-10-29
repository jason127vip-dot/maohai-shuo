// Mock must be explicitly enabled locally. A production build never uses mock data.
export const IS_MOCK = import.meta.env.DEV && import.meta.env.VITE_USE_MOCK === 'true'
export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api').replace(/\/$/, '')
