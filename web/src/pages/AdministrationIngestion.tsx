import { useState } from "react";
import type { FC } from "react";
import { usePage, useForm } from "@inertiajs/react";
import {
	Card,
	CardContent,
	CardDescription,
	CardFooter,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { FlashMessageDisplay } from "@/components/ui/flash-message";
import { Textarea } from "@/components/ui/textarea";
import { Info, ExternalLink, Filter } from "lucide-react";
import type { FlashMessage } from "@/types";
import { AdministrationLayout } from "@/components/administration-layout";

interface Setting {
	key: string;
	value: string;
}

interface AdministrationIngestionProps {
	flash?: FlashMessage;
	error?: string;
	settings?: Setting[];
	[key: string]: unknown;
}

// Exported for Pro to wrap with its own layout
export const AdministrationIngestionContent: FC = () => {
	const { props } = usePage<AdministrationIngestionProps>();
	const { settings, flash, error } = props;
	const [showCopySuccess, setShowCopySuccess] = useState<boolean>(false);
	const [localFlash, setLocalFlash] = useState<FlashMessage | null>(null);

	// Get excluded IPs from server props
	const excludedIPsSetting = settings?.find((s) => s.key === "excluded_ips");
	const initialExcludedIPs = excludedIPsSetting?.value || "";

	// Form for updating ingestion settings
	const form = useForm({
		excluded_ips: initialExcludedIPs,
	});

	const addIPToExcluded = (ip: string) => {
		const ips = form.data.excluded_ips
			.split(",")
			.map((ip) => ip.trim())
			.filter((ip) => ip !== "");
		if (ips.includes(ip)) return;

		const newIPs = [...ips, ip].join(", ");
		form.setData("excluded_ips", newIPs);
	};

	// This external API call must remain as fetch
	const handleFindMyIP = async () => {
		try {
			const response = await fetch("https://api.ipify.org?format=json");
			const data = await response.json();
			const ip = data.ip;

			await navigator.clipboard.writeText(ip);
			setShowCopySuccess(true);
			setTimeout(() => setShowCopySuccess(false), 3000);

			if (
				window.confirm(
					`Your public IP is ${ip}. Would you like to add it to the excluded IPs list?`,
				)
			) {
				addIPToExcluded(ip);
			}
		} catch (err) {
			console.error("Error fetching IP:", err);
			setLocalFlash({
				type: "error",
				message: "Could not fetch your IP address. Please try again later.",
			});
			setTimeout(() => setLocalFlash(null), 5000);
		}
	};

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		form.post("/admin/ingestion/settings", {
			preserveScroll: true,
		});
	};

	// Combine server flash and local flash
	const displayFlash = flash || localFlash;

	return (
		<div className="space-y-6">
			<div>
				<h1 className="text-2xl font-bold text-black">Ingestion Settings</h1>
				<p className="text-black/60 mt-1">
					Configure what data gets collected and tracked
				</p>
			</div>

			<FlashMessageDisplay
				flash={displayFlash}
				error={error}
				showSuccessMessage={showCopySuccess}
				successMessage="IP address copied to clipboard!"
			/>

			<form onSubmit={handleSubmit}>
				<Card className="border-black shadow-sm">
					<CardHeader className="pb-4">
						<div className="flex justify-between items-center">
							<CardTitle className="text-lg flex items-center gap-2">
								<Filter className="h-5 w-5" /> IP Address Exclusion
							</CardTitle>
						</div>
						<CardDescription>
							Exclude specific IP addresses from analytics tracking.
						</CardDescription>
					</CardHeader>
					<CardContent className="space-y-6">
						<div className="bg-black/5 p-4 rounded-lg flex items-start gap-3 border">
							<div className="shrink-0 mt-0.5">
								<Info className="h-4 w-4 text-black/60" />
							</div>
							<div className="text-sm text-black/70">
								<p>
									Useful for excluding your own visits, internal team traffic,
									or testing services.
								</p>
								<button
									type="button"
									onClick={handleFindMyIP}
									className="inline-flex items-center gap-1 text-sm font-medium text-black/70 hover:text-black mt-2"
								>
									<ExternalLink className="h-3.5 w-3.5" />
									Find and add my current IP address
								</button>
							</div>
						</div>
						<div>
							<label
								htmlFor="excluded_ips"
								className="block text-sm font-medium mb-1.5"
							>
								Excluded IP Addresses
							</label>
							<Textarea
								id="excluded_ips"
								name="excluded_ips"
								placeholder="Enter comma-separated IP addresses (e.g., 192.168.1.1, 10.0.0.1)"
								value={form.data.excluded_ips}
								onChange={(e) => form.setData("excluded_ips", e.target.value)}
								disabled={form.processing}
								className="h-36 w-full resize-y border-black/20 focus:border-black focus:ring-black rounded-md"
							/>
							<p className="text-xs text-black/50 mt-1.5">
								Separate multiple IP addresses with commas.
							</p>
							{form.errors.excluded_ips && (
								<p className="text-sm text-red-600 mt-1">{form.errors.excluded_ips}</p>
							)}
						</div>
					</CardContent>
					<CardFooter className="flex justify-end border-t pt-4">
						<Button
							type="submit"
							disabled={form.processing}
							className="bg-black hover:bg-black/80 text-white rounded-md min-w-[140px]"
						>
							{form.processing ? "Saving..." : "Save Filtering"}
						</Button>
					</CardFooter>
				</Card>
			</form>
		</div>
	);
};

// Default export wraps content with OSS layout (unchanged behavior)
export const AdministrationIngestion: FC = () => (
	<AdministrationLayout currentPage="ingestion">
		<AdministrationIngestionContent />
	</AdministrationLayout>
);
