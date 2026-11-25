import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const target = process.env.API_PROXY_TARGET ?? 'http://127.0.0.1:8080'
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { host: '127.0.0.1', proxy: { '/api/v1': target, '/health': target, '/openapi.json': target } },
})
