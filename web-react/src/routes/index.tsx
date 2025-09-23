import { Routes, Route, Navigate } from 'react-router-dom';

import { AppLayout } from '@/layouts/app-layout';
import { TrafficPage } from '@/pages/traffic';
import { SettingsPage } from '@/pages/settings';

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route index element={<Navigate to="/traffic" replace />} />
        <Route path="/traffic" element={<TrafficPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="*" element={<Navigate to="/traffic" replace />} />
      </Route>
    </Routes>
  );
}
