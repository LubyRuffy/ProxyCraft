import { jsx as _jsx } from "react/jsx-runtime";
import { useState } from 'react';
import { QueryClient, QueryClientProvider, } from '@tanstack/react-query';
export function QueryProvider({ children }) {
    const [client] = useState(() => new QueryClient({
        defaultOptions: {
            queries: {
                refetchOnWindowFocus: false,
                retry: 1,
            },
        },
    }));
    return _jsx(QueryClientProvider, { client: client, children: children });
}
