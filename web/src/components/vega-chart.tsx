import React, { useMemo, useState } from "react";
import { VegaEmbed } from "react-vega";

interface VegaChartProps {
	spec: any; // Using any for now to avoid type complexity
	data?: any[];
	className?: string;
}

export const VegaChart: React.FC<VegaChartProps> = ({ spec, data, className }) => {
	const [error, setError] = useState<string | null>(null);

	const finalSpec = useMemo(() => {
		try {
			// Validate spec exists
			if (!spec || typeof spec !== 'object') {
				console.error('Invalid Vega spec:', spec);
				setError('Invalid visualization specification');
				return null;
			}

			// Create a deep copy of the spec
			const specCopy = JSON.parse(JSON.stringify(spec));

			// If data is provided separately, inject it into the spec
			if (data && Array.isArray(data) && data.length > 0) {

				// Replace the data reference with actual values
				if (specCopy.data && typeof specCopy.data === 'object' && 'name' in specCopy.data) {
					// Replace named data source with inline values
					specCopy.data = { values: data };
				} else if (!specCopy.data) {
					// No data specified in spec, add it
					specCopy.data = { values: data };
				}

				// Auto-fix common field name mismatches by checking if the fields in encoding exist in data
				const firstDataItem = data[0];
				const dataFields = Object.keys(firstDataItem);

				// Check encoding fields
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

	// Vega-Lite theme configuration for Fusionaly - using custom brand colors
	const config = {
		background: "white",
		arc: { fill: "#1f77b4" },
		area: { fill: "#1f77b4" },
		line: { stroke: "#1f77b4", strokeWidth: 2 },
		path: { stroke: "#1f77b4" },
		rect: { fill: "#1f77b4" },
		shape: { stroke: "#1f77b4" },
		symbol: { fill: "#1f77b4", size: 30 },
		axis: {
			domainColor: "#E5E7EB",
			gridColor: "#F3F4F6",
			tickColor: "#E5E7EB",
			labelColor: "#6B7280",
			titleColor: "#374151",
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
			labelColor: "#6B7280",
			titleColor: "#374151",
		},
		range: {
			category: [
				"#1f77b4", // Blue
				"#ff7f0e", // Orange
				"#2ca02c", // Green
				"#d62728", // Red
				"#9467bd", // Purple
				"#8c564b", // Brown
				"#e377c2", // Pink
				"#7f7f7f", // Gray
				"#bcbd22", // Olive
				"#17becf"  // Cyan
			],
		},
	};

	const specWithConfig = useMemo(() => {
		if (!finalSpec) return null;

		// Remove width: "container" and set explicit width to avoid 0-width issue in modals
		const specCopy = { ...finalSpec };
		if (specCopy.width === "container") {
			delete specCopy.width;
		}

		// Use a large width that will be constrained by the container
		return {
			...specCopy,
			width: 1200, // Wider for better space utilization
			autosize: {
				type: "fit",
				contains: "padding",
				resize: true
			},
			config: {
				...config,
				...(finalSpec as any).config,
			},
		};
	}, [finalSpec, config]);

	// Show error state
	if (error) {
		return (
			<div className={`${className} p-4 text-center text-gray-500`}>
				<p>{error}</p>
				<p className="text-sm mt-2">The visualization could not be rendered.</p>
			</div>
		);
	}

	// Show loading state
	if (!specWithConfig) {
		return (
			<div className={`${className} p-4 text-center text-gray-500`}>
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
