import { defineConfig, type Plugin } from 'vite'
import preact from '@preact/preset-vite' // 使用 preact 插件
import { viteSingleFile } from 'vite-plugin-singlefile'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

const backendPort = process.env.BACKEND_PORT
const backendUrl = backendPort ? `http://localhost:${backendPort}` : 'http://localhost:8013'

// Vite plugin: intercept /wails/runtime.js imports and redirect to a stub.
// This lets the auto-generated binding files work in Vite dev mode without modification.
// On Android / Wails builds the real runtime is injected by the platform.
function wailsRuntimeStub(): Plugin {
  return {
    name: 'wails-runtime-stub',
    resolveId(id: string) {
      if (id === '/wails/runtime.js') {
        return resolve(__dirname, 'src/wails-runtime-stub.ts');
      }
    },
  };
}

export default defineConfig({
  ...(backendPort ? { define: { 'import.meta.env.VITE_API_URL': JSON.stringify(backendUrl) } } : {}),
  build: {
    assetsInlineLimit: Number.MAX_SAFE_INTEGER,
    rollupOptions: {
      external: ['/wails/runtime.js'],
    },
  },
  plugins: [
    tailwindcss(),
    preact(), // 代替 react()
    viteSingleFile(),
    wailsRuntimeStub(),
  ],
  resolve: {
    alias: {
      'react': 'preact/compat',
      'react-dom': 'preact/compat',
      'react/jsx-runtime': 'preact/jsx-runtime',
      '@wailsio/runtime': '/wails/runtime.js',
    },
  },
  assetsInclude: ['**/*.toml', '**/*.woff2'], // 允许 TOML 文件作为资产导入
  server: {
    proxy: {
      '/api': {
        target: backendUrl,
        changeOrigin: true,
      },
    },
  },
})