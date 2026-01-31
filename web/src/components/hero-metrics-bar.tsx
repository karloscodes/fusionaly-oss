import { formatNumber } from "@/lib/utils";
import { Users, DollarSign, Percent, TrendingUp, TrendingDown, Minus } from "lucide-react";

interface MetricData {
	label: string;
	value: string | number;
	trend?: number; // Percentage change from previous period
	icon: React.ReactNode;
}

interface HeroMetricsBarProps {
	metrics: MetricData[];
	trendLoading?: boolean; // Show skeleton for trend indicators
}

const TrendSkeleton = () => (
	<span className="flex items-center gap-1 animate-pulse">
		<div className="w-3 h-3 bg-black/10 rounded" />
		<div className="w-8 h-3 bg-black/10 rounded" />
	</span>
);

const TrendIndicator = ({ trend, loading }: { trend?: number; loading?: boolean }) => {
	if (loading) {
		return <TrendSkeleton />;
	}

	if (trend === undefined || trend === null) {
		return null;
	}

	const isPositive = trend > 0;
	const isNeutral = trend === 0;
	const absChange = Math.abs(trend);

	if (isNeutral) {
		return (
			<span className="flex items-center gap-1 text-xs text-black/50">
				<Minus className="w-3 h-3" />
				<span>0%</span>
			</span>
		);
	}

	return (
		<span className={`flex items-center gap-1 text-xs ${isPositive ? 'text-emerald-600' : 'text-rose-500'}`}>
			{isPositive ? (
				<TrendingUp className="w-3 h-3" />
			) : (
				<TrendingDown className="w-3 h-3" />
			)}
			<span>{absChange.toFixed(1)}%</span>
		</span>
	);
};

export const HeroMetricsBar = ({ metrics, trendLoading }: HeroMetricsBarProps) => {
	return (
		<div className="bg-white rounded-lg border border-black shadow-sm">
			<div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 divide-x divide-black/10">
				{metrics.map((metric, index) => (
					<div
						key={index}
						className="px-4 py-4 flex flex-col gap-2"
					>
						<div className="flex items-center justify-between">
							<span className="text-xs font-medium text-black/60 uppercase tracking-wide">
								{metric.label}
							</span>
							<div className="text-black/60">{metric.icon}</div>
						</div>
						<div className="flex items-end justify-between gap-2">
							<span className="text-2xl font-bold text-black">
								{typeof metric.value === 'number' ? formatNumber(metric.value) : metric.value}
							</span>
							<TrendIndicator trend={metric.trend} loading={trendLoading} />
						</div>
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
	icon: React.ReactNode,
	trend?: number
): MetricData => ({
	label,
	value,
	trend,
	icon,
});

// Export common icons for convenience
export const MetricIcons = {
	Users,
	DollarSign,
	Percent,
};
