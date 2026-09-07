// defineConfig comes from vitest/config rather than vite so the `test` block
// below is typed; it is the same function with the test options merged in.
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// The dev server proxies /api to the Go API rather than enabling CORS for it:
// one origin in development means the code that runs locally is the code that
// runs behind the reverse proxy in the compose stack.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: true,
  },
})
