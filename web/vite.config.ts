import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// La build tiene que ser estática: el binario Go embebe web/dist con go:embed
// y la sirve tal cual. Rutas por hash, sin SSR, base relativa.
export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsInlineLimit: 4096,
  },
  server: {
    port: 5173,
    proxy: {
      // En desarrollo, contra el servidor Go de verdad si está levantado.
      '/api': { target: 'http://127.0.0.1:7870', changeOrigin: true, ws: true },
    },
  },
})
