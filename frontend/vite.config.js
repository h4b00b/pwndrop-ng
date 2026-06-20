import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// During `npm run dev`, API calls are proxied to a running pwndrop backend.
// Adjust the target to wherever your backend listens (see pwndrop.ini).
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': { target: 'http://127.0.0.1:8088', changeOrigin: true },
    },
  },
})
