import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The admin is served by the Go server under /admin/.
export default defineConfig({
  base: '/admin/',
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
  build: {
    outDir: '../server/internal/webui/dist',
    emptyOutDir: true,
  },
})
