import { useState } from "react";
import { usePage, Link, router } from "@inertiajs/react";
import {
  Plus,
  MoreHorizontal,
  Code,
  Trash2,
  Copy,
  Check,
  ChevronDown,
  Settings,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AdminLayout } from "@/components/admin-layout";
import { itemTypeGlyphs, feedTime } from "@/components/feed-item-style";
import { cn } from "@/lib/utils";

interface FeedItem {
  id: number;
  websiteId: number;
  itemType: string;
  title: string;
  description: string;
  detectedAt: string;
  websiteDomain: string;
  metadata?: string;
}

// Filter groups for the feed, by item type.
const TYPE_GROUPS: { key: string; label: string; types: string[] }[] = [
  { key: "traffic", label: "Traffic", types: ["traffic_spike", "traffic_drop"] },
  { key: "sources", label: "Sources", types: ["new_referrer", "best_sources"] },
  { key: "pages", label: "Pages", types: ["trending_content", "dropping_pages", "page_problem"] },
  { key: "goals", label: "Goals", types: ["goal_hit", "milestone"] },
  { key: "summaries", label: "Summaries", types: ["daily_summary", "monthly_summary"] },
];

function parseMetadata(raw?: string): Record<string, unknown> | null {
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

interface Website {
  id: number;
  domain: string;
  event_count?: number;
  daily_visitors?: number[]; // last 15 full days, oldest first
}

// Each site keeps one color for its dot in the feed and on its card.
const SITE_COLORS = ["rgb(var(--c-accent))", "#2563eb", "#7c3aed", "#d97706", "#db2777", "#059669"];
const siteColor = (websites: Website[], id: number) =>
  SITE_COLORS[Math.max(0, websites.findIndex((w) => w.id === id)) % SITE_COLORS.length];

// Bold the lead of a feed description, as in the design: a leading number
// ("412 visitors…") or the name before a colon ("/pricing: 64 visitors").
function Description({ text }: { text: string }) {
  const m = text.match(/^([\d,.]+|[^\s:]+(?=:))(.*)$/);
  if (!m) return <>{text}</>;
  return (
    <>
      <b className="font-mono text-[12.5px] font-semibold text-gray-900">{m[1]}</b>
      {m[2]}
    </>
  );
}

interface CalendarDay {
  date: string;
  count: number;
}

interface HomeProps {
  feedItems: FeedItem[];
  websites: Website[];
  calendarData: CalendarDay[];
  totalVisitors: number;
  [key: string]: any;
}

// The date under each group label: "May 23", "May 22", "May 16–21".
function groupDate(label: string): string {
  const day = (offset: number) => {
    const d = new Date();
    d.setDate(d.getDate() - offset);
    return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
  };
  if (label === "Today") return day(0);
  if (label === "Yesterday") return day(1);
  if (label === "Last 7 days") return `${day(7)}–${day(2).split(" ")[1]}`;
  return "";
}

function groupByDate(items: FeedItem[]): Record<string, FeedItem[]> {
  const groups: Record<string, FeedItem[]> = {};
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000);
  const weekAgo = new Date(today.getTime() - 7 * 24 * 60 * 60 * 1000);

  for (const item of items) {
    const itemDate = new Date(item.detectedAt);
    const itemDay = new Date(itemDate.getFullYear(), itemDate.getMonth(), itemDate.getDate());

    let label: string;
    if (itemDay.getTime() >= today.getTime()) {
      label = "Today";
    } else if (itemDay.getTime() >= yesterday.getTime()) {
      label = "Yesterday";
    } else if (itemDay.getTime() >= weekAgo.getTime()) {
      label = "Last 7 days";
    } else {
      label = "Older";
    }

    if (!groups[label]) {
      groups[label] = [];
    }
    groups[label].push(item);
  }

  return groups;
}

