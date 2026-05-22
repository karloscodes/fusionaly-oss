import type { Config } from "tailwindcss";

// Map the neutral palette to CSS-variable channels so every existing utility
// (bg-white, text-gray-900, border-black, …) is theme-aware. The <alpha-value>
// form keeps opacity modifiers working (e.g. bg-white/70). Theme values live in
// src/index.css under :root / [data-theme="dark"] / [data-theme="terminal"].
// Non-neutral colors (green, red, amber, …) keep Tailwind's defaults.
const channel = (name: string) => `rgb(var(${name}) / <alpha-value>)`;

// One ramp drives gray/neutral/slate — shadcn ui components use `neutral`,
// the app uses `gray`. Mapping all three to the same vars themes them together.
const ramp = {
	50: channel("--c-gray-50"),
	100: channel("--c-gray-100"),
	200: channel("--c-gray-200"),
	300: channel("--c-gray-300"),
	400: channel("--c-gray-400"),
	500: channel("--c-gray-500"),
	600: channel("--c-gray-600"),
	700: channel("--c-gray-700"),
	800: channel("--c-gray-800"),
	900: channel("--c-gray-900"),
	950: channel("--c-gray-900"),
};

export default {
	darkMode: ["class"],
	content: [
		"./pages/**/*.{ts,tsx}",
		"./components/**/*.{ts,tsx}",
		"./app/**/*.{ts,tsx}",
		"./src/**/*.{ts,tsx}",
	],
	prefix: "",
	theme: {
		extend: {
			colors: {
				white: channel("--c-white"),
				black: channel("--c-black"),
				gray: ramp,
				neutral: ramp,
				slate: ramp,
			},
			// Corner radius scales with the --c-radius-scale token: 1 keeps the
			// stock values (Light/Dark), 0 squares everything off (Terminal). A
			// theme sets the scale as data — no per-theme CSS override needed.
			borderRadius: {
				none: "0px",
				sm: "calc(0.125rem * var(--c-radius-scale, 1))",
				DEFAULT: "calc(0.25rem * var(--c-radius-scale, 1))",
				md: "calc(0.375rem * var(--c-radius-scale, 1))",
				lg: "calc(0.5rem * var(--c-radius-scale, 1))",
				xl: "calc(0.75rem * var(--c-radius-scale, 1))",
				"2xl": "calc(1rem * var(--c-radius-scale, 1))",
				"3xl": "calc(1.5rem * var(--c-radius-scale, 1))",
				full: "calc(9999px * var(--c-radius-scale, 1))",
			},
		},
	},
} satisfies Config;
