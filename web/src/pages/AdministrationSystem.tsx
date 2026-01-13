import { useState } from "react";
import type { FC } from "react";
import { usePage, router } from "@inertiajs/react";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { FlashMessageDisplay } from "@/components/ui/flash-message";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Database,
	FileText,
	Download,
	Trash2,
	RefreshCw,
	Globe,
} from "lucide-react";
import type { FlashMessage } from "@/types";
import { AdministrationLayout } from "@/components/administration-layout";

interface AdministrationSystemProps {
	flash?: FlashMessage;
	error?: string;
	show_logs?: boolean;
	logs?: string;
	geolite_account_id?: string;
	geolite_license_key?: string;
	geolite_last_update?: string;
	geolite_db_exists?: boolean;
	[key: string]: unknown;
}

export const AdministrationSystem: FC = () => {
	const { props } = usePage<AdministrationSystemProps>();
	const { flash, error, show_logs, logs: serverLogs, geolite_account_id, geolite_license_key, geolite_last_update, geolite_db_exists } = props;
	const [exportLoading, setExportLoading] = useState(false);
	const [localFlash, setLocalFlash] = useState<FlashMessage | null>(null);
	const [geoAccountId, setGeoAccountId] = useState(geolite_account_id || "");
	const [geoLicenseKey, setGeoLicenseKey] = useState(geolite_license_key || "");
	const [geoSaving, setGeoSaving] = useState(false);

	// Use server logs if available
	const logs = serverLogs || "";

	const handlePurgeCache = () => {
		if (
			!window.confirm(
				"Are you sure you want to purge all caches? This action cannot be undone.",
			)
		) {
			return;
		}

		router.post("/admin/system/purge-cache", {}, {
			preserveScroll: true,
		});
	};

	// Database export must remain a fetch call for file download
	const handleExportDatabase = async () => {
		setExportLoading(true);
		try {
			const response = await fetch("/admin/api/system/export-database");

			if (response.ok) {
				const blob = await response.blob();
				const url = window.URL.createObjectURL(blob);
				const a = document.createElement("a");
				a.href = url;
				a.download = `fusionaly-backup-${new Date().toISOString().split("T")[0]}.db`;
				document.body.appendChild(a);
				a.click();
				window.URL.revokeObjectURL(url);
				document.body.removeChild(a);

				setLocalFlash({
					type: "success",
					message: "Database exported successfully",
				});
			} else {
				const result = await response.json();
				setLocalFlash({
					type: "error",
					message: result.error || "Failed to export database",
				});
			}
		} catch (err) {
			setLocalFlash({
				type: "error",
				message: `Error exporting database: ${(err as Error).message}`,
			});
		} finally {
			setExportLoading(false);
			setTimeout(() => setLocalFlash(null), 5000);
		}
	};

	const handleFetchLogs = () => {
		router.visit("/admin/administration/system?show_logs=true", {
			preserveScroll: true,
		});
	};

	const handleDownloadLogs = () => {
		if (!logs) {
			setLocalFlash({
				type: "error",
				message: "No logs to download. Please refresh logs first.",
			});
			setTimeout(() => setLocalFlash(null), 3000);
			return;
		}

		const blob = new Blob([logs], { type: "text/plain" });
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement("a");
		a.href = url;
		a.download = `fusionaly-logs-${new Date().toISOString().split("T")[0]}.txt`;
		document.body.appendChild(a);
		a.click();
		window.URL.revokeObjectURL(url);
		document.body.removeChild(a);

		setLocalFlash({
			type: "success",
			message: "Logs downloaded successfully",
		});
		setTimeout(() => setLocalFlash(null), 3000);
	};

	const handleSaveGeoLite = () => {
		setGeoSaving(true);
		router.post("/admin/system/geolite", {
			geolite_account_id: geoAccountId,
			geolite_license_key: geoLicenseKey,
		}, {
			preserveScroll: true,
			onFinish: () => setGeoSaving(false),
		});
	};

	// Combine server flash and local flash
	const displayFlash = flash || localFlash;

	return (
		<AdministrationLayout currentPage="system">
			<div className="space-y-6">
				<div>
					<h1 className="text-2xl font-bold text-gray-900">System Management</h1>
					<p className="text-gray-600 mt-1">
						Manage system operations, backups, and maintenance
					</p>
				</div>

				<FlashMessageDisplay flash={displayFlash} error={error} />

				{/* GeoLite Configuration */}
				<Card className="border-black shadow-sm">
					<CardHeader className="pb-4">
						<div className="flex justify-between items-center">
							<CardTitle className="text-lg flex items-center gap-2">
								<Globe className="h-5 w-5" /> GeoLite Configuration
							</CardTitle>
						</div>
						<CardDescription>
							Configure MaxMind GeoLite2 credentials for automatic database updates.
						</CardDescription>
					</CardHeader>
					<CardContent className="space-y-4">
						<div className="bg-blue-50 p-4 rounded-lg border border-blue-200 mb-4">
							<p className="text-sm text-blue-900">
								Get your free credentials at{" "}
								<a
									href="https://www.maxmind.com/en/geolite2/signup"
									target="_blank"
									rel="noopener noreferrer"
									className="underline"
								>
									maxmind.com
								</a>
								. Go to Account &rarr; Manage License Keys.
							</p>
						</div>

						<div className="space-y-2">
							<Label htmlFor="geolite_account_id">Account ID</Label>
							<Input
								id="geolite_account_id"
								type="text"
								value={geoAccountId}
								onChange={(e) => setGeoAccountId(e.target.value)}
								placeholder="e.g., 123456"
							/>
						</div>

						<div className="space-y-2">
							<Label htmlFor="geolite_license_key">License Key</Label>
							<Input
								id="geolite_license_key"
								type="password"
								value={geoLicenseKey}
								onChange={(e) => setGeoLicenseKey(e.target.value)}
								placeholder="Your MaxMind license key"
							/>
						</div>

						<Button
							onClick={handleSaveGeoLite}
							disabled={geoSaving}
							className="bg-black hover:bg-gray-800 text-white rounded-md"
						>
							{geoSaving ? "Saving..." : "Save GeoLite Settings"}
						</Button>

						{/* Status information */}
						<div className="pt-4 mt-4 border-t border-gray-200 space-y-2">
							<div className="flex items-center gap-2 text-sm">
								<span className="text-gray-600">Database Status:</span>
								{geolite_db_exists ? (
									<span className="text-green-600 font-medium">Downloaded</span>
								) : (
									<span className="text-amber-600 font-medium">Not downloaded</span>
								)}
							</div>
							{geolite_last_update && (
								<div className="flex items-center gap-2 text-sm">
									<span className="text-gray-600">Last Updated:</span>
									<span className="text-gray-900">{geolite_last_update}</span>
								</div>
							)}
							{!geolite_db_exists && !geolite_last_update && (geoAccountId && geoLicenseKey) && (
								<p className="text-xs text-gray-500">
									Database will be downloaded automatically within 24 hours after saving credentials.
								</p>
							)}
						</div>
					</CardContent>
				</Card>

				{/* Cache Management */}
				<Card className="border-black shadow-sm">
					<CardHeader className="pb-4">
						<div className="flex justify-between items-center">
							<CardTitle className="text-lg flex items-center gap-2">
								<Trash2 className="h-5 w-5" /> Cache Management
							</CardTitle>
						</div>
						<CardDescription>
							Clear cached data to free up space or resolve issues.
						</CardDescription>
					</CardHeader>
					<CardContent>
						<Button
							onClick={handlePurgeCache}
							className="bg-red-600 hover:bg-red-700 text-white rounded-md"
						>
							<Trash2 className="h-4 w-4 mr-2" />
							Purge All Caches
						</Button>
						<p className="text-xs text-gray-500 mt-2">
							This will clear all generic caches and temporary data.
						</p>
					</CardContent>
				</Card>

				{/* Database Export */}
				<Card className="border-black shadow-sm">
					<CardHeader className="pb-4">
						<div className="flex justify-between items-center">
							<CardTitle className="text-lg flex items-center gap-2">
								<Database className="h-5 w-5" /> Database Export
							</CardTitle>
						</div>
						<CardDescription>
							Download a complete backup of your database.
						</CardDescription>
					</CardHeader>
					<CardContent>
						<div className="bg-yellow-50 p-4 rounded-lg border border-yellow-200 mb-4">
							<p className="text-sm text-yellow-900">
								Warning: The exported database contains
								sensitive data including user passwords (hashed), API keys, and
								analytics data. Store it securely.
							</p>
						</div>
						<Button
							onClick={handleExportDatabase}
							disabled={exportLoading}
							className="bg-black hover:bg-gray-800 text-white rounded-md"
						>
							{exportLoading ? (
								<>
									<RefreshCw className="h-4 w-4 mr-2 animate-spin" />
									Exporting...
								</>
							) : (
								<>
									<Download className="h-4 w-4 mr-2" />
									Export Database
								</>
							)}
						</Button>
					</CardContent>
				</Card>

				{/* Application Logs */}
				<Card className="border-black shadow-sm">
					<CardHeader className="pb-4">
						<div className="flex justify-between items-center">
							<CardTitle className="text-lg flex items-center gap-2">
								<FileText className="h-5 w-5" /> Application Logs
							</CardTitle>
							<div className="flex gap-2">
								<Button
									onClick={handleFetchLogs}
									variant="outline"
									size="sm"
									className="border-black text-black hover:bg-gray-100"
								>
									<RefreshCw className="h-4 w-4" />
								</Button>
								<Button
									onClick={handleDownloadLogs}
									disabled={!logs}
									variant="outline"
									size="sm"
									className="border-black text-black hover:bg-gray-100"
								>
									<Download className="h-4 w-4" />
								</Button>
							</div>
						</div>
						<CardDescription>
							View application logs for debugging and monitoring.
						</CardDescription>
					</CardHeader>
					<CardContent>
						{!show_logs || !logs ? (
							<div className="text-center py-8 text-gray-500">
								<FileText className="h-12 w-12 mx-auto mb-3 text-gray-400" />
								<p className="text-sm">
									Click the refresh button to load logs
								</p>
							</div>
						) : (
							<Textarea
								value={logs}
								readOnly
								className="font-mono text-xs h-96 resize-y border-gray-300 focus:border-black focus:ring-black"
								placeholder="No logs available"
							/>
						)}
					</CardContent>
				</Card>
			</div>
		</AdministrationLayout>
	);
};
