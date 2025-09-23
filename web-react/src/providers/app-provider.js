import { jsx as _jsx } from "react/jsx-runtime";
import { QueryProvider } from './query-client';
export function AppProvider({ children }) {
    return _jsx(QueryProvider, { children: children });
}
