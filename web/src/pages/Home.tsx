import { useState } from "react";
import { usePage, Link, router } from "@inertiajs/react";
import { formatDistanceToNow } from "date-fns";
import {
  TrendingUp,
  TrendingDown,
  Globe,
  Target,
  Trophy,
  AlertTriangle,
  Plus,
  MoreHorizontal,
  Code,
  Trash2,
  Copy,
  Check,
  FileText,
  CalendarDays,
  Calendar,
  Users,
  HelpCircle,
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
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { AdminLayout } from "@/components/admin-layout";
import { cn } from "@/lib/utils";

interface FeedItem {
  id: number;
  websiteId: number;
  itemType: string;
  title: string;
  description: string;
  detectedAt: string;
  websiteDomain: string;
}

interface Website {
  id: number;
  domain: string;
  event_count?: number;
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

const itemTypeIcons: Record<string, React.ElementType> = {
  traffic_spike: TrendingUp,
  traffic_drop: TrendingDown,
  new_referrer: Globe,
  goal_hit: Target,
  milestone: Trophy,
  page_problem: AlertTriangle,
  trending_content: FileText,
  daily_summary: CalendarDays,
  monthly_summary: Calendar,
  dropping_pages: TrendingDown,
  best_sources: Users,
};

const itemTypeColors: Record<string, string> = {
  traffic_spike: "text-green-700 bg-green-100",
  traffic_drop: "text-red-700 bg-red-100",
  new_referrer: "text-blue-700 bg-blue-100",
  goal_hit: "text-purple-700 bg-purple-100",
  milestone: "text-amber-700 bg-amber-100",
  page_problem: "text-orange-700 bg-orange-100",
  trending_content: "text-indigo-700 bg-indigo-100",
  daily_summary: "text-slate-700 bg-slate-100",
  monthly_summary: "text-slate-700 bg-slate-100",
  dropping_pages: "text-orange-700 bg-orange-100",
  best_sources: "text-emerald-700 bg-emerald-100",
};

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

function FeedItemRow({ item }: { item: FeedItem }) {
  const Icon = itemTypeIcons[item.itemType] || AlertTriangle;
  const colorClass = itemTypeColors[item.itemType] || "text-gray-600 bg-gray-100";

  return (
    <Link
      href={`/admin/websites/${item.websiteId}/dashboard`}
      className="flex items-center gap-3 py-3 -mx-2 px-2 rounded-lg hover:bg-gray-50 transition-colors"
    >
      <div className={cn("p-2 rounded flex items-center justify-center", colorClass)}>
        <Icon className="w-4 h-4" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-gray-900">{item.websiteDomain}</span>
          <span className="text-xs text-gray-500">
            {formatDistanceToNow(new Date(item.detectedAt), { addSuffix: true })}
          </span>
        </div>
        <p className="text-sm text-gray-900 mt-0.5">{item.description}</p>
      </div>
    </Link>
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

function SiteCard({ site, onShowScript, onDelete }: SiteCardProps) {
  const eventCount = site.event_count || 0;
  const isActive = eventCount > 0;

  return (
    <div className="group relative bg-white border border-black rounded-lg hover:shadow-md transition-all">
      <Link href={`/admin/websites/${site.id}/dashboard`} className="block p-4">
        <div className="flex items-start gap-3">
          <img
            src={`https://www.google.com/s2/favicons?domain=${site.domain}&sz=32`}
            alt=""
            className="w-8 h-8 rounded-lg bg-gray-100 p-1"
            onError={(e) => {
              const target = e.target as HTMLImageElement;
              target.style.display = "none";
            }}
          />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-semibold text-gray-900 truncate">{site.domain}</p>
            <div className="flex items-center gap-1.5 mt-0.5">
              <span className={`w-1.5 h-1.5 rounded-full ${isActive ? "bg-green-500" : "bg-gray-300"}`} />
              <span className="text-xs text-gray-600">{formatCount(eventCount)} events</span>
            </div>
          </div>
        </div>
      </Link>

      <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity">
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0 bg-white/80 hover:bg-white shadow-sm"
              onClick={(e) => e.preventDefault()}
            >
              <MoreHorizontal className="h-4 w-4 text-gray-600" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem onClick={() => onShowScript(site)}>
              <Code className="h-4 w-4 mr-2" />
              Get tracking script
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => onDelete(site)}
              className="text-red-600 focus:text-red-600"
            >
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

  const cellSize = 10;
  const cellGap = 2;
  const weekWidth = cellSize + cellGap;

  return (
    <div className="bg-white border border-black rounded-lg p-4 w-fit">
      <div className="flex items-center gap-8 mb-2">
        <h3 className="text-sm font-medium text-gray-900">
          {total.toLocaleString()} visitors in the last year
        </h3>
        <div className="flex items-center gap-1 text-xs text-gray-500">
          <span>Less</span>
          {legendColors.map((c, i) => (
            <div key={i} className="w-[10px] h-[10px] rounded-sm" style={{ backgroundColor: c }} />
          ))}
          <span>More</span>
        </div>
      </div>

      {/* Month labels row */}
      <div className="relative h-4 mb-1">
        {monthLabels.map((m, i) => (
          <span
            key={i}
            className="absolute text-xs text-gray-500"
            style={{ left: m.weekIndex * weekWidth }}
          >
            {m.label}
          </span>
        ))}
      </div>

      {/* Calendar grid */}
      <div className="flex gap-[2px]">
        {weeks.map((week, weekIndex) => (
          <div key={weekIndex} className="flex flex-col gap-[2px]">
            {week.map((day, dayIndex) => (
              <div
                key={dayIndex}
                className={cn(
                  "w-[10px] h-[10px] rounded-sm",
                  day.count >= 0 && "cursor-pointer"
                )}
                style={{ backgroundColor: cellColor(day.count) }}
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
    </div>
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
  const FEED_PAGE_SIZE = 25;
  const [visibleCount, setVisibleCount] = useState(FEED_PAGE_SIZE);

  const filteredFeedItems = filterSiteId
    ? feedItems.filter((item) => item.websiteId === filterSiteId)
    : feedItems;
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
      <div className="py-6">
        {/* Your Sites Section */}
        <section className="mb-10">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-bold text-gray-900 flex items-center gap-2">
              <Globe className="w-5 h-5" />
              Your sites
            </h2>
            <Link
              href="/admin/websites/new"
              className="text-sm px-3 py-1.5 bg-black text-white rounded hover:bg-gray-800 flex items-center gap-1 font-medium transition-colors"
            >
              <Plus className="w-4 h-4" />
              Add site
            </Link>
          </div>

          {websites.length === 0 ? (
            <div className="text-center py-12 bg-white rounded-lg border border-black">
              <Globe className="w-10 h-10 text-gray-400 mx-auto mb-3" />
              <p className="text-sm text-gray-600 mb-4">No websites yet</p>
              <Link
                href="/admin/websites/new"
                className="inline-flex items-center px-4 py-2 bg-black text-white text-sm font-medium rounded-lg hover:bg-gray-800 transition-colors"
              >
                Add your first site
              </Link>
            </div>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
              {websites.map((site) => (
                <SiteCard
                  key={site.id}
                  site={site}
                  onShowScript={(s) => {
                    setSelectedWebsiteForIntegration(s);
                    setShowIntegrationHelp(true);
                  }}
                  onDelete={setWebsiteToDelete}
                />
              ))}
            </div>
          )}
        </section>

        {/* Visitor Calendar (always shown — every site has visitors) */}
        <section className="mb-10">
          <VisitorCalendar data={calendarData} total={totalVisitors} />
        </section>

        {/* What's New Section */}
        <section>
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <h2 className="text-xl font-bold text-gray-900 flex items-center gap-2">
                <TrendingUp className="w-5 h-5" />
                What's new
              </h2>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button className="text-gray-400 hover:text-gray-900">
                      <HelpCircle className="w-4 h-4" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="right" className="max-w-xs">
                    <p>
                      Surfaces what matters: traffic spikes, new referrers, milestones. Small
                      sites stay quiet until something real happens. No noise.
                    </p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            {websites.length > 1 && feedItems.length > 0 && (
              <DropdownMenu modal={false}>
                <DropdownMenuTrigger asChild>
                  <button className="text-sm text-gray-600 hover:text-gray-900 flex items-center gap-1">
                    {filterSiteId
                      ? websites.find((w) => w.id === filterSiteId)?.domain
                      : "All sites"}
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M19 9l-7 7-7-7"
                      />
                    </svg>
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    onClick={() => {
                      setFilterSiteId(null);
                      setVisibleCount(FEED_PAGE_SIZE);
                    }}
                    className={!filterSiteId ? "font-medium" : ""}
                  >
                    All sites
                  </DropdownMenuItem>
                  {websites.map((site) => (
                    <DropdownMenuItem
                      key={site.id}
                      onClick={() => {
                        setFilterSiteId(site.id);
                        setVisibleCount(FEED_PAGE_SIZE);
                      }}
                      className={filterSiteId === site.id ? "font-medium" : ""}
                    >
                      {site.domain}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </div>

          {feedItems.length === 0 ? (
            <p className="text-sm text-gray-500 py-4">Nothing yet.</p>
          ) : (
            <div className="space-y-6">
              {groupOrder.map((label) => {
                const items = groupedItems[label];
                if (!items || items.length === 0) return null;

                return (
                  <div key={label}>
                    <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                      {label}
                    </h3>
                    <div>
                      {items.map((item) => (
                        <FeedItemRow key={item.id} item={item} />
                      ))}
                    </div>
                  </div>
                );
              })}
              {hasMoreItems && (
                <button
                  onClick={() => setVisibleCount((prev) => prev + FEED_PAGE_SIZE)}
                  className="text-sm text-gray-500 hover:text-gray-900 transition-colors"
                >
                  Load more ({remainingCount})
                </button>
              )}
            </div>
          )}
        </section>
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
