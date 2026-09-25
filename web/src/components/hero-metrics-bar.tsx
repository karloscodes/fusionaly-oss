import { formatNumber } from "@/lib/utils";

interface MetricData {
	label: string;
	value: string | number;
	trend?: number; // Percentage change from previous period
	series?: number[]; // Values over the selected range, drawn as a sparkline
	lowerIsBetter?: boolean; // e.g. bounce rate: a drop is good news
	trendInPoints?: boolean; // trend is percentage points (rates), not percent
}

interface HeroMetricsBarProps {
	metrics: MetricData[];
	trendLoading?: boolean; // Show skeleton for trend indicators
	highlight?: number; // Index of the metric the chart shows; its sparkline takes the accent
}

// The sparkline shows the last SPARK_POINTS values of the range.
const SPARK_POINTS = 14;
const SPARK_HEIGHT = 28;

const TrendSkeleton = () => (
	<span className="flex items-center gap-1 animate-pulse">
		<div className="w-3 h-3 bg-gray-200 rounded" />
		<div className="w-8 h-3 bg-gray-200 rounded" />
	</span>
);

// formatTrend writes a change the way a person reads it: "+18.2%" for a
// relative change, "+6 pts" for a rate, and "72×" once a relative change
// passes +1000% (a period that grew from almost nothing).
export const formatTrend = (trend: number, inPoints?: boolean): string => {
	const sign = trend > 0 ? "+" : "−";
	if (inPoints) return `${sign}${Math.abs(trend).toFixed(1)} pts`;
	if (trend >= 1000) return `${Math.round(1 + trend / 100)}×`;
	return `${sign}${Math.abs(trend).toFixed(1)}%`;
};

// The change in mono, green for good news and red for bad news.
const TrendIndicator = ({ trend, loading, lowerIsBetter, inPoints }: { trend?: number; loading?: boolean; lowerIsBetter?: boolean; inPoints?: boolean }) => {
	if (loading) {
		return <TrendSkeleton />;
	}

	if (trend === undefined || trend === null) {
		return null;
	}

	if (trend === 0) {
		return <span className="font-mono text-xs text-gray-500">{inPoints ? "0 pts" : "0%"}</span>;
	}

	const good = lowerIsBetter ? trend < 0 : trend > 0;
	return (
		<span className={`font-mono text-xs ${good ? "text-emerald-600" : "text-rose-500"}`}>
			{formatTrend(trend, inPoints)}
		</span>
	);
};

const Sparkline = ({ series, highlighted }: { series: number[]; highlighted: boolean }) => {
	const points = series.slice(-SPARK_POINTS);
	const max = Math.max(...points, 1);
	return (
		<div className="hidden sm:flex items-end gap-0.5 shrink-0" style={{ height: SPARK_HEIGHT }} aria-hidden="true">
			{points.map((v, i) => (
				<span
					key={i}
					className="w-1 rounded-sm"
					style={{
						height: Math.max(2, Math.round((v / max) * SPARK_HEIGHT)),
						background: highlighted ? "rgb(var(--c-accent))" : "rgb(var(--c-gray-400) / 0.55)",
					}}
				/>
			))}
		</div>
	);
};

export const HeroMetricsBar = ({ metrics, trendLoading, highlight }: HeroMetricsBarProps) => {
	return (
		<div className="bg-white rounded-xl border border-black">
			<div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6">
				{metrics.map((metric, index) => (
					<div
						key={index}
						className="px-3 sm:px-4 py-3 sm:py-4 flex items-end justify-between gap-2 min-w-0 border-gray-200 border-r last:border-r-0 [&:nth-child(2n)]:border-r-0 md:[&:nth-child(2n)]:border-r md:[&:nth-child(3n)]:border-r-0 lg:[&:nth-child(3n)]:border-r lg:last:border-r-0 [&:nth-child(n+3)]:border-t md:[&:nth-child(n+3)]:border-t-0 md:[&:nth-child(n+4)]:border-t lg:[&:nth-child(n+4)]:border-t-0"
					>
						<div className="flex flex-col gap-1 min-w-0">
							<span className="text-[11px] font-semibold uppercase tracking-[0.08em] whitespace-nowrap text-gray-500">
								{metric.label}
							</span>
							<span className="text-xl sm:text-[26px] leading-tight font-bold tracking-tight text-black whitespace-nowrap">
								{typeof metric.value === 'number' ? formatNumber(metric.value) : metric.value}
							</span>
							<TrendIndicator trend={metric.trend} loading={trendLoading} lowerIsBetter={metric.lowerIsBetter} inPoints={metric.trendInPoints} />
						</div>
						{metric.series && metric.series.length > 1 && (
							<Sparkline series={metric.series} highlighted={index === highlight} />
						)}
					</div>
				))}
			</div>
		</div>
	);
};

// Export a builder function for easy metric creation
export const createMetric = (
	label: string,
	value: string | number,
	trend?: number,
	series?: number[],
	lowerIsBetter?: boolean,
	trendInPoints?: boolean
): MetricData => ({
	label,
	value,
	trend,
	series,
	lowerIsBetter,
	trendInPoints,
});
