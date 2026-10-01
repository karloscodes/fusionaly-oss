import { createInertiaApp, router } from '@inertiajs/react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { getTheme } from './lib/theme'

// Re-apply the saved theme on every full page load. The served HTML shell
// (cartridge's fallback template) has no inline theme script, so without this a
// browser refresh would drop back to Light. Runs before React mounts.
const savedTheme = getTheme()
if (savedTheme !== 'light') {
  document.documentElement.setAttribute('data-theme', savedTheme)
}

// Each page loads on demand, so a page ships only the code it uses (the
// charts libraries reach the dashboard and Lens only). Inertia waits for the
// page module before it swaps pages, so nothing renders differently.
const pageModules = import.meta.glob('./pages/*.tsx')

async function loadPage(name: string) {
  const load = pageModules[`./pages/${name}.tsx`]
  if (!load) {
    console.error(`Page ${name} not found in Inertia page registry`)
    const notFound: any = await pageModules['./pages/NotFound.tsx']()
    return notFound.NotFound
  }
  const mod: any = await load()
  return mod[name] ?? mod.default
}

// Create and inject loading progress bar
function createProgressBar() {
  const progressBar = document.createElement('div')
  progressBar.id = 'inertia-progress'
  progressBar.style.cssText = `
    position: fixed;
    top: 0;
    left: 0;
    height: 3px;
    background: #00D1FF;
    transition: width 0.3s ease;
    z-index: 9999;
    width: 0%;
    opacity: 0;
  `
  document.body.appendChild(progressBar)
  return progressBar
}

// Initialize progress bar when DOM is ready
let progressBar: HTMLElement | null = null
let timeout: ReturnType<typeof setTimeout> | null = null

function showProgress() {
  if (!progressBar) {
    progressBar = createProgressBar()
  }
  if (timeout) {
    clearTimeout(timeout)
    timeout = null
  }
  progressBar.style.opacity = '1'
  progressBar.style.width = '15%'

  // Animate progress incrementally
  setTimeout(() => {
    if (progressBar && progressBar.style.opacity === '1') {
      progressBar.style.width = '50%'
    }
  }, 100)
  setTimeout(() => {
    if (progressBar && progressBar.style.opacity === '1') {
      progressBar.style.width = '80%'
    }
  }, 500)
}

function hideProgress() {
  if (!progressBar) return
  progressBar.style.width = '100%'
  timeout = setTimeout(() => {
    if (progressBar) {
      progressBar.style.opacity = '0'
      setTimeout(() => {
        if (progressBar) {
          progressBar.style.width = '0%'
        }
      }, 200)
    }
  }, 100)
}

// Setup Inertia event listeners for progress
router.on('start', () => showProgress())
router.on('finish', () => hideProgress())

// Track if app has been initialized to prevent double-mounting
let appInitialized = false

createInertiaApp({
  // Fusionaly draws its own progress bar (above). Inertia v3 renders its
  // built-in bar on top of it, so turn the built-in one off.
  progress: false,
  resolve: (name) => loadPage(name),
  setup({ el, App, props }) {
    // Prevent double initialization which can cause nested page rendering
    if (appInitialized) {
      console.warn('Inertia app already initialized, skipping duplicate setup')
      return
    }
    appInitialized = true
    createRoot(el).render(<App {...props} />)
  },
})
