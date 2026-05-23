// Theme system. Themes are pure CSS-variable swaps: the Tailwind config maps
// white/black/gray-* to `rgb(var(--c-*) / <alpha-value>)`, and each theme below
// redefines those channels in index.css. So existing utilities (bg-white,
// text-gray-900, …) become theme-aware with no component changes.

export type Theme = "light" | "dark" | "phosphor" | "catppuccin" | "gruvbox";

export const THEMES: { id: Theme; label: string }[] = [
  { id: "light", label: "Light" },
  { id: "dark", label: "Dark" },
  // Terminal-style themes (monospace, square corners, btop bars).
  { id: "phosphor", label: "Phosphor" }, // green-CRT vibe
  { id: "catppuccin", label: "Catppuccin" }, // Mocha palette
  { id: "gruvbox", label: "Gruvbox" }, // warm retro palette
];

const STORAGE_KEY = "fusionaly-theme";
const CHANGE_EVENT = "fusionaly-theme-change";

function isTheme(v: string | null): v is Theme {
  return THEMES.some((t) => t.id === v);
}

export function getTheme(): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (isTheme(stored)) return stored;
  } catch {
    // localStorage unavailable (private mode, etc.) — fall back to light.
  }
  return "light";
}

// applyTheme sets the <html data-theme> attribute, persists the choice, and
// notifies subscribers (charts re-read their colors). Light is the default,
// so it clears the attribute rather than setting data-theme="light".
export function applyTheme(theme: Theme) {
  const el = document.documentElement;
  if (theme === "light") el.removeAttribute("data-theme");
  else el.setAttribute("data-theme", theme);

  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // ignore persistence failures
  }
  window.dispatchEvent(new CustomEvent(CHANGE_EVENT));
}

export function onThemeChange(cb: () => void): () => void {
  window.addEventListener(CHANGE_EVENT, cb);
  return () => window.removeEventListener(CHANGE_EVENT, cb);
}

// cssVarColor reads a "R G B" channel variable and returns an rgb() string
// usable by chart libraries (Recharts/Vega) that take plain color strings.
export function cssVarColor(name: string): string {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  if (!raw) return "";
  // Channel form "229 231 235" → "rgb(229 231 235)"; pass through anything else.
  return /^\d/.test(raw) ? `rgb(${raw})` : raw;
}

// cssVarList reads a comma-separated variable (e.g. a color ramp) into a list.
// Lets a theme declare a whole palette as data, e.g. --c-flow: #0B7A3E, #16C062.
export function cssVarList(name: string): string[] {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return raw ? raw.split(",").map((s) => s.trim()).filter(Boolean) : [];
}
