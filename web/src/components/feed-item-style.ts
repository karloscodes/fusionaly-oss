// How each activity feed item type looks. Home and the dashboard's
// "What's new" card both use these.

// The design's compact glyph per item type, with a tone for its color.
export const itemTypeGlyphs: Record<string, { glyph: string; tone: string }> = {
  traffic_spike: { glyph: "↑", tone: "text-emerald-600" },
  traffic_drop: { glyph: "↓", tone: "text-rose-500" },
  new_referrer: { glyph: "↗", tone: "text-sky-600" },
  best_sources: { glyph: "≡", tone: "text-sky-600" },
  goal_hit: { glyph: "◎", tone: "text-violet-600" },
  milestone: { glyph: "★", tone: "text-amber-600" },
  trending_content: { glyph: "¶", tone: "text-sky-600" },
  dropping_pages: { glyph: "↘", tone: "text-rose-500" },
  page_problem: { glyph: "!", tone: "text-amber-600" },
  daily_summary: { glyph: "Σ", tone: "text-gray-900" },
  monthly_summary: { glyph: "Σ", tone: "text-gray-900" },
};

// "09:14" for today, "Sep 21" before that.
export function feedTime(iso: string): string {
  const d = new Date(iso);
  const today = new Date();
  return d.toDateString() === today.toDateString()
    ? d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" })
    : d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}
