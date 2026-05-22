import { useEffect, useState } from "react";
import { cssVarColor, getTheme, onThemeChange, Theme } from "@/lib/theme";

// Chart libraries (Recharts/Vega) take plain color strings, so they can't read
// CSS variables directly. This hook resolves the themed colors a chart needs
// from CSS-var tokens and recomputes them on theme change. A new theme only has
// to define the tokens in index.css — no chart code changes.
export interface ChartColors {
  grid: string; // gridlines, axis lines, hover/cursor fill
  axisText: string; // tick labels
  line: string; // secondary (revenue) line — calm, --c-chart-line
  bar: string | null; // optional bar fill (gradient top), --c-bar; null → use metric color
  barDeep: string | null; // gradient bottom, --c-bar-deep
  barRadius: number; // bar corner radius in px, --c-bar-radius
  theme: Theme; // still used by the Sankey until it moves to tokens
}

function cssVarRaw(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

export function useChartColors(): ChartColors {
  const read = (): ChartColors => {
    const bar = cssVarColor("--c-bar"); // "" when the theme doesn't override the bar
    const barDeep = cssVarColor("--c-bar-deep");
    const radius = cssVarRaw("--c-bar-radius");
    return {
      grid: cssVarColor("--c-gray-200") || "#E5E7EB",
      axisText: cssVarColor("--c-gray-600") || "#4b5563",
      line: cssVarColor("--c-chart-line") || "#4b5563",
      bar: bar || null,
      barDeep: barDeep || bar || null,
      barRadius: radius ? Number(radius) : 4,
      theme: getTheme(),
    };
  };

  const [colors, setColors] = useState<ChartColors>(read);

  useEffect(() => {
    // Re-read after mount (vars are resolved) and on every theme change.
    setColors(read());
    return onThemeChange(() => setColors(read()));
  }, []);

  return colors;
}
