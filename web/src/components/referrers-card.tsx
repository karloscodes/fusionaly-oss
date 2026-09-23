import { useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { ChevronDown, Check } from "lucide-react";
import { tabClass } from "@/lib/tab-class";
import DataTable from "./data-table";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ReferrersCardProps, MetricType } from "../types";

// Where visitors came from, grouped the way people look for it.
const MENU_GROUPS: { label: string; items: MetricType[] }[] = [
	{ label: "Sources", items: ["referrers"] },
	{ label: "Campaigns (UTM)", items: ["utm_sources", "utm_mediums", "utm_campaigns", "utm_terms", "utm_contents"] },
	{ label: "Links", items: ["ref_params"] },
];

export const ReferrersCard = ({ data }: ReferrersCardProps) => {
	// State for the selected UTM metric type
	const [selectedMetricType, setSelectedMetricType] =
		useState<MetricType>("referrers");

	// Helper function to get metric display name
	const getMetricDisplayName = (metricType: MetricType): string => {
		const metricNames: Record<MetricType, string> = {
			referrers: "Referrers",
			utm_sources: "UTM Source",
			utm_mediums: "UTM Medium",
			utm_campaigns: "UTM Campaign",
			utm_terms: "UTM Term",
			utm_contents: "UTM Content",
			ref_params: "Ref",
		};
		return metricNames[metricType] || metricType;
	};

	// Get the correct data array based on selected metric type
	const getDataForMetricType = () => {
		switch (selectedMetricType) {
			case "referrers":
				return data.top_referrers || [];
			case "utm_sources":
				return data.top_utm_sources || [];
			case "utm_mediums":
				return data.top_utm_mediums || [];
			case "utm_campaigns":
				return data.top_utm_campaigns || [];
			case "utm_terms":
				return data.top_utm_terms || [];
			case "utm_contents":
				return data.top_utm_contents || [];
			case "ref_params":
				return data.top_ref_params || [];
			default:
				return data.top_referrers || [];
		}
	};

	// Reset the filter when changing metric type
	const handleMetricTypeChange = (metricType: MetricType): void => {
		setSelectedMetricType(metricType);
	};

	// Get the data to display
	const displayData = getDataForMetricType();


	return (
		<Card className="rounded-xl border-black">
			<CardContent className="p-4 sm:p-6">
				<div className="flex justify-between items-center mb-4">
					<h2 className="text-base font-semibold text-gray-900">Referrers</h2>
					<div>
						<DropdownMenu modal={false}>
							<DropdownMenuTrigger asChild>
								<button
									type="button"
									className={`${tabClass(true)} inline-flex items-center gap-1.5`}
								>
									{selectedMetricType === "referrers" ? "Top Referrers" : getMetricDisplayName(selectedMetricType)}
									<ChevronDown className="h-3 w-3" />
								</button>
							</DropdownMenuTrigger>
							<DropdownMenuContent
								align="end"
								className="max-h-[300px] overflow-y-auto"
							>
								{MENU_GROUPS.map((group, gi) => (
									<DropdownMenuGroup key={group.label}>
										{gi > 0 && <DropdownMenuSeparator />}
										<DropdownMenuLabel className="text-xs font-medium uppercase tracking-wide text-gray-500">
											{group.label}
										</DropdownMenuLabel>
										{group.items.map((type) => (
											<DropdownMenuItem
												key={type}
												onClick={() => handleMetricTypeChange(type)}
												className="flex items-center justify-between"
											>
												<span className="truncate">{type === "referrers" ? "Top Referrers" : getMetricDisplayName(type)}</span>
												{selectedMetricType === type && <Check className="h-4 w-4 ml-2" />}
											</DropdownMenuItem>
										))}
									</DropdownMenuGroup>
								))}
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</div>

				<div className="h-[320px] sm:h-[380px] flex flex-col">
					<DataTable
						data={displayData || []}
						showPercentage={true}
						pageSize={8}
						columns={[
							{
								name: "name",
								label: getMetricDisplayName(selectedMetricType).replace(
									"Top ",
									"",
								),
							},
							{ name: "count", label: "Visitors" },
						]}
						emptyMessage={`No ${getMetricDisplayName(selectedMetricType).toLowerCase()} data available.`}
					/>
				</div>
			</CardContent>
		</Card>
	);
};
