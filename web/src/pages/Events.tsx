import { useEffect, useState } from "react";
import type { KeyboardEvent } from "react";
import { usePage, router } from "@inertiajs/react";
import { EventsTable } from "../components/events-table";
import type { EventsResponse } from "@/types";
import { Zap, Search, X, Layers } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { formatNumber } from "@/lib/utils";
import { WebsiteLayout } from "@/components/website-layout";

interface Website {
	id: number;
	domain: string;
}

interface EventsProps extends EventsResponse {
	current_website_id: number;
	website_domain?: string;
	websites?: Website[];
	flash?: any;
	error?: string;
	[key: string]: any;
}

export function Events() {
	const { props } = usePage<EventsProps>();
	const { events, pagination, current_website_id, website_domain, websites } = props;
	const [eventsData, setEventsData] = useState<EventsResponse | null>({ events, pagination });
	const isLoading = false; // No loading state needed with Inertia SSR
	const websiteId = current_website_id || 0;
	const websiteDomain = website_domain || "";

	// Get current filters and page from URL
	const searchParams = new URLSearchParams(window.location.search);
	const currentPage = Number.parseInt(searchParams.get("page") || "1");

	// Local state for filters
	const [filters, setFilters] = useState({
		referrer: searchParams.get("referrer") || "",
		user: searchParams.get("user") || "",
		type: searchParams.get("type") || "",
		event_key: searchParams.get("event_key") || "",
		from: searchParams.get("from") || "",
		to: searchParams.get("to") || "",
		range: searchParams.get("range") || "last_7_days",
	});

	// Session grouping state (persisted in localStorage)
	const [groupBySessions, setGroupBySessions] = useState(() => {
		try {
			const stored = localStorage.getItem("events_group_by_sessions");
			return stored === "true";
		} catch {
			return false;
		}
	});

	// Persist session grouping preference
	const toggleSessionGrouping = () => {
		const newValue = !groupBySessions;
		setGroupBySessions(newValue);
		try {
			localStorage.setItem("events_group_by_sessions", String(newValue));
		} catch (error) {
			console.error("Failed to save session grouping preference:", error);
		}
	};

	useEffect(() => {
		// Update eventsData when props change (Inertia navigation)
		setEventsData({ events, pagination });
	}, [events, pagination]);

	// Base URL for this website's events page
	const eventsBaseUrl = `/admin/websites/${websiteId}/events`;

	const buildUrl = (params: Record<string, string>) => {
		const newParams = new URLSearchParams(window.location.search);
		for (const [key, value] of Object.entries(params)) {
			if (value) {
				newParams.set(key, value);
			} else {
				newParams.delete(key);
			}
		}
		return `${eventsBaseUrl}?${newParams.toString()}`;
	};

	const handleFilterChange = (key: string, value: string) => {
		setFilters((prev) => ({ ...prev, [key]: value }));
	};

	const applyFilters = (overrideFilters?: typeof filters) => {
		const filtersToApply = overrideFilters || filters;
		const params: Record<string, string> = { page: "1" };

		// Add all non-empty filter values
		for (const [key, value] of Object.entries(filtersToApply)) {
			if (key === "url" || key === "user" || key === "from" || key === "to") continue;
			if (value !== undefined && value !== null && value !== "") {
				params[key] = String(value);
			}
		}

		router.get(eventsBaseUrl, params, { preserveState: false });
	};

	const handleQuickFilter = (key: string, value: string) => {
		const newFilters = { ...filters, [key]: value };
		applyFilters(newFilters);
	};

	const clearFilters = () => {
		// Build clean URL with default range
		const params: Record<string, string> = {
			page: "1",
			range: "last_7_days",
		};

		router.get(eventsBaseUrl, params, { preserveState: false });
	};

	const handleKeyPress = (
		e: KeyboardEvent<HTMLInputElement | HTMLSelectElement>,
	) => {
		if (e.key === "Enter") {
			applyFilters();
		}
	};

	return (
		<WebsiteLayout
			websiteId={websiteId}
			websiteDomain={websiteDomain}
			currentPath={eventsBaseUrl}
			websites={websites}
		>
			<div className="py-4">
				<div className="flex flex-col gap-4">
					<PageHeader
						title="Events"
						icon={Zap}
					/>

				{/* Filters Bar */}
				<div className="flex items-center justify-between gap-4 flex-wrap">
					<div className="flex items-center gap-3 flex-wrap">
						{/* Date Range */}
						<div className="flex items-center gap-1">
							{[
								{ value: "today", label: "Today" },
								{ value: "yesterday", label: "Yesterday" },
								{ value: "last_7_days", label: "7d" },
								{ value: "last_30_days", label: "30d" },
								{ value: "this_month", label: "Month" },
							].map((range) => (
								<button
									key={range.value}
									type="button"
									onClick={() => handleQuickFilter("range", range.value)}
									className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
										filters.range === range.value
											? "bg-gray-900 text-white"
											: "bg-white text-gray-600 hover:bg-gray-100 hover:text-gray-900"
									}`}
								>
									{range.label}
								</button>
							))}
						</div>

						<div className="h-5 w-px bg-gray-200" />

						{/* Type Filter */}
						<div className="flex items-center gap-1">
							<button
								type="button"
								onClick={() => handleQuickFilter("type", "")}
								className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
									filters.type === ""
										? "bg-gray-900 text-white"
										: "bg-white text-gray-600 hover:bg-gray-100 hover:text-gray-900"
								}`}
							>
								All
							</button>
							<button
								type="button"
								onClick={() => handleQuickFilter("type", "page")}
								className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
									filters.type === "page"
										? "bg-gray-900 text-white"
										: "bg-white text-gray-600 hover:bg-gray-100 hover:text-gray-900"
								}`}
							>
								Pages
							</button>
							<button
								type="button"
								onClick={() => handleQuickFilter("type", "event")}
								className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
									filters.type === "event"
										? "bg-gray-900 text-white"
										: "bg-white text-gray-600 hover:bg-gray-100 hover:text-gray-900"
								}`}
							>
								Events
							</button>
						</div>

						<div className="h-5 w-px bg-gray-200" />

						{/* Search Inputs */}
						<div className="relative">
							<Search className="absolute left-2.5 top-1/2 transform -translate-y-1/2 h-3.5 w-3.5 text-gray-400" />
							<input
								type="text"
								placeholder="Referrer..."
								value={filters.referrer}
								onChange={(e) => handleFilterChange("referrer", e.target.value)}
								onKeyPress={handleKeyPress}
								className="w-32 pl-8 pr-3 py-1.5 text-xs border border-gray-200 rounded-md bg-white focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent placeholder:text-gray-400"
							/>
						</div>
						<div className="relative">
							<Search className="absolute left-2.5 top-1/2 transform -translate-y-1/2 h-3.5 w-3.5 text-gray-400" />
							<input
								type="text"
								placeholder="Event Key..."
								value={filters.event_key}
								onChange={(e) => handleFilterChange("event_key", e.target.value)}
								onKeyPress={handleKeyPress}
								className="w-32 pl-8 pr-3 py-1.5 text-xs border border-gray-200 rounded-md bg-white focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent placeholder:text-gray-400"
							/>
						</div>
					</div>

					<div className="flex items-center gap-2">
						<button
							type="button"
							onClick={toggleSessionGrouping}
							className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
								groupBySessions
									? "bg-gray-900 text-white"
									: "bg-white text-gray-600 hover:bg-gray-100 hover:text-gray-900"
							}`}
							title={groupBySessions ? "Grouped by sessions" : "Group by sessions"}
						>
							<Layers className="h-3.5 w-3.5" />
							<span>Sessions</span>
						</button>

						{(filters.referrer || filters.type || filters.event_key || filters.range !== "last_7_days") && (
							<button
								type="button"
								onClick={clearFilters}
								className="flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-gray-500 bg-white hover:bg-gray-100 hover:text-gray-700 rounded-md transition-colors"
							>
								<X className="h-3.5 w-3.5" />
								Clear
							</button>
						)}
					</div>
				</div>

				{/* Events Table Card */}
				<div className="bg-white border border-black rounded-lg overflow-hidden">
					<div className="p-4">
						<EventsTable
							events={eventsData?.events ?? []}
							isLoading={isLoading}
							groupBySessions={groupBySessions}
						/>
					</div>
				</div>

				{/* Pagination */}
				{eventsData && (
					<div className="flex justify-between items-center text-sm">
						<div className="text-sm text-gray-500">
							Page {eventsData.pagination.current_page} of{" "}
							{eventsData.pagination.total_pages} (
							{formatNumber(eventsData.pagination.total_items)} events)
						</div>
						<div className="flex gap-2">
							<a
								href={buildUrl({ page: (currentPage - 1).toString() })}
								className={`px-4 py-2 text-sm border border-gray-200 rounded-lg text-gray-700 bg-white font-medium hover:bg-gray-50 hover:border-gray-300 transition-colors ${currentPage === 1 || isLoading
										? "opacity-50 pointer-events-none cursor-not-allowed"
										: ""
									}`}
							>
								Previous
							</a>
							<a
								href={buildUrl({ page: (currentPage + 1).toString() })}
								className={`px-4 py-2 text-sm border border-gray-200 rounded-lg text-gray-700 bg-white font-medium hover:bg-gray-50 hover:border-gray-300 transition-colors ${currentPage === eventsData?.pagination.total_pages ||
										isLoading
										? "opacity-50 pointer-events-none cursor-not-allowed"
										: ""
									}`}
							>
								Next
							</a>
						</div>
					</div>
				)}
				</div>
			</div>
		</WebsiteLayout>
	);
}
