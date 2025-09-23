import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { BrowserRouter } from 'react-router-dom';
import { Toaster } from 'sonner';
import { AppRoutes } from '@/routes';
function App() {
    return (_jsxs(BrowserRouter, { children: [_jsx(AppRoutes, {}), _jsx(Toaster, {})] }));
}
export default App;
