import { useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { ChevronDown, Check, SquareArrowOutUpRight } from "lucide-react";
import DataTable from "./data-table";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import type { ReferrersCardProps, MetricType } from "../types";

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

	// Get the icon for the current metric type
	const getIconForMetricType = () => {
		return <SquareArrowOutUpRight className="w-4 h-4" />;
	};

	return (
		<Card className="border-black">
			<CardContent className="p-4 sm:p-6">
				<div className="flex justify-between items-center mb-4">
					<div className="flex items-center gap-2">
						{getIconForMetricType()}
						<span>{getMetricDisplayName(selectedMetricType)}</span>
					</div>
					<div>
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button
									variant="outline"
									size="default"
									className="py-1.5 sm:py-2 px-3 sm:px-4 h-auto sm:h-[38px] text-sm"
								>
									{getMetricDisplayName(selectedMetricType)}{" "}
									<ChevronDown className="ml-1 h-3 w-3" />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent
								align="end"
								className="max-h-[300px] overflow-y-auto"
							>
								<DropdownMenuItem
									onClick={() => handleMetricTypeChange("referrers")}
									className="flex items-center justify-between"
								>
									<span className="truncate">Top Referrers</span>
									{selectedMetricType === "referrers" && (
										<Check className="h-4 w-4 ml-2" />
									)}
								</DropdownMenuItem>
								<DropdownMenuItem
									onClick={() => handleMetricTypeChange("utm_sources")}
									className="flex items-center justify-between"
								>
									<span className="truncate">UTM Source</span>
									{selectedMetricType === "utm_sources" && (
										<Check className="h-4 w-4 ml-2" />
									)}
								</DropdownMenuItem>
								<DropdownMenuItem
									onClick={() => handleMetricTypeChange("utm_mediums")}
									className="flex items-center justify-between"
								>
									<span className="truncate">UTM Medium</span>
									{selectedMetricType === "utm_mediums" && (
										<Check className="h-4 w-4 ml-2" />
									)}
								</DropdownMenuItem>
								<DropdownMenuItem
									onClick={() => handleMetricTypeChange("utm_campaigns")}
									className="flex items-center justify-between"
								>
									<span className="truncate">UTM Campaign</span>
									{selectedMetricType === "utm_campaigns" && (
										<Check className="h-4 w-4 ml-2" />
									)}
								</DropdownMenuItem>
								<DropdownMenuItem
									onClick={() => handleMetricTypeChange("utm_terms")}
									className="flex items-center justify-between"
								>
									<span className="truncate">UTM Term</span>
									{selectedMetricType === "utm_terms" && (
										<Check className="h-4 w-4 ml-2" />
									)}
								</DropdownMenuItem>
								<DropdownMenuItem
									onClick={() => handleMetricTypeChange("utm_contents")}
									className="flex items-center justify-between"
								>
									<span className="truncate">UTM Content</span>
									{selectedMetricType === "utm_contents" && (
										<Check className="h-4 w-4 ml-2" />
									)}
								</DropdownMenuItem>
								<DropdownMenuItem
									onClick={() => handleMetricTypeChange("ref_params")}
									className="flex items-center justify-between"
								>
									<span className="truncate">Ref</span>
									{selectedMetricType === "ref_params" && (
										<Check className="h-4 w-4 ml-2" />
									)}
								</DropdownMenuItem>
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
