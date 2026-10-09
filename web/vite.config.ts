import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Suppervisor — Dinner, handled.',
        short_name: 'Suppervisor',
        description: 'A local-first dinner planner for the people actually eating.',
        theme_color: '#c94b35',
        background_color: '#f7efe2',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: '/favicon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
        ],
      },
    }),
  ],
  server: {
    host: '0.0.0.0',
    port: 4179,
    proxy: {
      '/api': 'http://127.0.0.1:8091',
    },
  },
})
