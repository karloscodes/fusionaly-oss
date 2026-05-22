import React, { useMemo, useState } from "react";
import { VegaEmbed } from "react-vega";
import { useChartColors } from "@/lib/use-chart-colors";
import { cssVarColor } from "@/lib/theme";

interface VegaChartProps {
	spec: any;
	data?: any[];
	className?: string;
}

export const VegaChart: React.FC<VegaChartProps> = ({ spec, data, className }) => {
	const [error, setError] = useState<string | null>(null);

	// Subscribe to theme changes so the Vega config (built below) re-reads the
	// themed axis/grid/label colors and the chart re-renders on theme switch.
	const chartColors = useChartColors();

	const finalSpec = useMemo(() => {
		try {
			if (!spec || typeof spec !== 'object') {
				console.error('Invalid Vega spec:', spec);
				setError('Invalid visualization specification');
				return null;
			}

			const specCopy = JSON.parse(JSON.stringify(spec));

			if (data && Array.isArray(data) && data.length > 0) {
				if (specCopy.data && typeof specCopy.data === 'object' && 'name' in specCopy.data) {
					specCopy.data = { values: data };
				} else if (!specCopy.data) {
					specCopy.data = { values: data };
				}

				const firstDataItem = data[0];
				const dataFields = Object.keys(firstDataItem);

				if (specCopy.encoding) {
					Object.keys(specCopy.encoding).forEach(channel => {
						const encoding = specCopy.encoding[channel];
						if (encoding && encoding.field && !dataFields.includes(encoding.field)) {
							console.warn(`Field '${encoding.field}' not found in data. Available fields:`, dataFields);
						}
					});
				}
			} else if (!specCopy.data || !specCopy.data.values) {
				console.warn('No data available for visualization');
				setError('No data available');
				return null;
			}

			setError(null);
			return specCopy;
		} catch (err) {
			console.error('Error preparing Vega spec:', err);
			setError('Failed to prepare visualization');
			return null;
		}
	}, [spec, data]);

	// Vega-Lite theme matching Dashboard Recharts colors
	const config = {
		background: cssVarColor("--c-white"),
		// Primary marks use cyan (same as Dashboard page views)
		arc: { fill: "#00D1FF" },
		area: { fill: "#00D1FF" },
		line: { stroke: "#00D1FF", strokeWidth: 2 },
		path: { stroke: "#00D1FF" },
		rect: { fill: "#00D1FF" },
		shape: { stroke: "#00D1FF" },
		symbol: { fill: "#00D1FF", size: 30 },
		bar: { fill: "#00D1FF" },
		axis: {
			domainColor: chartColors.grid,
			gridColor: cssVarColor("--c-gray-100"),
			tickColor: chartColors.grid,
			labelColor: cssVarColor("--c-gray-500"),
			titleColor: chartColors.axisText,
		},
		axisX: {
			grid: false,
			labelAngle: -45,
			labelAlign: "right",
		},
		axisY: {
			grid: true,
			gridDash: [3, 3],
		},
		legend: {
			labelColor: cssVarColor("--c-gray-500"),
			titleColor: chartColors.axisText,
		},
		// Category palette: Cyan, Green, Orange (matches Dashboard)
		range: {
			category: [
				"#00D1FF", // Cyan - Page Views
				"#00D678", // Green - Visitors
				"#FF7733", // Orange - accent
				"#0E7490", // Teal - Revenue
				"#9333EA", // Purple
				"#374151", // Gray
			],
		},
	};

	const specWithConfig = useMemo(() => {
		if (!finalSpec) return null;

		const specCopy = { ...finalSpec };
		if (specCopy.width === "container") {
			delete specCopy.width;
		}

		// Strip any color properties from mark definitions so our config takes effect
		if (specCopy.mark && typeof specCopy.mark === 'object') {
			delete specCopy.mark.color;
			delete specCopy.mark.fill;
			delete specCopy.mark.stroke;
		}

		// Strip color encoding if it's a static value (not data-driven)
		if (specCopy.encoding?.color && specCopy.encoding.color.value) {
			delete specCopy.encoding.color;
		}

		return {
			...specCopy,
			width: 1200,
			autosize: {
				type: "fit",
				contains: "padding",
				resize: true
			},
			// Our config takes precedence over spec's config
			config: {
				...(finalSpec as any).config,
				...config,
			},
		};
	}, [finalSpec, config]);

	if (error) {
		return (
			<div className={`${className} p-4 text-center text-black/50`}>
				<p>{error}</p>
				<p className="text-sm mt-2">The visualization could not be rendered.</p>
			</div>
		);
	}

	if (!specWithConfig) {
		return (
			<div className={`${className} p-4 text-center text-black/50`}>
				<p>Loading visualization...</p>
			</div>
		);
	}

	return (
		<div className={`${className} flex justify-center items-center`} style={{ width: '100%', minHeight: '400px', position: 'relative', zIndex: 1, overflow: 'hidden' }}>
			<VegaEmbed
				spec={specWithConfig}
				options={{
					actions: false,
					renderer: 'svg',
					scaleFactor: 1
				}}
				onError={(error: any) => {
					console.error('Vega rendering error:', error);
					setError('Visualization rendering failed');
				}}
				style={{ width: '100%' }}
			/>
		</div>
	);
};
