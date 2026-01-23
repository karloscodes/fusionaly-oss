import { createInertiaApp, router } from '@inertiajs/react'
import { createRoot } from 'react-dom/client'
import './index.css'

// Import all pages
import { Home } from './pages/Home'
import { Login } from './pages/Login'
import Dashboard from './pages/Dashboard'
import PublicDashboard from './pages/PublicDashboard'
import Websites from './pages/Websites'
import WebsiteNew from './pages/WebsiteNew'
import WebsiteSetup from './pages/WebsiteSetup'
import WebsiteEdit from './pages/WebsiteEdit'
import { Events } from './pages/Events'
import { Lens } from './pages/Lens'
import Onboarding from './pages/Onboarding'
import { AdministrationIngestion } from './pages/AdministrationIngestion'
import { AdministrationAccount } from './pages/AdministrationAccount'
import { AdministrationSystem } from './pages/AdministrationSystem'
import { NotFound } from './pages/NotFound'

// Map of page components
const pages: Record<string, any> = {
  Home,
  Login,
  Dashboard,
  PublicDashboard,
  Websites,
  WebsiteNew,
  WebsiteSetup,
  WebsiteEdit,
  Events,
  Lens,
  Onboarding,
  AdministrationIngestion,
  AdministrationAccount,
  AdministrationSystem,
  NotFound,
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
    background: #00D678;
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
  resolve: (name) => {
    const page = pages[name]
    if (!page) {
      console.error(`Page ${name} not found in Inertia page registry`)
      return pages['NotFound']
    }
    return page
  },
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
