export type RouteData = Record<string, unknown> & {
  websites?: Website[];
  current_website_id?: number;
  data?: unknown;
};

export interface FlashMessage {
  type: "success" | "error" | "warning" | "info";
  message: string;
}
declare global {
  interface Window {
    __PAGE_DATA__?: RouteData & Partial<AnalyticsData>;
    __ERROR__?: string | null;
    __AUTHENTICATED__: boolean;
    __CSRF_TOKEN__: string;
    __FLASH__?: FlashMessage | null;
  }
}
export interface DataItem {
  name: string;
  count: number;
}

export interface PageViewData {
  date: string;
  count: number;
}

export interface MetricCountResult {
  name: string;
  count: number;
}

export interface RevenueMetrics {
  total_revenue: number;
  total_sales: number;
  average_order_value: number;
  conversion_rate: number;
  currency: string;
}

export interface ComparisonMetrics {
  visitors_change?: number;
  views_change?: number;
  sessions_change?: number;
  bounce_rate_change?: number;
  avg_time_change?: number;
  revenue_change?: number;
}

export interface AnalyticsData {
  page_views: PageViewData[];
  visitors: PageViewData[];
  sessions: PageViewData[];
  revenue: PageViewData[];
  top_urls: MetricCountResult[];
  top_countries: MetricCountResult[];
  top_devices: MetricCountResult[];
  top_referrers: MetricCountResult[];
  top_browsers: MetricCountResult[];
  top_operating_systems: MetricCountResult[];
  top_custom_events: MetricCountResult[];
  event_revenue_totals?: Record<string, number>;
  event_conversion_rates?: Record<string, number>;
  bounce_rate: number;
  visits_duration: number;
  revenue_per_visitor: number;
  top_entry_pages: MetricCountResult[];
  top_exit_pages: MetricCountResult[];
  top_utm_sources: MetricCountResult[];
  top_utm_mediums: MetricCountResult[];
  top_utm_campaigns: MetricCountResult[];
  top_utm_terms: MetricCountResult[];
  top_utm_contents: MetricCountResult[];
  top_ref_params: MetricCountResult[];
  bucket_size: "hour" | "day" | "week" | "month" | "year";
  total_visitors?: number;
  total_views?: number;
  total_sessions?: number;
  total_entry_count?: number;
  total_exit_count?: number;
  total_custom_events?: number;
  revenue_metrics?: RevenueMetrics;
  top_revenue_events?: MetricCountResult[];
  conversion_goals: string[];
  insights: Insight[];
  comparison?: ComparisonMetrics;
}

export interface TimeRange {
  label: string;
  value: string;
  shortcut: string;
}

export interface TimeRangeGroup {
  label: string;
  ranges: TimeRange[];
}

export const timeRanges: TimeRangeGroup[] = [
  {
    label: 'Quick select',
    ranges: [
      { label: 'Today', value: 'today', shortcut: '1' },
      { label: 'Yesterday', value: 'yesterday', shortcut: '2' },
      { label: 'Last 7 Days', value: 'last_7_days', shortcut: '3' },
      { label: 'Last 30 Days', value: 'last_30_days', shortcut: '4' },
      { label: 'Month to Date', value: 'month_to_date', shortcut: '5' },
      { label: 'Last Month', value: 'last_month', shortcut: '6' },
      { label: 'Year to Date', value: 'year_to_date', shortcut: '7' },
      { label: 'Last 12 Months', value: 'last_12_months', shortcut: '8' },
    ]
  },
  {
    label: 'Advanced',
    ranges: [
      { label: 'All time (careful! 🐌)', value: 'all_time', shortcut: '9' },
      { label: 'Custom Range', value: 'custom', shortcut: '0' },
    ]
  }
];

export interface Event {
  timestamp: string;
  raw_url: string;
  device_type: string;
  operating_system: string;
  browser: string;
  country: string;
  referrer: string;
  event_type: number;
  user: string;
  custom_event_key?: string;
}

export interface PaginationData {
  current_page: number;
  total_pages: number;
  total_items: number;
  per_page: number;
}

export interface EventsResponse {
  events: Event[];
  pagination: PaginationData;
}

export interface SettingsResponse {
  data: [
    {
      key: string;
      value: string;
    }
  ]
}

// Types for ReferrersCard component
export type MetricType = 'referrers' | 'utm_sources' | 'utm_mediums' | 'utm_campaigns' | 'utm_terms' | 'utm_contents' | 'ref_params';

export interface ReferrersCardProps {
  data: {
    top_referrers: DataItem[];
    top_utm_sources: DataItem[];
    top_utm_mediums: DataItem[];
    top_utm_campaigns: DataItem[];
    top_utm_terms: DataItem[];
    top_utm_contents: DataItem[];
    top_ref_params: DataItem[];
  };
}

// Website related types
export interface Website {
  id: number;
  domain: string;
  public_key?: string;
  created_at?: string;
  updated_at?: string;
}

export interface WebsitesResponse {
  data: Website[];
}

export type InsightSeverity = "high" | "medium" | "low" | "info";

export interface Insight {
  title: string;
  description: string;
  severity: InsightSeverity;
  date: string;
  ai_recommendation?: string;
  suggested_query?: string;
  percentage_change?: number;
  additional_context?: unknown;
}

export type AnnotationType = "general";

export interface Annotation {
  id: number;
  website_id: number;
  title: string;
  description?: string;
  annotation_type: AnnotationType;
  annotation_date: string;
  color: string;
  created_at: string;
  updated_at: string;
}
