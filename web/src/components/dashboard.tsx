import { useState, useEffect } from "react";
import {
	Bar,
	XAxis,
	YAxis,
	Tooltip as RechartsTooltip,
	ResponsiveContainer,
	CartesianGrid,
	ComposedChart,
	Line,
	ReferenceLine,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { FlashMessageDisplay } from "@/components/ui/flash-message";
import {
	Users,
	Zap,
	LayoutDashboard,
	Percent,
	Mouse,
	DollarSign,
	FileText,
	Clock,
	Globe,
	Smartphone,
	Check,
	GitBranch,
} from "lucide-react";
import { HeroMetricsBar, createMetric } from "@/components/hero-metrics-bar";
import DataTable from "./data-table";
import type {
	AnalyticsData,
	PageViewData,
	FlashMessage,
	Annotation,
	UserFlowLink,
} from "../types";
import { timeRanges } from "../types";
import { TimeRangeSelector } from "@/components/time-range-selector";
import { ReferrersCard } from "@/components/referrers-card";
import { AnnotationManager, AnnotationDetailDialog } from "@/components/annotation-manager";
import { VisitorFlowSankey } from "@/components/user-flow-sankey";
import {
	TooltipProvider,
	TooltipTrigger,
	TooltipContent,
	Tooltip as ShadcnTooltip,
} from "@/components/ui/tooltip";
import { formatNumber } from "@/lib/utils";
import { convertRangeToDateRange } from "@/utils/date-range-converter";
import { Checkbox } from "./ui/checkbox";
import { usePage, Deferred } from "@inertiajs/react";

// --- Helper Functions ---

// Format session duration in a human-readable format
const formatSessionDuration = (seconds: number): string => {
	if (seconds < 60) {
		return `${Math.round(seconds)}s`;
	}
	const minutes = Math.floor(seconds / 60);
	const remainingSeconds = Math.round(seconds % 60);
	if (minutes < 60) {
		return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
	}
	const hours = Math.floor(minutes / 60);
	const remainingMinutes = minutes % 60;
	return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
};

// --- Component ---

// We no longer need the inline onboarding component since we redirect to /admin/websites/new
// This helps maintain consistency and avoids duplicated code

interface DashboardComponentProps extends Partial<AnalyticsData> {
	current_website_id?: number;
	flash?: FlashMessage | null;
	error?: string | null;
	annotations?: Annotation[];
	is_public_view?: boolean;
	user_flow?: UserFlowLink[];
}

export const Dashboard = (props: DashboardComponentProps) => {
	// Use Inertia's usePage hook for reactive URL
	const { url } = usePage();
	const defaultTimeRange = "last_30_days"; // Changed from last_7_days to reduce granularity

	// Parse query params from Inertia URL (reactive)
	const searchParams = new URLSearchParams(url.split('?')[1] || '');

	// Use the current URL path (without query string) for navigation
	const baseDashboardPath = url.split('?')[0];

	// Get website ID from URL or props (reactive to URL changes)
	const websiteIdParam = searchParams.get("website_id");
	const selectedWebsiteId = websiteIdParam
		? Number.parseInt(websiteIdParam, 10)
		: props.current_website_id || null;

	const [isLoading, setIsLoading] = useState(true);

	// Get time range from URL (reactive to URL changes)
	const rangeParam = searchParams.get("range");
	const timeRange = rangeParam ||
		(searchParams.get("from") && searchParams.get("to") ? "custom" : defaultTimeRange);

	// State for active chart and data loading
	const [deviceTab, setDeviceTab] = useState("devices");
	const [pagesTab, setPagesTab] = useState("pages");
	const [data, setData] = useState<AnalyticsData | null>(null);
	const [activeChart, setActiveChart] = useState<
		"views" | "visitors"
	>("views"); // Toggle between charts
	const [showRevenueLine, setShowRevenueLine] = useState(true); // Control revenue line visibility
	const [tooltipOpen, setTooltipOpen] = useState(false);

	// State for viewing/editing existing annotations when clicked on chart
	const [selectedAnnotation, setSelectedAnnotation] = useState<Annotation | null>(null);

	// State for creating new annotation when clicking on chart
	const [createAnnotationOpen, setCreateAnnotationOpen] = useState(false);
	const [createAnnotationDate, setCreateAnnotationDate] = useState<string | undefined>(undefined);

	// Add effect to sync URL params with state

	// Keyboard shortcuts for time ranges - only active on dashboard
	useEffect(() => {
		const handleKeyPress = (e: KeyboardEvent) => {
			// Only handle shortcuts if we're on a dashboard page (admin or public)
			const isDashboardPage = window.location.pathname.includes('/dashboard') &&
			                        (window.location.pathname.includes('/admin/') ||
			                         window.location.pathname.includes('/public/'));
			if (!isDashboardPage) {
				return;
			}

		if (e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) {
			return;
		}

		const target = e.target as HTMLElement | null;
		if (target) {
			const tag = target.tagName;
			const isFormField =
				tag === "INPUT" ||
				tag === "TEXTAREA" ||
				tag === "SELECT" ||
				target.isContentEditable;
			if (isFormField) {
				return;
			}
		}

		const key = e.key;
		const range = timeRanges
			.flatMap((group) => group.ranges)
			.find((r) => r.shortcut === key);

		if (range) {
			e.preventDefault();
			e.stopPropagation();

			if (range.value === "custom") {
				window.dispatchEvent(new CustomEvent("openCustomRange"));
			} else {
				const dateRange = convertRangeToDateRange(range.value);
				window.location.href = `${baseDashboardPath}?from=${dateRange.from}&to=${dateRange.to}&range=${range.value}`;
			}
		}
		};

		document.addEventListener("keydown", handleKeyPress, true); // Use capture phase
		return () => document.removeEventListener("keydown", handleKeyPress, true);
	}, []);

	// Load data from Inertia props
	useEffect(() => {
		setIsLoading(true);

		// Cast props to AnalyticsData since backend provides complete data
		setData({ ...props } as AnalyticsData);

		setIsLoading(false);
	}, [props, selectedWebsiteId]);


	if (isLoading || !data || !data.page_views) {
		// Loading state
		return (
			<div className="min-h-screen bg-white flex items-center justify-center">
				<div className="text-lg">Loading analytics data...</div>
			</div>
		);
	}

	// Calculate totals
	const totalViews =
		data.total_views !== undefined
			? data.total_views
			: (data.page_views && Array.isArray(data.page_views))
				? data.page_views.reduce(
					(acc: number, curr: PageViewData) => acc + curr.count,
					0,
				)
				: 0;
	const totalVisitors =
		data.total_visitors !== undefined
			? data.total_visitors
			: (data.visitors && Array.isArray(data.visitors))
				? data.visitors.reduce(
					(acc: number, curr: PageViewData) => acc + curr.count,
					0,
				)
				: 0;
	const totalSessions =
		data.total_sessions !== undefined
			? data.total_sessions
			: (data.sessions && Array.isArray(data.sessions))
				? data.sessions.reduce(
					(acc: number, curr: PageViewData) => acc + curr.count,
					0,
				)
				: 0;
	const bucketSize = data.bucket_size;
	const eventRevenueTotals = data.event_revenue_totals || {};
	const eventConversionRates = data.event_conversion_rates || {};

	const formatDate = (item: PageViewData) => {
		// Parse the RFC3339 date string (now in UTC format from backend)
		const date = new Date(item.date);

		// Quick sanity check - if date is invalid, return the raw date
		if (Number.isNaN(date.getTime())) {
			console.error(`Invalid date format: ${item.date}`);
			return {
				date: item.date,
				formattedDate: item.date,
			};
		}

		// Check if the dataset spans multiple years
		const hasMultipleYears = (() => {
			if (!data.page_views || data.page_views.length === 0) return false;

			const years = new Set();
			for (const item of data.page_views) {
				const itemDate = new Date(item.date);
				if (!Number.isNaN(itemDate.getTime())) {
					years.add(itemDate.getFullYear());
				}
			}
			return years.size > 1;
		})();

		let formattedDate: string;

		switch (bucketSize) {
			case "hour": {
				// For hourly data, just show time if it's today, otherwise add date
				const today = new Date();
				const isToday =
					date.getDate() === today.getDate() &&
					date.getMonth() === today.getMonth() &&
					date.getFullYear() === today.getFullYear();

				if (isToday) {
					formattedDate = date.toLocaleTimeString(undefined, {
						hour: "numeric",
						minute: "2-digit",
					});
				} else {
					const dateOptions: Intl.DateTimeFormatOptions = {
						month: "short",
						day: "numeric",
					};

					// Add year if data spans multiple years
					if (hasMultipleYears) {
						dateOptions.year = "numeric";
					}

					formattedDate = `${date.toLocaleDateString(undefined, dateOptions)}, ${date.toLocaleTimeString(undefined, {
						hour: "numeric",
					})}`;
				}
				break;
			}
			case "day": {
				const dayOptions: Intl.DateTimeFormatOptions = {
					month: "short",
					day: "numeric",
				};

				// Add year if data spans multiple years
				if (hasMultipleYears) {
					dayOptions.year = "numeric";
				}

				formattedDate = date.toLocaleDateString(undefined, dayOptions);
				break;
			}
			case "week": {
				const weekOptions: Intl.DateTimeFormatOptions = {
					month: "short",
					day: "numeric",
				};

				// Add year if data spans multiple years
				if (hasMultipleYears) {
					weekOptions.year = "numeric";
				}

				formattedDate = date.toLocaleDateString(undefined, weekOptions);
				break;
			}
			case "month": {
				const monthOptions: Intl.DateTimeFormatOptions = {
					year: "numeric",
					month: "short",
				};

				formattedDate = date.toLocaleDateString(undefined, monthOptions);
				break;
			}
			case "year": {
				formattedDate = date.toLocaleDateString(undefined, {
					year: "numeric",
				});
				break;
			}
			default: {
				const defaultOptions: Intl.DateTimeFormatOptions = {
					month: "short",
					day: "numeric",
				};

				// Add year if data spans multiple years
				if (hasMultipleYears) {
					defaultOptions.year = "numeric";
				}

				formattedDate = date.toLocaleDateString(undefined, defaultOptions);
			}
		}

		return {
			date: item.date,
			formattedDate,
		};
	};

	// Format chart data
	const chartData = data.page_views.map(
		(item: PageViewData, index: number) => {
			const pageViews = data.page_views[index].count;
			const revenue = data.revenue?.[index]?.count ?? 0; // Revenue in cents
			const visitorsCount = data.visitors?.[index]?.count ?? 0;
			const sessionsCount = data.sessions?.[index]?.count ?? 0;
			return {
				date: item.date,
				formattedDate: formatDate(item).formattedDate,
				views: pageViews,
				visitors: visitorsCount,
				sessions: sessionsCount,
				revenue: revenue, // Revenue in cents
				revenueFormatted: `$${(revenue / 100).toFixed(2)}`, // Convert to dollars for display
			};
		},
	);

	// Check if there's any data in the time range
	const hasData = chartData.some(
		(item) => item.views > 0 || item.visitors > 0 || item.sessions > 0,
	);

	// Calculate maximum value for y-axis domain
	const getMaxValue = () => {
		if (!hasData) return 5; // Default for empty charts
		const dataKey = getActiveDataKey();
		const maxValue = Math.max(...chartData.map((item) => item[dataKey] || 0));

		// More intelligent scaling:
		// For small values (1-3), use 0-4 with ticks at every integer
		// For medium values, use a reasonable max to get natural tick spacing
		// For larger values, use percentage-based padding
		if (maxValue <= 3) {
			return 4; // Show 0, 1, 2, 3, 4
		}
		if (maxValue <= 10) {
			return Math.ceil(maxValue) + (Math.ceil(maxValue) % 2 === 0 ? 2 : 1); // Make sure we end with an even number of ticks
		}
		// For larger values, use percentage padding
		return Math.ceil(maxValue * 1.1);
	};

	// Get the appropriate tick count for the Y-axis
	const getTickCount = () => {
		const dataKey = getActiveDataKey();
		const maxValue = Math.max(...chartData.map((item) => item[dataKey] || 0));

		if (maxValue <= 3) return 5; // 0, 1, 2, 3, 4
		if (maxValue <= 10) return 6; // Reasonable number of ticks for small ranges
		return undefined; // Let Recharts decide for larger values
	};

	// Define colors for different chart types
	const getChartColors = () => {
		switch (activeChart) {
			case "views":
				return {
					default: "#00D1FF",
					hover: "#E5E7EB", // Light gray
				};
			case "visitors":
				return {
					default: "#00D678",
					hover: "#E5E7EB", // Light gray
				};
		}
	};

	// Get the active data key based on the selected chart
	const getActiveDataKey = () => {
		switch (activeChart) {
			case "views":
				return "views";
			case "visitors":
				return "visitors";
		}
	};

	// Get the active chart name
	const getActiveChartName = () => {
		switch (activeChart) {
			case "views":
				return "Page Views";
			case "visitors":
				return "Visitors";
		}
	};

	// Get the revenue line color (darker version of the active chart color)
	const getRevenueLineColor = () => {
		switch (activeChart) {
			case "views":
				return "#0E7490"; // Muted teal that complements the view bars
			case "visitors":
				return "#047857"; // Balanced emerald tone for visitor view
		}
	};

	// Get the revenue axis domain with a reasonable minimum
	const getRevenueAxisDomain = () => {
		const maxRevenue = Math.max(...chartData.map((item) => item.revenue || 0));

		// If all revenue is 0, don't show the axis at all by using a very small range
		if (maxRevenue === 0) {
			return [0, 1000]; // $10 range so the line doesn't dominate the chart
		}

		// Otherwise, use a reasonable range with padding
		return [0, Math.max(maxRevenue * 1.1, 1000)];
	};

	// Find matching chart data point for an annotation date
	const findAnnotationChartMatch = (annotationDate: string): string | null => {
		const annotationTime = new Date(annotationDate).getTime();

		// Find the closest chart data point
		let closestMatch: { formattedDate: string; diff: number } | null = null;

		for (const item of chartData) {
			const itemTime = new Date(item.date).getTime();
			const diff = Math.abs(itemTime - annotationTime);

			// For hourly buckets, match within 1 hour
			// For daily buckets, match within the same day
			// For weekly buckets, match within the week
			const maxDiff = bucketSize === "hour" ? 3600000 : // 1 hour
				bucketSize === "day" ? 86400000 : // 1 day
				bucketSize === "week" ? 604800000 : // 1 week
				bucketSize === "month" ? 2678400000 : // ~31 days
				31536000000; // 1 year

			if (diff <= maxDiff && (!closestMatch || diff < closestMatch.diff)) {
				closestMatch = { formattedDate: item.formattedDate, diff };
			}
		}

		return closestMatch?.formattedDate || null;
	};

	// Render bar chart
	const renderChart = () => {
		const colors = getChartColors();
		const dataKey = getActiveDataKey();
		const chartName = getActiveChartName();

		if (!hasData) {
			return (
				<div className="flex h-64 w-full items-center justify-center">
					<p className="text-gray-500">
						No data available for this time period
					</p>
				</div>
			);
		}

		// Map annotations to chart x-values
		const annotationsWithChartMatch = (props.annotations || [])
			.map(annotation => ({
				...annotation,
				chartX: findAnnotationChartMatch(annotation.annotation_date),
			}))
			.filter(a => a.chartX !== null);

		// Handle chart click for creating annotations (disabled in public view)
		const handleChartClick = (chartEvent: { activePayload?: Array<{ payload?: { date?: string } }> } | null) => {
			if (props.is_public_view) return; // Disable annotation creation in public view
			if (props.current_website_id && chartEvent?.activePayload?.[0]?.payload?.date) {
				setCreateAnnotationDate(chartEvent.activePayload[0].payload.date);
				setCreateAnnotationOpen(true);
			}
		};

		return (
			<ComposedChart
				data={chartData}
				margin={{ top: 60, right: 20, bottom: 80, left: 20 }}
				barGap={8}
				barCategoryGap={16}
				barSize={36}
				onClick={props.is_public_view ? undefined : handleChartClick}
			>
				<CartesianGrid
					horizontal={true}
					vertical={false}
					strokeDasharray="3 4"
					opacity={0.2}
				/>
				<XAxis
					dataKey="formattedDate"
					strokeWidth={1}
					tick={{ fill: "#374151", fontSize: 10, textAnchor: "end" }}
					axisLine={{ stroke: "#E5E7EB" }}
					tickLine={{ stroke: "#E5E7EB" }}
					dy={10}
					interval="preserveStartEnd"
					angle={-45}
				/>
				<YAxis
					strokeWidth={1}
					tick={{ fill: "#374151", fontSize: 10 }}
					axisLine={{ stroke: "#E5E7EB" }}
					tickLine={{ stroke: "#E5E7EB" }}
					dx={-10}
					domain={[0, getMaxValue()]}
					allowDecimals={false}
					tickCount={getTickCount()}
				/>
				<YAxis
					yAxisId="right"
					orientation="right"
					strokeWidth={1}
					tick={{ fill: "#374151", fontSize: 10 }}
					axisLine={{ stroke: "#E5E7EB" }}
					tickLine={{ stroke: "#E5E7EB" }}
					dx={10}
					domain={getRevenueAxisDomain()}
					allowDecimals={true}
					tickFormatter={(value) => `$${(value / 100).toFixed(0)}`}
				/>
				<RechartsTooltip
					content={({ active, payload, label }) => {
						if (!active || !payload || payload.length === 0) return null;

						return (
							<div className="bg-gray-50 border border-gray-300 rounded-lg shadow-lg p-4 min-w-[180px]">
								<div className="text-gray-900 text-sm font-semibold mb-3 pb-2 border-b border-gray-200">
									{label}
								</div>
								<div className="space-y-1">
									{payload.map((entry, index) => (
										<div key={index} className="flex justify-between items-center text-sm gap-4">
											<span className="text-gray-600">{entry.name}</span>
											<span className="font-medium text-gray-900">
												{entry.name === "Revenue"
													? `$${((entry.value as number) / 100).toFixed(2)}`
													: formatNumber(entry.value as number)
												}
											</span>
										</div>
									))}
								</div>
								{props.current_website_id && !props.is_public_view && (
									<div className="mt-2 pt-2 border-t border-gray-200">
										<p className="text-xs text-gray-400">Click bar to add annotation</p>
									</div>
								)}
							</div>
						);
					}}
					wrapperStyle={{ outline: "none" }}
					cursor={{ fill: "#E5E7EB", opacity: 0.4, radius: 4 }}
				/>
				<Bar
					dataKey={dataKey}
					name={chartName}
					fill={colors.default}
					radius={[4, 4, 0, 0]}
					animationDuration={300}
					animationEasing="ease-out"
					style={{ cursor: props.current_website_id ? "pointer" : "default" }}
				/>
				{showRevenueLine && data.revenue && (
					<Line
						type="linear"
						dataKey="revenue"
						name="Revenue"
						stroke={getRevenueLineColor()}
						strokeWidth={2}
						dot={{ fill: getRevenueLineColor(), r: 3 }}
						activeDot={{ r: 6, fill: getRevenueLineColor() }}
						yAxisId="right"
					/>
				)}
				{/* Render annotation markers */}
				{annotationsWithChartMatch.map((annotation) => (
					<ReferenceLine
						key={annotation.id}
						x={annotation.chartX as string}
						stroke={annotation.color || "#f97316"}
						strokeWidth={2}
						strokeDasharray="4 4"
						label={(labelProps) => {
							const { viewBox } = labelProps;
							const x = viewBox?.x ?? 0;
							const color = annotation.color || "#f97316";
							return (
								<g
									style={{ cursor: "pointer" }}
									onClick={(e) => {
										e.stopPropagation();
										setSelectedAnnotation(annotation);
									}}
								>
									{/* Circle marker */}
									<circle
										cx={x}
										cy={14}
										r={6}
										fill={color}
									/>
									{/* Title text */}
									<text
										x={x + 12}
										y={18}
										fill={color}
										fontSize={11}
										fontWeight={600}
									>
										{annotation.title.length > 16 ? `${annotation.title.slice(0, 16)}…` : annotation.title}
									</text>
								</g>
							);
						}}
					/>
				))}
			</ComposedChart>
		);
	};

	return (
		<div className="min-h-screen bg-white py-4">
			<FlashMessageDisplay flash={props.flash} error={props.error} />

			<div className="flex flex-col gap-6">
				<div className="flex flex-wrap justify-between items-center gap-4">
					<h1 className="font-bold text-gray-900 flex items-center text-2xl">
						<LayoutDashboard className="w-6 h-6 mr-2 inline" />
						Dashboard
					</h1>
					<TimeRangeSelector
						timeRanges={timeRanges}
						currentTimeRange={timeRange}
						websiteId={selectedWebsiteId}
					/>
				</div>

				{/* Hero Metrics Bar */}
				<Deferred
					data="comparison"
					fallback={
						<HeroMetricsBar
							trendLoading={true}
							metrics={[
								createMetric("Visitors", totalVisitors, <Users className="w-4 h-4" />),
								createMetric("Page Views", totalViews, <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
									<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
									<circle cx="12" cy="12" r="3" />
								</svg>),
								createMetric("Sessions", totalSessions, <Mouse className="w-4 h-4" />),
								createMetric("Bounce Rate", `${(data.bounce_rate * 100).toFixed(0)}%`, <Percent className="w-4 h-4" />),
								createMetric("Avg Time", formatSessionDuration(data.visits_duration), <Clock className="w-4 h-4" />),
								createMetric("Revenue", `$${data.revenue_metrics ? formatNumber(Math.round(data.revenue_metrics.total_revenue)) : '0'}`, <DollarSign className="w-4 h-4" />),
							]}
						/>
					}
				>
					<HeroMetricsBar
						metrics={[
							createMetric("Visitors", totalVisitors, <Users className="w-4 h-4" />, data.comparison?.visitors_change),
							createMetric("Page Views", totalViews, <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
								<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
								<circle cx="12" cy="12" r="3" />
							</svg>, data.comparison?.views_change),
							createMetric("Sessions", totalSessions, <Mouse className="w-4 h-4" />, data.comparison?.sessions_change),
							createMetric("Bounce Rate", `${(data.bounce_rate * 100).toFixed(0)}%`, <Percent className="w-4 h-4" />, data.comparison?.bounce_rate_change),
							createMetric("Avg Time", formatSessionDuration(data.visits_duration), <Clock className="w-4 h-4" />, data.comparison?.avg_time_change),
							createMetric("Revenue", `$${data.revenue_metrics ? formatNumber(Math.round(data.revenue_metrics.total_revenue)) : '0'}`, <DollarSign className="w-4 h-4" />, data.comparison?.revenue_change),
						]}
					/>
				</Deferred>

				{/* Main chart with internal toggles and restored height */}
				<Card className="rounded-lg border border-black">
					<CardContent className="p-6">
						<div className="mb-4 flex justify-between items-center">
							<div className="flex gap-2">
								<button
									type="button"
									onClick={() => setActiveChart("views")}
									className={`px-4 py-2 text-sm border rounded ${activeChart === "views" ? "bg-black text-white" : "bg-white text-black"}`}
								>
									Page Views
								</button>
								<button
									type="button"
									onClick={() => setActiveChart("visitors")}
									className={`px-4 py-2 text-sm border rounded ${activeChart === "visitors" ? "bg-black text-white" : "bg-white text-black"}`}
								>
									Visitors
								</button>
								{data.conversion_goals && data.conversion_goals.length > 0 && (
									<div className="flex items-center space-x-3 px-3 py-2 bg-gray-50 rounded-lg border border-gray-200 hover:bg-gray-100 transition-colors">
										<Checkbox
											id="show-revenue"
											checked={showRevenueLine}
											onCheckedChange={(checked) => setShowRevenueLine(checked === true)}
											className="data-[state=checked]:bg-black data-[state=checked]:border-black"
										/>
										<label
											htmlFor="show-revenue"
											className="text-sm font-medium text-gray-900 cursor-pointer select-none"
										>
											Revenue
										</label>
									</div>
								)}
							</div>
							{props.current_website_id && !props.is_public_view && (
								<AnnotationManager
									websiteId={props.current_website_id}
									initialDate={createAnnotationDate}
									open={createAnnotationOpen}
									onOpenChange={(open) => {
										setCreateAnnotationOpen(open);
										if (!open) setCreateAnnotationDate(undefined);
									}}
								/>
							)}
						</div>
						<div className="h-[450px]">
							{" "}
							{/* Restored height to 450px */}
							<ResponsiveContainer width="100%" height="100%">
								{renderChart()}
							</ResponsiveContainer>
						</div>
					</CardContent>
				</Card>

				{/* Two-column grid for Pages and Referrers */}
				<div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
					{/* Page Analytics Card - Left Column */}
					<Card className="rounded-lg border border-black">
						<CardContent className="p-6">
							<div className="flex justify-between items-center mb-4">
								<div className="flex items-center gap-2">
									<FileText className="w-4 h-4" />
									<span>Pages</span>
								</div>
								<div className="flex space-x-2">
									<button
										type="button"
										onClick={() => setPagesTab("pages")}
										className={`px-4 py-2 text-sm border rounded ${pagesTab === "pages" ? "bg-black text-white" : "bg-white text-black"}`}
									>
										Top Pages
									</button>
									<button
										type="button"
										onClick={() => setPagesTab("entry")}
										className={`px-4 py-2 text-sm border rounded ${pagesTab === "entry" ? "bg-black text-white" : "bg-white text-black"}`}
									>
										Entry Pages
									</button>
									<button
										type="button"
										onClick={() => setPagesTab("exit")}
										className={`px-4 py-2 text-sm border rounded ${pagesTab === "exit" ? "bg-black text-white" : "bg-white text-black"}`}
									>
										Exit Pages
									</button>
								</div>
							</div>
							<div className="h-[380px] flex flex-col">
								{pagesTab === "pages" && (
									<DataTable
										data={data.top_urls}
										showPercentage={true}
										totalVisitors={totalVisitors}
										pageSize={8}
										columns={[
											{ name: "name", label: "URL" },
											{ name: "count", label: "Visitors" },
										]}
									/>
								)}
								{pagesTab === "entry" && (
									<DataTable
										data={data.top_entry_pages}
										showPercentage={true}
										totalVisitors={data.total_entry_count || totalSessions}
										pageSize={8}
										columns={[
											{ name: "name", label: "URL" },
											{ name: "count", label: "Entries" },
										]}
									/>
								)}
								{pagesTab === "exit" && (
									<DataTable
										data={data.top_exit_pages}
										showPercentage={true}
										totalVisitors={data.total_exit_count || totalSessions}
										pageSize={8}
										columns={[
											{ name: "name", label: "URL" },
											{ name: "count", label: "Exits" },
										]}
									/>
								)}
							</div>
						</CardContent>
					</Card>

					{/* Referrers & UTM Card - Right Column */}
					<ReferrersCard data={data} />
				</div>

				{/* Two-column grid for Countries and Device Analytics */}
				<div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
					{/* Countries Card - Left Column */}
					<Card className="rounded-lg border border-black">
						<CardContent className="p-6">
							<div className="flex justify-between items-center mb-4">
								<div className="flex items-center gap-2">
									<Globe className="w-4 h-4" />
									<span>Countries</span>
								</div>
							</div>
							<div className="h-[380px] flex flex-col">
								<DataTable
									data={data.top_countries}
									showPercentage={true}
									totalVisitors={totalVisitors}
									pageSize={8}
									columns={[
										{ name: "name", label: "Country" },
										{ name: "count", label: "Visitors" },
									]}
								/>
							</div>
						</CardContent>
					</Card>

					{/* Device Analytics Card - Right Column */}
					<Card className="rounded-lg border border-black">
						<CardContent className="p-6">
							<div className="flex justify-between items-center mb-4">
								<div className="flex items-center gap-2">
									<Smartphone className="w-4 h-4" />
									<span>Device Analytics</span>
								</div>
								<div className="flex space-x-2">
									<button
										type="button"
										onClick={() => setDeviceTab("devices")}
										className={`px-4 py-2 text-sm border rounded ${deviceTab === "devices" ? "bg-black text-white" : "bg-white text-black"}`}
									>
										Devices
									</button>
									<button
										type="button"
										onClick={() => setDeviceTab("browsers")}
										className={`px-4 py-2 text-sm border rounded ${deviceTab === "browsers" ? "bg-black text-white" : "bg-white text-black"}`}
									>
										Browsers
									</button>
									<button
										type="button"
										onClick={() => setDeviceTab("os")}
										className={`px-4 py-2 text-sm border rounded ${deviceTab === "os" ? "bg-black text-white" : "bg-white text-black"}`}
									>
										OSs
									</button>
								</div>
							</div>
							<div className="h-[380px] flex flex-col">
								{deviceTab === "devices" && (
									<DataTable
										data={data.top_devices}
										showPercentage={true}
										totalVisitors={totalVisitors}
										pageSize={8}
										columns={[
											{ name: "name", label: "Device" },
											{ name: "count", label: "Visitors" },
										]}
									/>
								)}
								{deviceTab === "browsers" && (
									<DataTable
										data={data.top_browsers}
										showPercentage={true}
										totalVisitors={totalVisitors}
										pageSize={8}
										columns={[
											{ name: "name", label: "Browser" },
											{ name: "count", label: "Visitors" },
										]}
									/>
								)}
								{deviceTab === "os" && data && data.top_operating_systems && (
									<DataTable
										data={data.top_operating_systems}
										showPercentage={true}
										totalVisitors={totalVisitors}
										pageSize={8}
										columns={[
											{ name: "name", label: "Operating System" },
											{ name: "count", label: "Visitors" },
										]}
									/>
								)}
								{deviceTab === "os" && data && !data.top_operating_systems && (
									<div className="flex items-center justify-center h-full">
										<p className="text-gray-500">Operating systems data is currently unavailable. Please ensure the application is fully updated and try a hard refresh.</p>
									</div>
								)}
							</div>
						</CardContent>
					</Card>
				</div>

				{/* Full-width Events Card */}
				<Card className="rounded-lg border border-black">
					<CardContent className="p-6">
						<div className="flex justify-between items-center mb-4">
							<div className="flex items-center gap-2">
								<Zap className="w-4 h-4" />
								<span>Events</span>
								{selectedWebsiteId && (
									<TooltipProvider>
										<ShadcnTooltip open={tooltipOpen} onOpenChange={setTooltipOpen}>
											<TooltipTrigger asChild>
												<button
													className="text-gray-400 hover:text-gray-600 cursor-pointer"
													onClick={() => setTooltipOpen(!tooltipOpen)}
												>
													<svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
														<circle cx="12" cy="12" r="10" />
														<path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
														<line x1="12" y1="17" x2="12.01" y2="17" />
													</svg>
												</button>
											</TooltipTrigger>
											<TooltipContent className="bg-gray-800 text-white border-gray-700">
												<p className="mb-2">Configure which events are tracked as conversion goals</p>
												<a
													href={`/admin/websites/${selectedWebsiteId}/edit`}
													className="text-white underline hover:text-gray-200"
												>
													Edit Goals
												</a>
											</TooltipContent>
										</ShadcnTooltip>
									</TooltipProvider>
								)}
							</div>
						</div>
						<div className="h-[380px] flex flex-col">
				<DataTable
					data={data.top_custom_events}
					showPercentage={true}
					totalVisitors={data.total_custom_events || totalVisitors}
					pageSize={8}
				columns={[
					{
						name: "name",
						label: "Event",
						render: (item) => (
											<span className="flex items-center gap-1">
												<span className="truncate" title={item.name}>
													{item.name}
												</span>
												{item.name === "revenue:purchased" && (
													<DollarSign className="w-3 h-3 text-green-600 flex-shrink-0" />
												)}
							</span>
						)
					},
					{
						name: "goal",
						label: "",
						align: "center",
						widthClass: "w-12",
						render: (item) => {
							const isGoal = data.conversion_goals && data.conversion_goals.includes(item.name);
							return isGoal ? (
								<span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium border border-gray-800 text-gray-900 bg-white whitespace-nowrap">
									<Check className="w-3 h-3" />
									Goal
								</span>
							) : null;
						}
					},
					{ name: "count", label: "Visitors" },
					{
						name: "revenue",
						label: "Revenue",
						align: "right",
						widthClass: "w-24",
						render: (item) => {
							const amount = eventRevenueTotals[item.name] || 0;
							if (amount <= 0) {
								return "—";
							}
							return `$${formatNumber(Math.round(amount))}`;
						},
					},
					{
						name: "conversion_rate",
						label: "Conversion Rate",
						align: "right",
						widthClass: "w-28",
						render: (item) => {
							const rate = eventConversionRates[item.name];
							if (rate === undefined) {
								return "—";
							}
							return `${rate.toFixed(1)}%`;
						},
					},
								]}
							/>
						</div>
					</CardContent>
				</Card>
			</div>

			{/* Visitor Flow */}
			<div className="mt-4">
				<Deferred data="user_flow" fallback={
					<Card>
						<CardHeader className="pb-2">
							<CardTitle className="text-lg font-medium flex items-center gap-2">
								<GitBranch className="w-5 h-5" />
								Visitor Flows
							</CardTitle>
						</CardHeader>
						<CardContent className="pt-2">
							<div className="h-64 flex items-center justify-center">
								<p className="text-sm text-gray-500">Loading visitor flow data...</p>
							</div>
						</CardContent>
					</Card>
				}>
					<VisitorFlowSankey links={props.user_flow || []} />
				</Deferred>
			</div>

				{/* Annotation detail dialog - shown when clicking an annotation on the chart */}
			{selectedAnnotation && props.current_website_id && (
				<AnnotationDetailDialog
					websiteId={props.current_website_id}
					annotation={selectedAnnotation}
					open={!!selectedAnnotation}
					onOpenChange={(open) => {
						if (!open) setSelectedAnnotation(null);
					}}
					readOnly={props.is_public_view}
				/>
			)}
		</div>
	);
};
