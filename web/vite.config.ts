import { writeFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// The Go server listens on 7777; in development Vite serves the UI and
// forwards API and attachment requests to it.
const backend = 'http://127.0.0.1:7777'

export default defineConfig({
  plugins: [
    svelte(),
    {
      // go:embed needs dist/ to exist with at least one file even before the
      // first build, so keep a tracked placeholder after Vite empties it.
      name: 'keep-dist',
      closeBundle() {
        writeFileSync('dist/.gitkeep', '')
      },
    },
  ],
  server: {
    proxy: {
      '/api': backend,
      '/files': backend,
    },
  },
})
