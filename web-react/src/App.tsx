import { useEffect } from 'react';
import { BrowserRouter } from 'react-router-dom';
import { Toaster } from 'sonner';

import { AppRoutes } from '@/routes';
import { useSettingsStore } from '@/stores/use-traffic-store';

function ThemeSync() {
  const theme = useSettingsStore((state) => state.theme);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const root = document.documentElement;
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const applyTheme = (isDark: boolean) => {
      root.classList.toggle('dark', isDark);
    };

    if (theme === 'auto') {
      applyTheme(media.matches);
      const handleChange = (event: MediaQueryListEvent) => applyTheme(event.matches);
      media.addEventListener('change', handleChange);
      return () => media.removeEventListener('change', handleChange);
    }

    applyTheme(theme === 'dark');
    return;
  }, [theme]);

  return null;
}

function App() {
  return (
    <BrowserRouter>
      <ThemeSync />
      <AppRoutes />
      <Toaster />
    </BrowserRouter>
  );
}

export default App;
