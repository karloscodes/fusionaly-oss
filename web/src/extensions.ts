/**
 * Pro Extension Points
 *
 * This file exports all components and types that Pro can use to extend OSS.
 * Import from this single file instead of reaching into individual page files.
 *
 * DO NOT add Pro-specific code here - this is OSS only.
 */

// Administration page content components (for Pro to wrap with custom layout)
export { AdministrationAccountContent } from "./pages/AdministrationAccount";
export { AdministrationSystemContent } from "./pages/AdministrationSystem";
export { AdministrationIngestionContent } from "./pages/AdministrationIngestion";

// Dashboard component with insightsSlot prop for Pro content injection
export { Dashboard } from "./components/dashboard";