function FeedItemRow({ item, color }: { item: FeedItem; color: string }) {
  const g = itemTypeGlyphs[item.itemType] || { glyph: "•", tone: "text-gray-900" };
  const meta = item.itemType === "monthly_summary" ? parseMetadata(item.metadata) : null;
  const topPages = (meta?.topPages as { pathname: string; visitors: number }[] | undefined) || [];
  const topSources = (meta?.topSources as { hostname: string; visitors: number }[] | undefined) || [];

  return (
    <Link
      href={`/admin/websites/${item.websiteId}/dashboard`}
      className="group grid grid-cols-[30px_1fr_auto] gap-3.5 items-start py-3 px-2.5 -mx-2.5 rounded-lg hover:bg-gray-50 transition-colors"
    >
      <span
        className={cn("w-[30px] h-[30px] rounded flex items-center justify-center font-mono text-sm font-bold bg-[var(--databar)]", g.tone)}
        aria-hidden="true"
      >
        {g.glyph}
      </span>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-gray-900">{item.title}</p>
        <p className="text-[13.5px] text-gray-500 mt-0.5"><Description text={item.description} /></p>
        {(topPages.length > 0 || topSources.length > 0) && (
          <div className="mt-2.5 grid grid-cols-1 sm:grid-cols-2 gap-3 rounded-lg border border-gray-200 bg-white px-3.5 py-3">
            {[
              { title: "Top pages", rows: topPages.slice(0, 3).map((p) => [p.pathname, p.visitors] as const) },
              { title: "Top sources", rows: topSources.slice(0, 3).map((p) => [p.hostname, p.visitors] as const) },
            ].map((col) => col.rows.length > 0 && (
              <div key={col.title}>
                <p className="text-[11px] font-semibold uppercase tracking-wide text-gray-500 mb-1">{col.title}</p>
                {col.rows.map(([name, n]) => (
                  <div key={name} className="flex justify-between gap-2 text-xs py-0.5">
                    <span className="truncate text-gray-900">{name}</span>
                    <span className="font-mono text-gray-500">{formatCount(n)}</span>
                  </div>
                ))}
              </div>
            ))}
          </div>
        )}
        <p className="mt-1.5 flex flex-wrap items-center gap-x-2.5 font-mono text-[11.5px] text-gray-500">
          <span className="inline-flex items-center gap-1.5">
            <span className="w-[7px] h-[7px] rounded-full" style={{ background: color }} aria-hidden="true" />
            {item.websiteDomain}
          </span>
          <span aria-hidden="true">·</span>
          <span>{feedTime(item.detectedAt)}</span>
          <span aria-hidden="true">·</span>
          <span>{item.itemType}</span>
        </p>
      </div>
      <span className="text-gray-400 group-hover:text-gray-900 pt-1.5" aria-hidden="true">→</span>
    </Link>
  );
}

function FilterChip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        "h-7 px-3 text-xs font-medium border rounded-full whitespace-nowrap transition-colors",
        active ? "bg-black text-white border-black" : "bg-white text-gray-500 border-gray-200 hover:text-gray-900 hover:border-gray-400"
      )}
    >
      {children}
    </button>
  );
}

function formatCount(n: number): string {
  if (n >= 1000000) {
    const val = (n / 1000000).toFixed(1);
    return `${val.replace(/\.0$/, "")}M`;
  }
  if (n >= 1000) {
    const val = (n / 1000).toFixed(1);
    return `${val.replace(/\.0$/, "")}k`;
  }
  return n.toString();
}

interface SiteCardProps {
  site: Website;
  onShowScript: (site: Website) => void;
  onDelete: (site: Website) => void;
}

