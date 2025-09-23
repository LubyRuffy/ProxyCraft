import { PropsWithChildren } from 'react';

import { QueryProvider } from './query-client';

export function AppProvider({ children }: PropsWithChildren) {
  return <QueryProvider>{children}</QueryProvider>;
}
