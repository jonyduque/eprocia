import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { readFileSync } from 'node:fs'

// A versão mostrada no rodapé é a mesma que o wails build grava no exe.
const wails = JSON.parse(readFileSync(new URL('../wails.json', import.meta.url), 'utf8'))

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  define: {
    __APP_VERSION__: JSON.stringify(wails.info?.productVersion ?? '')
  }
})
