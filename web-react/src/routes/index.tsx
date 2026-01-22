import { useEffect } from 'react';
import { Routes, Route, Navigate, useNavigate } from 'react-router-dom';

import { AppLayout } from '@/layouts/app-layout';
import { TrafficPage } from '@/pages/traffic';
import { useLayoutStore } from '@/stores/use-layout-store';

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route index element={<TrafficPage />} />
        <Route path="/traffic" element={<TrafficPage />} />
        <Route path="/settings" element={<SettingsRedirect />} />
        <Route path="*" element={<Navigate to="/traffic" replace />} />
      </Route>
    </Routes>
  );
}

function SettingsRedirect() {
  const navigate = useNavigate();
  const setShowSettings = useLayoutStore((state) => state.setShowSettings);

  useEffect(() => {
    setShowSettings(true);
    navigate('/traffic', { replace: true });
  }, [navigate, setShowSettings]);

  return null;
}
