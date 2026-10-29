import { Navigate, Route, Routes } from 'react-router'
import { CreatePage } from '../pages/create/CreatePage'
import { ResultPage } from '../pages/result/ResultPage'
import { PrivacyPage } from '../pages/privacy/PrivacyPage'

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<CreatePage />} />
      <Route path="/generations/:taskId" element={<ResultPage />} />
      <Route path="/privacy" element={<PrivacyPage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
