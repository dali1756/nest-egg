/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

const securityHeaders = {
  'X-Frame-Options': 'DENY',
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'no-referrer',
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  return {
    plugins: [react(), tailwindcss()],
    server: {
      headers: securityHeaders,
      proxy: {
        '/api': env.API_PROXY_TARGET || 'http://localhost:8080',
      },
    },
    preview: {
      headers: securityHeaders,
    },
    test: {
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
    },
  }
})
