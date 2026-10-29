import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: { target: ['es2020', 'safari14'] },
  server: {
    proxy: { '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true } },
  },
})
