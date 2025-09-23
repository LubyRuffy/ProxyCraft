/// <reference types="vite/client" />

declare interface ImportMetaEnv {
  readonly VITE_PROXYCRAFT_SOCKET_URL?: string;
}

declare interface ImportMeta {
  readonly env: ImportMetaEnv;
}