function SiteCard({ site, color, onShowScript, onDelete }: SiteCardProps & { color: string }) {
  const days = site.daily_visitors || [];
  const yesterday = days.length ? days[days.length - 1] : 0;
  const before = days.slice(0, -1);
  const usual = before.length ? before.reduce((a, b) => a + b, 0) / before.length : 0;
  const change = usual > 0 ? Math.round(((yesterday - usual) / usual) * 100) : null;
  const spark = days.slice(-14);
  const sparkMax = Math.max(...spark, 1);

  return (
    <div className="group relative bg-white border border-black rounded-xl">
      <Link href={`/admin/websites/${site.id}/dashboard`} className="block px-4 pt-4 pb-3.5">
        <div className="flex items-center gap-2 pr-16">
          <span className="w-2 h-2 rounded-full shrink-0" style={{ background: color }} aria-hidden="true" />
          <span className="truncate text-[14.5px] font-semibold text-gray-900">{site.domain}</span>
        </div>
        <div className="mt-3 flex items-end justify-between gap-3">
          <div>
            <p className="text-2xl font-bold leading-none tracking-tight text-gray-900">{yesterday.toLocaleString()}</p>
            <p className="mt-1 font-mono text-[11.5px] text-gray-500">visitors yesterday</p>
          </div>
          {spark.some((v) => v > 0) && (
            <div className="flex items-end gap-0.5 h-8" aria-hidden="true">
              {spark.map((v, i) => (
                <span key={i} className="w-[5px] rounded-[1px] bg-[rgb(var(--c-accent))]" style={{ height: Math.max(2, Math.round((v / sparkMax) * 32)) }} />
              ))}
            </div>
          )}
        </div>
        <div className="mt-3 flex justify-between gap-2 font-mono text-[11.5px] text-gray-500">
          <span className={change === null ? "" : change >= 0 ? "text-emerald-600" : "text-rose-500"}>
            {change === null ? `${formatCount(site.event_count || 0)} events` : `${change >= 0 ? "+" : "−"}${Math.abs(change)}% vs usual`}
          </span>
          <span className="group-hover:text-gray-900">Open dashboard →</span>
        </div>
      </Link>

      <div className="absolute top-3 right-3 flex items-center gap-0.5">
        <button
          type="button"
          onClick={() => onShowScript(site)}
          className="w-[30px] h-[30px] grid place-items-center rounded text-gray-500 hover:text-gray-900 hover:bg-gray-100"
          aria-label={`Show tracking script for ${site.domain}`}
        >
          <Code className="h-4 w-4" />
        </button>
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="w-[30px] h-[30px] grid place-items-center rounded text-gray-500 hover:text-gray-900 hover:bg-gray-100 opacity-0 group-hover:opacity-100 focus:opacity-100"
              aria-label={`More actions for ${site.domain}`}
            >
              <MoreHorizontal className="h-4 w-4" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem asChild>
              <Link href={`/admin/websites/${site.id}/edit`}>
                <Settings className="h-4 w-4 mr-2" />
                Website settings
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => onDelete(site)} className="text-red-600 focus:text-red-600">
              <Trash2 className="h-4 w-4 mr-2" />
              Delete site
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}

