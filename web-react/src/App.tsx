import { BrowserRouter } from 'react-router-dom';
import { Toaster } from 'sonner';

import { AppRoutes } from '@/routes';

function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
      <Toaster />
    </BrowserRouter>
  );
}

export default App;
