import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 5173,
    open: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
      '/socket.io': {
        target: 'http://localhost:8081',
        ws: true,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: path.resolve(__dirname, '../api/dist'),
    emptyOutDir: true,
  },
});
