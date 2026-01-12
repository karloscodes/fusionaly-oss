import { useState, useEffect } from "react";
import { Clock, ChevronDown } from "lucide-react";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { DateRange } from "react-day-picker";
import { DateRangePicker } from "@/components/date-range-picker";
import { convertRangeToDateRange } from "@/utils/date-range-converter";
import { router, usePage } from "@inertiajs/react";

interface TimeRangeSelectorProps {
	timeRanges: Array<{
		label: string;
		ranges: Array<{
			label: string;
			value: string;
			shortcut?: string;
		}>;
	}>;
	currentTimeRange: string;
	websiteId?: number | null; // Kept for backwards compatibility but no longer used
}

export const TimeRangeSelector = ({
	timeRanges,
	currentTimeRange,
}: TimeRangeSelectorProps) => {
	const [isTimeRangeOpen, setIsTimeRangeOpen] = useState(false);
	const [isCustomRangeOpen, setIsCustomRangeOpen] = useState(false);

	// Get current URL from Inertia
	const { url } = usePage();

	const initialDateRangeFrom = new Date();
	initialDateRangeFrom.setDate(new Date().getDate() - 30);

	const initialDateRangeTo = new Date();
	initialDateRangeTo.setDate(new Date().getDate() - 1);

	const [dateRange, setDateRange] = useState<DateRange | undefined>({
		from: initialDateRangeFrom,
		to: initialDateRangeTo,
	});

	useEffect(() => {
		const handleClickOutside = (event: MouseEvent) => {
			if (
				isTimeRangeOpen &&
				!(event.target as Element).closest(".time-range-dropdown")
			) {
				setIsTimeRangeOpen(false);
			}
		};

		const handleOpenCustomRange = () => {
			setIsCustomRangeOpen(true);
		};

		document.addEventListener("mousedown", handleClickOutside);
		window.addEventListener("openCustomRange", handleOpenCustomRange);
		return () => {
			document.removeEventListener("mousedown", handleClickOutside);
			window.removeEventListener("openCustomRange", handleOpenCustomRange);
		};
	}, [isTimeRangeOpen]);

	const handleTimeRangeClick = () => {
		setIsTimeRangeOpen(!isTimeRangeOpen);
	};

	const formatDate = (date: Date) => {
		return date.toLocaleDateString("en-US", {
			month: "short",
			day: "numeric",
			year: "numeric",
		});
	};

	const handleCustomRangeChange = (newDateRange: DateRange | undefined) => {
		setDateRange(newDateRange);
	};

	// Parse date string in a timezone-safe way
	// When we get "2025-07-06" from URL, we want to interpret it as a local date
	const parseLocalDate = (dateString: string): Date => {
		const [year, month, day] = dateString.split('-').map(Number);
		// Create date in local timezone (month is 0-indexed)
		return new Date(year, month - 1, day);
	};

	const formatDateToYYYYMMDD = (date: Date) => {
		const year = date.getFullYear();
		const month = String(date.getMonth() + 1).padStart(2, "0");
		const day = String(date.getDate()).padStart(2, "0");
		return `${year}-${month}-${day}`;
	};

	const handleCustomDateRangeApply = (newDateRange: DateRange | undefined) => {
		if (newDateRange?.from && newDateRange?.to) {
			const newDateRangeFromFormatted = formatDateToYYYYMMDD(newDateRange.from);
			const newDateRangeToFormatted = formatDateToYYYYMMDD(newDateRange.to);

			// Parse current URL from Inertia
			const [path, queryString] = url.split('?');
			const params = new URLSearchParams(queryString || '');

			params.set("from", newDateRangeFromFormatted);
			params.set("to", newDateRangeToFormatted);
			// Set range to custom for custom date selections
			params.set("range", "custom");
			// Remove website_id from query params - it's now in the URL path
			params.delete("website_id");

			// Navigate using Inertia
			const newUrl = `${path}?${params.toString()}`;
			router.get(newUrl, {}, {
				preserveScroll: false,
				preserveState: false,
			});
		}
	};

	// Convert all predefined ranges to explicit from/to dates
	const handleTimeRangeChange = (range: string) => {
		// Convert the range to explicit from/to dates
		const dateRange = convertRangeToDateRange(range);

		// Parse current URL from Inertia
		const [path, queryString] = url.split('?');
		const params = new URLSearchParams(queryString || '');

		params.set("from", dateRange.from);
		params.set("to", dateRange.to);
		// Add range parameter to persist the selection state
		params.set("range", range);
		// Remove website_id from query params - it's now in the URL path
		params.delete("website_id");

		// Navigate using Inertia
		const newUrl = `${path}?${params.toString()}`;
		router.get(newUrl, {}, {
			preserveScroll: false,
			preserveState: false,
		});
	};

	const getTimeRangeDisplay = () => {
		// Use Inertia's URL to ensure we get current params after navigation
		const [, queryString] = url.split('?');
		const searchParams = new URLSearchParams(queryString || '');
		const rangeParam = searchParams.get("range");

		// First check if we have a range parameter to show the proper label
		if (rangeParam && rangeParam !== "custom") {
			// Find the label for this range
			const rangeOption = timeRanges
				.flatMap((group) => group.ranges)
				.find((range) => range.value === rangeParam);

			if (rangeOption) {
				return rangeOption.label;
			}
		}

		// Check if we have explicit from/to dates in URL for custom range
		// Parse directly from Inertia URL, not window.location
		const fromParam = searchParams.get('from');
		const toParam = searchParams.get('to');
		if (fromParam && toParam) {
			const fromDate = parseLocalDate(fromParam);
			const toDate = parseLocalDate(toParam);
			return `${formatDate(fromDate)} - ${formatDate(toDate)}`;
		}

		// Fallback to current time range
		return (
			timeRanges
				.flatMap((group) => group.ranges)
				.find((range) => range.value === currentTimeRange)?.label ||
			"Select range"
		);
	};

	const handleRangeSelect = (range: string) => {
		if (range === "custom") {
			setIsCustomRangeOpen(true);
		} else {
			handleTimeRangeChange(range);
		}
		setIsTimeRangeOpen(false);
	};

	// Keyboard shortcut handler
	useEffect(() => {
		const handleKeyDown = (event: KeyboardEvent) => {
			// Don't trigger shortcuts when typing in input fields
			if (
				event.target instanceof HTMLInputElement ||
				event.target instanceof HTMLTextAreaElement
			) {
				return;
			}

			// Find matching shortcut
			const allRanges = timeRanges.flatMap((group) => group.ranges);
			const matchingRange = allRanges.find(
				(range) => range.shortcut?.toLowerCase() === event.key.toLowerCase()
			);

			if (matchingRange) {
				event.preventDefault();
				handleRangeSelect(matchingRange.value);
			}
		};

		document.addEventListener("keydown", handleKeyDown);
		return () => {
			document.removeEventListener("keydown", handleKeyDown);
		};
	}, [timeRanges]);

	return (
		<div className="relative">
			<button
				onClick={handleTimeRangeClick}
				className="inline-flex items-center gap-2 px-4 py-2 bg-white border-black border rounded hover:bg-black hover:text-white transition-colors time-range-dropdown text-sm"
				type="button"
			>
				<Clock className="w-4 h-4" />
				<span>{getTimeRangeDisplay()}</span>
				<ChevronDown className="w-4 h-4" />
			</button>

			{isTimeRangeOpen && (
				<div className="absolute right-0 mt-2 w-[240px] py-2 bg-white border-black border rounded shadow-lg z-50 time-range-dropdown">
					{timeRanges.map((group, groupIndex) => (
						<div
							key={`group-${group.label}-${groupIndex}`}
							className="px-2 time-range-dropdown"
						>
							<div className="text-xs font-medium text-gray-500 px-2 py-1">
								{group.label}
							</div>
							{group.ranges.map((range, rangeIndex) => (
								range.value === "custom" ? (
									<button
										key={`range-${range.value}-${rangeIndex}`}
										onClick={() => handleRangeSelect(range.value)}
										className="w-full px-2 py-1.5 text-left text-sm hover:bg-black hover:text-white rounded flex justify-between items-center time-range-dropdown"
										type="button"
									>
										<span>{range.label}</span>
										{range.shortcut && (
							<span className="text-xs px-1.5 py-0.5 bg-gray-100 text-gray-600 rounded">
								{range.shortcut.toUpperCase()}
							</span>
										)}
									</button>
								) : (
									<button
								key={`range-${range.value}-${rangeIndex}`}
								onClick={() => handleRangeSelect(range.value)}
								className="w-full px-2 py-1.5 text-left text-sm hover:bg-black hover:text-white rounded flex justify-between items-center time-range-dropdown"
								type="button"
							>
										<span>{range.label}</span>
										{range.shortcut && (
							<span className="text-xs px-1.5 py-0.5 bg-gray-100 text-gray-600 rounded">
								{range.shortcut.toUpperCase()}
							</span>
										)}
									</button>
								)
							))}
							{groupIndex < timeRanges.length - 1 && (
								<div className="my-2 border-t border-gray-200" />
							)}
						</div>
					))}
				</div>
			)}

			<Dialog open={isCustomRangeOpen} onOpenChange={setIsCustomRangeOpen}>
				<DialogContent className="sm:max-w-[600px]">
					<DialogHeader>
						<DialogTitle>Select Date Range</DialogTitle>
					</DialogHeader>
					<div className="space-y-4">
						<DateRangePicker
							onRangeChange={handleCustomRangeChange}
							onRangeApply={handleCustomDateRangeApply}
							initialDateRange={dateRange}
						/>
					</div>
				</DialogContent>
			</Dialog>
		</div>
	);
};
