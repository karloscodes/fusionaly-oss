import path from "path"
import react from "@vitejs/plugin-react-swc"
import { defineConfig } from "vite"
// import type { Plugin } from 'vite'

// const goHTML = (): Plugin => {
//   return {
//     name: 'vite:go-html-template',
//     transformIndexHtml: {
//       enforce: 'pre',
//       transform(html: string) {
//         const rootDivClose = html.lastIndexOf('</div>')
//         if (rootDivClose === -1) return html

//         const before = html.slice(0, rootDivClose + 6)
//         const after = html.slice(rootDivClose + 6)

//         const customScript = `
//           <!-- Page data -->
//           <script type="text/javascript">
//             window.__PAGE_DATA__ = {{.PageData}};
//             window.__PAGE__ = "{{.Page}}";
//             {{if .Error}}
//             window.__ERROR__ = {{.Error}};
//             {{else}}
//             window.__ERROR__ = null;
//             {{end}} 
//           </script>`

//         return before + customScript + after
//       }
//     }
//   }
// }

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    // Generate manifest for Inertia.js to find hashed assets
    manifest: true,
    // Increase limit to 1200 KB since charts library (Vega) is inherently large
    // We've already optimized by splitting into separate chunks
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      input: {
        inertia: path.resolve(__dirname, 'src/inertia.tsx'),
      },
      output: {
        // Use content hashes in filenames for cache busting
        // When code changes, the hash changes, forcing browsers to fetch new files
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]',
        manualChunks: {
          // Split vendor chunks for better caching and parallel loading
          'react-vendor': ['react', 'react-dom'],
          'charts': ['recharts', 'vega', 'vega-lite', 'vega-embed', 'react-vega'],
          'ui': [
            '@radix-ui/react-checkbox',
            '@radix-ui/react-dialog',
            '@radix-ui/react-dropdown-menu',
            '@radix-ui/react-label',
            '@radix-ui/react-popover',
            '@radix-ui/react-progress',
            '@radix-ui/react-select',
            '@radix-ui/react-separator',
            '@radix-ui/react-slot',
            '@radix-ui/react-tabs',
            '@radix-ui/react-tooltip',
          ],
        },
      },
    },
  }
})