function VisitorCalendar({
  data,
  total,
}: {
  data: CalendarDay[];
  total: number;
}) {
  const countMap = new Map(data.map((d) => [d.date, d.count]));
  const maxCount = Math.max(...data.map((d) => d.count), 1);

  const today = new Date();
  today.setHours(0, 0, 0, 0);

  // Generate last 365 days
  const days: { date: Date; count: number }[] = [];
  for (let i = 364; i >= 0; i--) {
    const date = new Date(today);
    date.setDate(date.getDate() - i);
    const dateStr = date.toISOString().split("T")[0];
    days.push({ date, count: countMap.get(dateStr) || 0 });
  }

  // Group by weeks (columns)
  const weeks: { date: Date; count: number }[][] = [];
  let currentWeek: { date: Date; count: number }[] = [];

  // Pad first week
  const firstDayOfWeek = days[0].date.getDay();
  for (let i = 0; i < firstDayOfWeek; i++) {
    currentWeek.push({ date: new Date(0), count: -1 });
  }

  for (const day of days) {
    currentWeek.push(day);
    if (currentWeek.length === 7) {
      weeks.push(currentWeek);
      currentWeek = [];
    }
  }
  if (currentWeek.length > 0) {
    weeks.push(currentWeek);
  }

  // Month labels - label every month the window spans, at the column where
  // that month first appears. We record a label whenever the month changes so
  // no month is ever skipped.
  const monthLabels: { label: string; weekIndex: number }[] = [];
  let lastMonth = -1; // last *labeled* month
  let lastLabelCol = -100; // column of the last label
  const minLabelGap = 3; // weeks between labels so they don't overlap
  weeks.forEach((week, weekIndex) => {
    const firstValidDay = week.find((d) => d.count >= 0);
    if (firstValidDay) {
      const month = firstValidDay.date.getMonth();
      // Label the first week of a new month that's far enough from the previous
      // label. Keeping lastMonth as the last *labeled* month means a month whose
      // first week is too close still gets labeled a week or two later (never
      // dropped), while adjacent labels never overlap.
      if (month !== lastMonth && weekIndex - lastLabelCol >= minLabelGap) {
        monthLabels.push({
          label: firstValidDay.date.toLocaleDateString("en-US", { month: "short" }),
          weekIndex,
        });
        lastMonth = month;
        lastLabelCol = weekIndex;
      }
    }
  });

  // Cell background as the theme accent at increasing opacity, so the heat map
  // matches each theme (green, mauve, …). Empty days use a faint themed gray.
  const cellColor = (count: number): string => {
    if (count < 0) return "transparent";
    if (count === 0) return "rgb(var(--c-gray-200))";
    const ratio = count / maxCount;
    if (ratio < 0.25) return "rgb(var(--c-accent) / 0.3)";
    if (ratio < 0.5) return "rgb(var(--c-accent) / 0.55)";
    if (ratio < 0.75) return "rgb(var(--c-accent) / 0.8)";
    return "rgb(var(--c-accent))";
  };

  // Legend swatches: empty → 4 ascending accent levels.
  const legendColors = [
    "rgb(var(--c-gray-200))",
    "rgb(var(--c-accent) / 0.3)",
    "rgb(var(--c-accent) / 0.55)",
    "rgb(var(--c-accent) / 0.8)",
    "rgb(var(--c-accent))",
  ];

  const [hoveredDay, setHoveredDay] = useState<{
    date: Date;
    count: number;
    x: number;
    y: number;
  } | null>(null);

  // Three numbers next to the grid: busiest day, daily average, this week.
  const realDays = days.filter((d) => d.count >= 0);
  const busiest = realDays.reduce((a, b) => (b.count > a.count ? b : a), realDays[0]);
  const sumOf = (list: { count: number }[]) => list.reduce((acc, d) => acc + d.count, 0);
  const thisWeek = sumOf(realDays.slice(-7));
  const lastWeek = sumOf(realDays.slice(-14, -7));
  const weekChange = lastWeek > 0 ? Math.round(((thisWeek - lastWeek) / lastWeek) * 100) : null;
  const stats = [
    {
      label: "Busiest day",
      value: busiest && busiest.count > 0 ? busiest.count.toLocaleString() : "—",
      note: busiest && busiest.count > 0 ? busiest.date.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" }) : "no visitors yet",
      tone: "neutral",
    },
    {
      label: "Daily average",
      value: Math.round(sumOf(realDays) / Math.max(realDays.length, 1)).toLocaleString(),
      note: "visitors per day",
      tone: "neutral",
    },
    {
      label: "This week",
      value: thisWeek.toLocaleString(),
      note: weekChange === null ? "no data last week" : `${weekChange >= 0 ? "+" : ""}${weekChange}% vs last week`,
      tone: weekChange === null ? "neutral" : weekChange >= 0 ? "up" : "down",
    },
  ];

  // Design: 13px cells with 3px gaps, weekday labels on the left.
  const cellSize = 13;
  const cellGap = 3;
  const weekWidth = cellSize + cellGap;
  const weekdayLabels = ["", "Mon", "", "Wed", "", "Fri", ""];

  return (
    <section className="bg-white border border-black rounded-xl p-4 sm:p-5">
      <div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-1">
        <h2 className="text-base font-semibold text-gray-900">Visitors, last 12 months</h2>
        <span className="font-mono text-xs text-gray-500">all sites · {total.toLocaleString()} total</span>
      </div>

      <div className="mt-3.5 flex flex-col lg:flex-row gap-7 lg:items-start">
        <div className="min-w-0 flex-1">
          <div className="overflow-x-auto pb-1">
            <div className="w-max">
              {/* Month labels */}
              <div className="relative h-4 ml-8 font-mono text-[10px] text-gray-500">
                {monthLabels.map((m, i) => (
                  <span key={i} className="absolute" style={{ left: m.weekIndex * weekWidth }}>
                    {m.label}
                  </span>
                ))}
              </div>

              <div className="flex gap-[3px]">
                {/* Weekday labels */}
                <div className="flex flex-col gap-[3px] w-[29px] font-mono text-[10px] leading-[13px] text-gray-500">
                  {weekdayLabels.map((d, i) => (
                    <span key={i} style={{ height: cellSize }}>{d}</span>
                  ))}
                </div>
                {weeks.map((week, weekIndex) => (
                  <div key={weekIndex} className="flex flex-col gap-[3px]">
                    {week.map((day, dayIndex) => (
                      <div
                        key={dayIndex}
                        className={cn("rounded-[2px]", day.count >= 0 && "cursor-pointer")}
                        style={{ width: cellSize, height: cellSize, backgroundColor: cellColor(day.count) }}
                        onMouseEnter={(e) => {
                          if (day.count >= 0) {
                            const rect = e.currentTarget.getBoundingClientRect();
                            setHoveredDay({ ...day, x: rect.left, y: rect.top });
                          }
                        }}
                        onMouseLeave={() => setHoveredDay(null)}
                      />
                    ))}
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Legend */}
          <div className="mt-2.5 flex items-center gap-[3px] font-mono text-[10.5px] text-gray-500">
            <span className="mr-1">less</span>
            {legendColors.map((c, i) => (
              <span key={i} className="w-[11px] h-[11px] rounded-[2px]" style={{ backgroundColor: c }} />
            ))}
            <span className="ml-1">more</span>
          </div>
        </div>

        <dl className="grid grid-cols-3 lg:grid-cols-1 gap-3.5 lg:w-[220px] shrink-0">
          {stats.map((st) => (
            <div key={st.label} className="border-l border-gray-200 pl-3.5">
              <dt className="text-[11px] font-semibold uppercase tracking-[0.08em] text-gray-500">{st.label}</dt>
              <dd className="text-xl font-bold tracking-tight text-gray-900">{st.value}</dd>
              <dd className={cn("font-mono text-[11.5px]", st.tone === "up" ? "text-emerald-600" : st.tone === "down" ? "text-rose-500" : "text-gray-500")}>{st.note}</dd>
            </div>
          ))}
        </dl>
      </div>

      {/* Tooltip */}
      {hoveredDay && (
        <div
          className="fixed z-50 bg-black text-white text-xs px-2 py-1 rounded shadow-lg pointer-events-none whitespace-nowrap"
          style={{ left: hoveredDay.x - 40, top: hoveredDay.y - 32 }}
        >
          {hoveredDay.count} visitor{hoveredDay.count !== 1 ? "s" : ""} on{" "}
          {hoveredDay.date.toLocaleDateString("en-US", { month: "short", day: "numeric" })}
        </div>
      )}
    </section>
  );
}

export const Home = () => {
  const { props } = usePage<HomeProps>();

  const feedItems = props.feedItems || [];
  const websites = props.websites || [];
  const calendarData = props.calendarData || [];
  const totalVisitors = props.totalVisitors || 0;

  const [websiteToDelete, setWebsiteToDelete] = useState<Website | null>(null);
  const [showIntegrationHelp, setShowIntegrationHelp] = useState(false);
  const [selectedWebsiteForIntegration, setSelectedWebsiteForIntegration] = useState<Website | null>(null);
  const [copiedScript, setCopiedScript] = useState(false);
  const [filterSiteId, setFilterSiteId] = useState<number | null>(null);
  const [filterType, setFilterType] = useState<string | null>(null);
  const FEED_PAGE_SIZE = 25;
  const [visibleCount, setVisibleCount] = useState(FEED_PAGE_SIZE);

  const typeGroup = TYPE_GROUPS.find((g) => g.key === filterType);
  const filteredFeedItems = feedItems.filter(
    (item) =>
      (!filterSiteId || item.websiteId === filterSiteId) &&
      (!typeGroup || typeGroup.types.includes(item.itemType))
  );
  const visitorsYesterday = websites.reduce((acc, w) => acc + (w.daily_visitors?.[w.daily_visitors.length - 1] ?? 0), 0);
  const newToday = (groupByDate(feedItems)["Today"] || []).length;

  // Busiest sites first on the cards (by visitors over the last 15 days).
  const recentVisitors = (w: Website) => (w.daily_visitors || []).reduce((a, b) => a + b, 0);
  const sitesByTraffic = [...websites].sort((a, b) => recentVisitors(b) - recentVisitors(a));

  // Site chips only for sites that have something in the feed.
  const sitesInFeed = websites.filter((site) => feedItems.some((item) => item.websiteId === site.id));
  const pickFilter = (site: number | null, type: string | null) => {
    setFilterSiteId(site);
    setFilterType(type);
    setVisibleCount(FEED_PAGE_SIZE);
  };
  const visibleFeedItems = filteredFeedItems.slice(0, visibleCount);
  const hasMoreItems = visibleCount < filteredFeedItems.length;
  const remainingCount = filteredFeedItems.length - visibleCount;

  const groupedItems = groupByDate(visibleFeedItems);
  const groupOrder = ["Today", "Yesterday", "Last 7 days", "Older"];

  const handleDeleteWebsite = () => {
    if (!websiteToDelete) return;
    router.post(
      `/admin/websites/${websiteToDelete.id}/delete`,
      {},
      {
        onSuccess: () => setWebsiteToDelete(null),
      }
    );
  };

  return (
    <AdminLayout currentPath="/admin">
      <div className="py-6 flex flex-col gap-6">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Home</h1>
            <p className="text-sm text-gray-500 mt-1">
              {websites.length} {websites.length === 1 ? "site" : "sites"} · {visitorsYesterday.toLocaleString()} visitors yesterday · {newToday} new {newToday === 1 ? "item" : "items"} today
            </p>
          </div>
          <Link
            href="/admin/websites/new"
            className="text-sm px-3 py-2 bg-black text-white rounded hover:bg-gray-800 flex items-center gap-1 font-medium transition-colors"
          >
            <Plus className="w-4 h-4" />
            Add site
          </Link>
        </div>

        {/* Visitor calendar with its three numbers */}
        <VisitorCalendar data={calendarData} total={totalVisitors} />

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-stretch">
          {/* What's new: the activity feed */}
          <section className="lg:col-span-2 bg-white border border-black rounded-xl flex flex-col">
            <div className="px-4 sm:px-5 py-4 border-b border-gray-200 flex flex-col gap-3">
              <div>
                <h2 className="text-base font-semibold text-gray-900 flex flex-wrap items-baseline gap-x-2.5">
                  What's new <span className="font-mono text-xs font-normal text-gray-500">updated daily from yesterday's data</span>
                </h2>
                <p className="text-[13px] text-gray-500 mt-0.5">
                  Spikes, new referrers, milestones. Small sites stay quiet until something real happens.
                </p>
              </div>
              {feedItems.length > 0 && (
                <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Filter the feed">
                  {sitesInFeed.length > 1 && (
                    <>
                      <DropdownMenu modal={false}>
                        <DropdownMenuTrigger asChild>
                          <button
                            type="button"
                            className={cn(
                              "h-7 px-3 inline-flex items-center gap-1.5 text-xs font-medium border rounded-full whitespace-nowrap transition-colors",
                              filterSiteId ? "bg-black text-white border-black" : "bg-white text-gray-900 border-gray-200 hover:border-gray-400"
                            )}
                          >
                            {sitesInFeed.find((w) => w.id === filterSiteId)?.domain ?? "All sites"}
                            <ChevronDown className="w-3 h-3" />
                          </button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="start" className="max-h-72 overflow-y-auto">
                          <DropdownMenuItem onClick={() => pickFilter(null, filterType)} className={!filterSiteId ? "font-semibold" : ""}>
                            All sites
                          </DropdownMenuItem>
                          {sitesInFeed.map((site) => (
                            <DropdownMenuItem
                              key={site.id}
                              onClick={() => pickFilter(site.id, filterType)}
                              className={filterSiteId === site.id ? "font-semibold" : ""}
                            >
                              {site.domain}
                            </DropdownMenuItem>
                          ))}
                        </DropdownMenuContent>
                      </DropdownMenu>
                      <span className="w-px h-5 bg-gray-200 mx-1" aria-hidden="true" />
                    </>
                  )}
                  <FilterChip active={!filterType} onClick={() => pickFilter(filterSiteId, null)}>Everything</FilterChip>
                  {TYPE_GROUPS.map((g) => (
                    <FilterChip key={g.key} active={filterType === g.key} onClick={() => pickFilter(filterSiteId, g.key)}>
                      {g.label}
                    </FilterChip>
                  ))}
                </div>
              )}
            </div>

            {filteredFeedItems.length === 0 ? (
              <p className="flex-1 text-sm text-gray-500 p-5">
                {feedItems.length === 0 ? "Nothing yet." : "Nothing matches these filters."}
              </p>
            ) : (
              <div className="flex-1">
                {groupOrder.map((label) => {
                  const items = groupedItems[label];
                  if (!items || items.length === 0) return null;

                  return (
                    <div key={label} className="grid grid-cols-1 sm:grid-cols-[110px_1fr] border-b border-gray-200 last:border-b-0">
                      <h3 className="px-4 sm:px-5 pt-4 font-mono text-[11px] font-semibold text-gray-500 uppercase tracking-[0.08em]">
                        {label}
                        {groupDate(label) && <span className="block mt-0.5 font-normal normal-case tracking-normal">{groupDate(label)}</span>}
                      </h3>
                      <div className="px-4 sm:pl-0 sm:pr-5 py-1">
                        {items.map((item) => (
                          <FeedItemRow key={item.id} item={item} color={siteColor(websites, item.websiteId)} />
                        ))}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}

            <div className="flex items-center justify-between gap-3 border-t border-gray-200 px-4 sm:px-5 py-3 text-xs text-gray-500">
              <span>
                {visibleFeedItems.length} of {filteredFeedItems.length} items
              </span>
              {hasMoreItems && (
                <button
                  onClick={() => setVisibleCount((prev) => prev + FEED_PAGE_SIZE)}
                  className="text-sm px-3 py-1.5 border rounded text-gray-900 hover:bg-gray-50 transition-colors"
                >
                  Load more ({remainingCount})
                </button>
              )}
            </div>
          </section>

          {/* Your sites */}
          <aside className="flex flex-col gap-3 self-start" aria-label="Your sites">
            <div className="flex items-baseline gap-2.5">
              <h2 className="text-base font-semibold text-gray-900">Your sites</h2>
              <span className="font-mono text-xs text-gray-500">{websites.length}</span>
            </div>
            {websites.length === 0 ? (
              <div className="text-center py-10 px-4 bg-white border border-black rounded-xl">
                <p className="text-sm text-gray-600 mb-4">No websites yet</p>
                <Link
                  href="/admin/websites/new"
                  className="inline-flex items-center px-4 py-2 bg-black text-white text-sm font-medium rounded-lg hover:bg-gray-800 transition-colors"
                >
                  Add your first site
                </Link>
              </div>
            ) : (
              sitesByTraffic.map((site) => (
                <SiteCard
                  key={site.id}
                  site={site}
                  color={siteColor(websites, site.id)}
                  onShowScript={(s) => {
                    setSelectedWebsiteForIntegration(s);
                    setShowIntegrationHelp(true);
                  }}
                  onDelete={setWebsiteToDelete}
                />
              ))
            )}
            <Link
              href="/admin/websites/new"
              className="flex items-center justify-center gap-1 min-h-[64px] rounded-xl border border-dashed border-gray-300 text-[13.5px] font-medium text-gray-500 hover:text-gray-900 hover:border-gray-500 transition-colors"
            >
              + Add site
            </Link>
            <div className="rounded-xl border border-gray-200 px-4 py-3.5 text-[12.5px] text-gray-500">
              <p>One script per site:</p>
              <code className="mt-1.5 block font-mono text-[11.5px] text-gray-900 break-all">
                {`<script defer src="${window.location.origin}/y/api/v1/sdk.js"></script>`}
              </code>
            </div>
          </aside>
        </div>
      </div>

      {/* Delete Dialog */}
      <Dialog
        open={websiteToDelete !== null}
        onOpenChange={(open) => !open && setWebsiteToDelete(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete {websiteToDelete?.domain}?</DialogTitle>
            <DialogDescription>
              This will permanently delete all analytics data for this site. This action cannot be
              undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setWebsiteToDelete(null)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDeleteWebsite}>
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Tracking Script Dialog */}
      <Dialog open={showIntegrationHelp} onOpenChange={setShowIntegrationHelp}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Code className="h-5 w-5" />
              Tracking script
            </DialogTitle>
            <DialogDescription>
              Add this to {selectedWebsiteForIntegration?.domain}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="bg-gray-900 p-4 rounded-lg font-mono text-sm overflow-x-auto">
              <code className="text-green-400">
                {`<script defer src="${window.location.origin}/y/api/v1/sdk.js" data-website-id="${selectedWebsiteForIntegration?.id}"></script>`}
              </code>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                const script = `<script defer src="${window.location.origin}/y/api/v1/sdk.js" data-website-id="${selectedWebsiteForIntegration?.id}"></script>`;
                navigator.clipboard.writeText(script);
                setCopiedScript(true);
                setTimeout(() => setCopiedScript(false), 2000);
              }}
            >
              {copiedScript ? (
                <>
                  <Check className="h-4 w-4 mr-2" />
                  Copied!
                </>
              ) : (
                <>
                  <Copy className="h-4 w-4 mr-2" />
                  Copy to clipboard
                </>
              )}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </AdminLayout>
  );
};
