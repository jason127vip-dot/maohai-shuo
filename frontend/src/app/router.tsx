import { Navigate, Route, Routes } from 'react-router'
import { CreatePage } from '../pages/create/CreatePage'
import { ResultPage } from '../pages/result/ResultPage'
import { PrivacyPage } from '../pages/privacy/PrivacyPage'
import { HistoryPage } from '../pages/history/HistoryPage'
import { CreditsPage } from '../pages/credits/CreditsPage'
import { PetsPage } from '../pages/pets/PetsPage'
import { GrowthPage } from '../pages/growth/GrowthPage'

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<CreatePage />} />
      <Route path="/generations/:taskId" element={<ResultPage />} />
      <Route path="/privacy" element={<PrivacyPage />} />
      <Route path="/history" element={<HistoryPage />} />
      <Route path="/credits" element={<CreditsPage />} />
      <Route path="/pets" element={<PetsPage />} />
      <Route path="/growth" element={<GrowthPage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
