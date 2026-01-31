import type { ReactNode } from "react";
import type { DataItem } from "../types";
import { FileBarChart, ChevronLeft, ChevronRight } from "lucide-react";
import { useState, useMemo, useEffect, useRef } from "react";
import { formatNumber } from "@/lib/utils";

type ColumnAlignment = "left" | "center" | "right";

interface DataTableColumn {
	name: string;
	label: string;
	render?: (item: DataItem) => ReactNode;
	align?: ColumnAlignment;
	widthClass?: string;
	hideOnMobile?: boolean;
}

interface DataTableProps {
	title?: ReactNode;
	data: DataItem[];
	showPercentage?: boolean;
	totalVisitors?: number;
	columns?: DataTableColumn[];
	emptyMessage?: string;
	pageSize?: number;
}

const DataTable = ({
	title,
	data,
	showPercentage = false,
	totalVisitors,
	columns = [
		{ name: "name", label: "Name" },
		{ name: "count", label: "Views" },
	],
	emptyMessage = "No data available yet.",
	pageSize = 10,
}: DataTableProps) => {
	// Calculate total for fallback (if totalVisitors isn't provided)
	const total = Math.max(
		(data && Array.isArray(data)) ? data.reduce((acc: number, curr: DataItem) => acc + curr.count, 0) : 0,
		1,
	);
	// Use totalVisitors if provided, otherwise fall back to total
	const denominator =
		totalVisitors !== undefined ? Math.max(totalVisitors, 1) : total;
	const maxCount = (data && Array.isArray(data) && data.length > 0) ? Math.max(...data.map((item) => item.count), 0) : 0;

	// Pagination state
	const [currentPage, setCurrentPage] = useState(1);
	const totalPages = Math.ceil(data.length / pageSize);

	// Use refs to track previous data and pageSize
	const prevDataLengthRef = useRef(data.length);
	const prevPageSizeRef = useRef(pageSize);

	// Reset page when data or pageSize changes
	useEffect(() => {
		if (
			prevDataLengthRef.current !== data.length ||
			prevPageSizeRef.current !== pageSize
		) {
			setCurrentPage(1);
			prevDataLengthRef.current = data.length;
			prevPageSizeRef.current = pageSize;
		}
	}, [data.length, pageSize]);

	// Get current page data
	const currentItems = useMemo(() => {
		const startIndex = (currentPage - 1) * pageSize;
		return data.slice(startIndex, startIndex + pageSize);
	}, [data, currentPage, pageSize]);

	// Pagination handlers
	const goToNextPage = () => {
		if (currentPage < totalPages) {
			setCurrentPage(currentPage + 1);
		}
	};

	const goToPreviousPage = () => {
		if (currentPage > 1) {
			setCurrentPage(currentPage - 1);
		}
	};

	return (
		<div className="h-full flex flex-col">
			{title && (
				<div className="pb-2 border-b border-black/10">
					<h3 className="font-medium text-base">{title}</h3>
				</div>
			)}

			{data.length === 0 ? (
				<div className="flex-grow flex items-center justify-center">
					<div className="flex flex-col items-center text-center">
						<div className="w-16 h-16 rounded-full bg-black/5 flex items-center justify-center mb-3">
							<FileBarChart className="h-8 w-8 text-black/40" />
						</div>
						<h3 className="text-sm font-medium text-black mb-1">
							No data available
						</h3>
						<p className="text-sm text-black/50 max-w-sm">{emptyMessage}</p>
					</div>
				</div>
			) : (
				<>
					{/* Column Headers */}
					<div className="flex justify-between text-xs font-medium text-black/50 py-2">
						<span className="truncate mr-4 overflow-hidden flex-1 min-w-0">
							{columns[0].label}
						</span>
						<div className="flex items-center gap-3 sm:gap-6 tabular-nums">
							{columns.slice(1).map((column) => {
								const alignment =
									column.align === "left"
										? "text-left"
										: column.align === "center"
											? "text-center"
											: "text-right";
								const width = column.widthClass ?? "w-16";
								const hideOnMobile = column.hideOnMobile ? "hidden sm:block" : "";

								return (
									<span
										key={column.name}
										className={`${width} ${alignment} ${hideOnMobile}`}
									>
										{column.label}
									</span>
								);
							})}
							{showPercentage && <span className="w-12 text-right">%</span>}
						</div>
					</div>

					{/* Data list */}
					<div className="space-y-0.5 flex-grow overflow-y-hidden">
						{currentItems.map((item) => (
							<div
								key={`${item.name}-${item.count}`}
								className="flex justify-between items-stretch hover:bg-black/5 transition-colors"
							>
								<div className="flex-1 relative min-w-0 pr-2">
					<div
						className="absolute inset-0 bg-blue-50/50"
										style={{
											width: `${maxCount > 0 ? (item.count / maxCount) * 100 : 0}%`,
										}}
									/>
									<div className="relative py-2 px-2 overflow-hidden">
										<span
											className="truncate block text-sm font-medium w-full"
											title={item.name}
										>
											{columns[0].render ? columns[0].render(item) : item.name}
										</span>
									</div>
								</div>

								<div className="flex items-center gap-3 sm:gap-6 tabular-nums text-sm py-2 px-2 ml-auto">
									{columns.slice(1).map((column) => {
										const alignment =
											column.align === "left"
												? "text-left"
												: column.align === "center"
													? "text-center"
													: "text-right";
										const width = column.widthClass ?? "w-16";
										const hideOnMobile = column.hideOnMobile ? "hidden sm:inline-block" : "";
										const rawValue = column.render
											? column.render(item)
											: (item as unknown as Record<string, unknown>)[column.name];

										const displayValue = column.render
											? rawValue
											: (typeof rawValue !== "number"
												? rawValue ?? "—"
												: formatNumber(rawValue));

										return (
											<span
												key={column.name}
												className={`font-medium ${width} ${alignment} ${hideOnMobile}`}
											>
												{displayValue as ReactNode}
											</span>
										);
									})}
									{showPercentage && (
										<span className="w-12 text-right font-medium">
											{((item.count / denominator) * 100).toFixed(1)}%
										</span>
									)}
								</div>
							</div>
						))}
					</div>

					{/* Pagination controls - always at the bottom */}
					<div className="py-2 flex items-center justify-center gap-2 text-xs h-2">
						<button
							type="button"
							onClick={goToPreviousPage}
							disabled={currentPage === 1 || totalPages <= 1}
							className={`p-1 rounded flex items-center justify-center ${currentPage === 1 || totalPages <= 1
								? "text-black/30 cursor-not-allowed"
								: "text-black/60"
								}`}
							aria-label="Previous page"
						>
							<ChevronLeft className="w-3.5 h-3.5" />
						</button>
						<span className="text-black/60 font-medium text-xs flex items-center justify-center h-full">
							{currentPage} of {totalPages}
						</span>
						<button
							type="button"
							onClick={goToNextPage}
							disabled={currentPage === totalPages || totalPages <= 1}
							className={`p-1 rounded flex items-center justify-center ${currentPage === totalPages || totalPages <= 1
								? "text-black/30 cursor-not-allowed"
								: "text-black/60"
								}`}
							aria-label="Next page"
						>
							<ChevronRight className="w-3.5 h-3.5" />
						</button>
					</div>
				</>
			)}
		</div>
	);
};

export default DataTable;
