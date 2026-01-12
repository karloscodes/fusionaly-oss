import { useEffect, useState } from "react";
import { Globe, ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import type { Website } from "../types";
import { PlusCircle } from "lucide-react";

interface WebsiteSelectorProps {
	selectedWebsiteId?: number;
	websites?: Website[];
	className?: string;
	onWebsiteChange?: (websiteId: number) => void;
	onAddWebsite?: () => void;
}

export const WebsiteSelector = ({
	selectedWebsiteId,
	websites: websitesProp,
	className,
	onWebsiteChange,
	onAddWebsite,
}: WebsiteSelectorProps) => {
	const [websites, setWebsites] = useState<Website[]>(websitesProp || []);
	const [selectedId, setSelectedId] = useState<number | undefined>(
		selectedWebsiteId,
	);

	// Update websites when prop changes (Inertia navigation)
	useEffect(() => {
		if (websitesProp && Array.isArray(websitesProp)) {
			setWebsites(websitesProp);
		}
	}, [websitesProp]);

	useEffect(() => {
		if (selectedWebsiteId !== undefined && selectedWebsiteId !== selectedId) {
			setSelectedId(selectedWebsiteId);
		}
	}, [selectedWebsiteId, selectedId]);

	const handleWebsiteChange = (value: string) => {
		const websiteId = Number.parseInt(value, 10);
		if (Number.isNaN(websiteId) || websiteId <= 0) {
			console.error("Invalid website ID selected:", value);
			return;
		}

		setSelectedId(websiteId);

		if (onWebsiteChange) {
			onWebsiteChange(websiteId);
		}
	};

	if (websites.length === 0) {
		if (onAddWebsite) {
			return (
				<button
					data-testid="website-selector"
					aria-label="Add website"
					className={cn(
						"inline-flex items-center gap-2 px-4 py-2 bg-white border-black border rounded hover:bg-black hover:text-white transition-colors text-sm justify-center min-w-[200px]",
						className,
					)}
					type="button"
					onClick={onAddWebsite}
				>
					<PlusCircle className="w-4 h-4" />
					<span>New Website</span>
				</button>
			);
		}

		return (
			<div
				data-testid="website-selector"
				className={cn(
					"inline-flex items-center gap-2 px-4 py-2 bg-gray-100 border-gray-300 border rounded text-gray-500 text-sm justify-center min-w-[200px]",
					className,
				)}
			>
				<Globe className="w-4 h-4" />
				<span>No websites</span>
			</div>
		);
	}

	const selectedWebsite =
		selectedId !== undefined
			? websites.find((w) => w.id === selectedId)
			: undefined;

	const displayName = selectedWebsite?.domain || "Select website";

	return (
		<button
			data-testid="website-selector"
			aria-label="Select website"
			className={cn(
				"inline-flex items-center gap-2 px-4 py-2 bg-white border-black border rounded hover:bg-black hover:text-white transition-colors text-sm justify-between min-w-[200px]",
				className,
			)}
			type="button"
			onClick={() => document.getElementById("website-selector")?.click()}
		>
			<Globe className="w-4 h-4" />
			<span className="flex-1 text-center">
				{displayName}
			</span>
			<ChevronDown className="w-4 h-4" />
			<Select
				value={selectedId?.toString()}
				onValueChange={handleWebsiteChange}
			>
				<SelectTrigger
					className="w-0 h-0 p-0 m-0 border-0 absolute opacity-0"
					id="website-selector"
				>
					<SelectValue />
				</SelectTrigger>
				<SelectContent className="border-black min-w-[250px] w-auto">
					{websites.map((website) => (
						<SelectItem
							key={website.id}
							value={website.id.toString()}
							className={cn(
								"py-2 px-2 cursor-pointer hover:bg-black hover:text-white focus:bg-black focus:text-white",
								selectedId === website.id && "font-medium",
								selectedId === website.id &&
								"bg-black text-white",
							)}
						>
							{website.domain}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</button>
	);
};
