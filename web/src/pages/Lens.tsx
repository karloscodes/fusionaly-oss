import React, { useState, useCallback, useRef, useMemo, useEffect } from "react";
import { usePage, router, useForm, Link } from "@inertiajs/react";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FlashMessageDisplay } from "@/components/ui/flash-message";
import { LoadingPulse } from "@/components/loading-pulse";
import { WebsiteLayout } from "@/components/website-layout";
import { formatNumber, cn } from "@/lib/utils";
import { VegaChart } from "@/components/VegaChart";
import { AskAIChat } from "@/components/AskAIChat";
import { ModelSelector } from "@/components/ModelSelector";
import { Trash2, Edit3, Copy, RefreshCw, Loader2, Key } from "lucide-react";
import type { FlashMessage } from "@/types";
import {
	BarChart,
	Bar,
	XAxis,
	YAxis,
	CartesianGrid,
	Tooltip as RechartsTooltip,
	ResponsiveContainer,
	Legend,
} from "recharts";

interface InitialResult {
	id: number;
	results?: Record<string, unknown>[];
	error?: string;
	query_type?: string;
	vega_spec?: string;
}

interface LensPageProps {
	current_website_id?: number;
	website_domain?: string;
	saved_queries?: SavedQuery[];
	websites?: Website[];
	initial_results?: InitialResult[];
	ai_configured?: boolean;
	flash?: FlashMessage | null;
	error?: string;
	// AI result passed back from server after Ask AI submission
	ai_result?: {
		question: string;
		query: string;
		results: Record<string, unknown>[];
		queryType: string;
		vegaSpec?: string;
		summary?: string;
		followUps?: string[];
		websiteId: number;
	} | null;
	[key: string]: unknown;
}

// Infinity Icon Component for Ask page (black)
const InfinityIcon: React.FC<{ className?: string }> = ({ className }) => (
	<svg
		viewBox="0 0 512 512"
		className={cn("w-6 h-6", className)}
		fill="currentColor"
		xmlns="http://www.w3.org/2000/svg"
	>
		<path
			d="M501.98,206.15c-9.769-23.023-25.998-42.56-46.458-56.389c-10.181-6.873-21.446-12.34-33.536-16.068c-12.009-3.809-24.842-5.798-38.088-5.798c-16.982,0-33.294,3.316-48.197,9.365c-1.246,0.492-2.402,0.986-3.558,1.568c-13.416,5.879-25.675,14.16-36.188,24.017c-3.396,3.227-6.623,6.623-9.858,10.432c-5.709,6.542-11.588,14.079-17.305,21.696c-1.157,1.568-2.402,3.226-3.558,4.804c-3.146,4.302-33.212,48.358-38.509,56.226c-2.652,3.97-5.798,8.442-9.195,13.327c-0.744,1.076-1.568,2.24-2.393,3.396c-5.636,8.031-11.928,16.481-17.726,23.937c-2.895,3.72-5.798,7.197-8.281,10.1c-2.563,2.976-4.884,5.378-6.542,6.954c-7.116,6.704-15.486,12.171-24.672,15.899c-9.194,3.728-19.214,5.798-29.816,5.798c-7.286,0-14.322-0.996-20.944-2.815c-3.396-0.913-6.712-2.07-9.939-3.477c-14.248-5.968-26.419-16.068-34.95-28.74c-4.302-6.372-7.699-13.327-10.019-20.783c-2.233-7.456-3.558-15.316-3.558-23.597c0-11.014,2.24-21.365,6.21-30.892c6.049-14.24,16.149-26.329,28.821-34.942c6.372-4.31,13.326-7.618,20.782-9.939c7.448-2.321,15.316-3.638,23.597-3.638c10.602,0.08,20.622,2.07,29.816,5.79c9.187,3.808,17.556,9.194,24.672,15.898c1.658,1.577,3.979,4.059,6.542,6.962c4.472,5.216,9.769,11.92,15.074,18.964c2.07,2.814,4.14,5.628,6.21,8.523c7.949-11.588,21.858-31.959,29.144-42.48c-1.237-1.658-2.482-3.307-3.72-4.965c-3.316-4.23-6.631-8.281-9.938-12.009c-3.316-3.809-6.462-7.205-9.858-10.432c-11.426-10.772-24.922-19.545-39.746-25.586c-14.904-6.049-31.222-9.365-48.196-9.365c-17.637,0-34.53,3.566-49.927,10.108c-23.022,9.688-42.487,25.918-56.316,46.369c-6.873,10.19-12.332,21.527-16.141,33.536C1.989,229.997,0,242.75,0,256.004c0,17.637,3.558,34.53,10.02,49.846c9.768,23.104,25.998,42.569,46.369,56.397c10.27,6.874,21.535,12.332,33.624,16.141c12.008,3.728,24.842,5.717,38.088,5.717c16.974,0,33.293-3.316,48.196-9.356c14.824-6.049,28.239-14.824,39.666-25.506l0.08-0.081c3.397-3.146,6.543-6.631,9.858-10.44c5.709-6.542,11.588-14.071,17.305-21.689c1.157-1.577,2.402-3.154,3.558-4.723c3.146-4.391,44.307-64.758,47.696-69.642c0.752-1.076,1.577-2.232,2.401-3.396c5.637-7.95,11.928-16.48,17.726-23.928c2.895-3.728,5.798-7.206,8.281-10.101c2.564-2.984,4.885-5.386,6.542-6.962c7.116-6.704,15.486-12.09,24.673-15.898c2.24-0.906,4.472-1.649,6.792-2.402c7.286-2.15,14.984-3.307,23.023-3.388c11.013,0.08,21.446,2.232,30.882,6.291c14.241,5.96,26.42,16.06,34.943,28.732c4.31,6.38,7.706,13.335,10.019,20.782c2.321,7.456,3.566,15.324,3.566,23.605c0,11.014-2.24,21.446-6.21,30.883c-6.049,14.24-16.149,26.419-28.821,34.942c-6.372,4.31-13.326,7.707-20.782,9.939c-7.367,2.321-15.316,3.648-23.597,3.648c-10.602,0-20.622-2.07-29.816-5.798c-9.187-3.728-17.557-9.195-24.673-15.899c-1.658-1.577-3.979-4.059-6.542-6.954c-4.472-5.135-9.776-11.928-15.074-18.963c-2.15-2.815-4.221-5.718-6.291-8.613c-0.663,0.994-1.326,1.99-2.07,3.065c-13.666,20.039-22.279,32.71-26.994,39.576c1.237,1.658,2.483,3.235,3.72,4.893c3.316,4.221,6.631,8.281,9.938,12c3.234,3.808,6.462,7.294,9.858,10.44c11.426,10.763,24.923,19.538,39.746,25.587c14.904,6.04,31.215,9.356,48.197,9.356c17.636,0,34.53-3.558,49.846-10.019c23.103-9.769,42.56-25.999,56.396-46.458c6.866-10.181,12.421-21.446,16.141-33.536C510.01,282.083,512,269.25,512,256.004C512,238.367,508.442,221.474,501.98,206.15z"
		/>
	</svg>
);

interface Website {
	id: number;
	domain: string;
	created_at: string;
}

interface SavedQuery {
	id: number;
	title: string;
	generated_sql: string;
	query_type: string;
	vega_spec?: string;
	model?: string;
	website_id?: number;
	created_at: string;
	updated_at: string;
}

interface QueryResult {
	id: number;
	results?: Record<string, unknown>[];
	error?: string;
	query_type?: string;
	vega_spec?: string;
}

interface ResultItem {
	[key: string]: string | number | null;
}

// Dynamic color function
const getDynamicColor = (index: number): string => {
	const colors = ["#00D1FF", "#00D678", "#FF7733"];
	return colors[index % colors.length];
};

export const Lens: React.FC = () => {
	const { props } = usePage<LensPageProps>();
	const websiteId = props.current_website_id || 0;
	const websiteDomain = props.website_domain || "";
	const lensBaseUrl = `/admin/websites/${websiteId}/lens`;
	const aiConfigured = props.ai_configured ?? false;

	// Data comes from server props (Inertia SSR)
	const savedQueries = props.saved_queries || [];
	const websites = props.websites || [];

	// Build query results map from initial_results prop
	const queryResults = useMemo(() => {
		const resultsMap = new Map<number, QueryResult>();
		if (props.initial_results) {
			props.initial_results.forEach((result) => {
				resultsMap.set(result.id, {
					id: result.id,
					results: result.results,
					error: result.error,
					query_type: result.query_type,
					vega_spec: result.vega_spec,
				});
			});
		}
		return resultsMap;
	}, [props.initial_results]);

	// UI state
	const [refreshing, setRefreshing] = useState(false);
	const [editingId, setEditingId] = useState<number | null>(null);
	const editInputRef = useRef<HTMLInputElement>(null);

	// Loading states for actions using router.post() directly
	const [deletingId, setDeletingId] = useState<number | null>(null);
	const [cloningId, setCloningId] = useState<number | null>(null);
	const [isAsking, setIsAsking] = useState(false);
	const [isSaving, setIsSaving] = useState(false);

	// Forms using Inertia useForm hook
	const editForm = useForm({
		id: 0,
		title: "",
		model: "gpt-5.2",
		website_id: websiteId > 0 ? websiteId : null as number | null,
	});

	// AI Chat state - use server-provided result if available
	const [aiResult, setAiResult] = useState<{
		question: string;
		query: string;
		results: Record<string, unknown>[];
		queryType: string;
		vegaSpec?: string;
		summary?: string;
		websiteId: number;
	} | null>(props.ai_result || null);

	// External question state for passing to AskAIChat
	const [pendingQuestion, setPendingQuestion] = useState<string | undefined>(undefined);
	// Signal to clear the input field after saving
	const [shouldClearInput, setShouldClearInput] = useState(false);
	// Auto-submit state for URL-based questions
	const [shouldAutoSubmit, setShouldAutoSubmit] = useState(false);
	// Model selection state
	const [selectedModel, setSelectedModel] = useState("gpt-5.2");

	// Read question from URL parameter on mount (for Insights → Lens flow)
	useEffect(() => {
		const params = new URLSearchParams(window.location.search);
		const q = params.get('q');
		if (q) {
			setPendingQuestion(q);
			setShouldAutoSubmit(true);
			// Clean URL without reload
			window.history.replaceState({}, '', window.location.pathname);
		}
	}, []);

	// Auto-submit the question when coming from Insights
	useEffect(() => {
		if (shouldAutoSubmit && pendingQuestion && !isAsking) {
			setShouldAutoSubmit(false);
			handleAISubmit(pendingQuestion, websiteId, selectedModel);
		}
	}, [shouldAutoSubmit, pendingQuestion, isAsking, websiteId, selectedModel]);

	// Refresh all queries using Inertia reload
	const handleRefresh = () => {
		setRefreshing(true);
		router.reload({
			only: ['saved_queries', 'initial_results'],
			onFinish: () => setRefreshing(false),
		});
	};

	const handleEditStart = (query: SavedQuery) => {
		setEditingId(query.id);
		editForm.setData({
			id: query.id,
			title: query.title,
			model: query.model || "gpt-5.2",
			website_id: websiteId > 0 ? websiteId : null,
		});
		setTimeout(() => editInputRef.current?.focus(), 0);
	};

	const handleEditSave = () => {
		if (!editForm.data.title.trim()) return;

		// Use editForm.post for proper Inertia form handling with loading state
		editForm.post(`${lensBaseUrl}/update`, {
			preserveScroll: true,
			forceFormData: true,
			onSuccess: () => {
				handleEditCancel();
			},
		});
	};

	const handleEditCancel = () => {
		setEditingId(null);
		editForm.reset();
		editForm.clearErrors();
	};

	// Handle AI question submission using Inertia router
	const handleAISubmit = (question: string, targetWebsiteId: number, model?: string) => {
		setAiResult(null); // Clear previous result

		// Create FormData to send as form-encoded data (required by the server)
		const formData = new FormData();
		formData.append("query", question);
		formData.append("model", model || selectedModel);
		if (targetWebsiteId > 0) {
			formData.append("website_id", targetWebsiteId.toString());
		}

		setIsAsking(true);
		router.post(`${lensBaseUrl}/ask-ai`, formData, {
			preserveScroll: true,
			preserveUrl: true,
			forceFormData: true,
			onSuccess: (page) => {
				const newProps = page.props as LensPageProps;
				if (newProps.ai_result) {
					setAiResult(newProps.ai_result);
				}
			},
			onFinish: () => {
				setIsAsking(false);
			},
		});
	};

	// Save AI result to saved questions using Inertia router
	const handleSaveAIResult = () => {
		if (!aiResult) return;

		// Create FormData to send as form-encoded data (required by the server)
		const formData = new FormData();
		formData.append("title", aiResult.question);
		formData.append("generated_sql", aiResult.query);
		formData.append("query_type", aiResult.queryType);
		formData.append("vega_spec", aiResult.vegaSpec || "");
		formData.append("model", selectedModel);
		if (aiResult.websiteId) {
			formData.append("website_id", aiResult.websiteId.toString());
		}

		setIsSaving(true);
		router.post(`${lensBaseUrl}/save`, formData, {
			preserveScroll: true,
			forceFormData: true,
			onSuccess: () => {
				setAiResult(null);
				// Clear the input field after saving
				setShouldClearInput(true);

				// Smooth scroll to top of saved questions section
				setTimeout(() => {
					const savedQueriesSection = document.querySelector('[data-saved-queries]');
					if (savedQueriesSection) {
						const elementPosition = savedQueriesSection.getBoundingClientRect().top;
						const offsetPosition = elementPosition + window.pageYOffset - 80;
						window.scrollTo({ top: offsetPosition, behavior: 'smooth' });
					}
				}, 150);
			},
			onFinish: () => {
				setIsSaving(false);
			},
		});
	};

	const handleDelete = (queryId: number) => {
		if (!confirm("Are you sure you want to delete this query?")) return;

		// Create FormData to send as form-encoded data (required by the server)
		const formData = new FormData();
		formData.append("id", queryId.toString());

		setDeletingId(queryId);
		router.post(`${lensBaseUrl}/delete`, formData, {
			preserveScroll: true,
			forceFormData: true,
			onFinish: () => {
				setDeletingId(null);
			},
		});
	};

	const handleClone = (queryId: number) => {
		// Create FormData to send as form-encoded data (required by the server)
		const formData = new FormData();
		formData.append("id", queryId.toString());

		setCloningId(queryId);
		router.post(`${lensBaseUrl}/clone`, formData, {
			preserveScroll: true,
			forceFormData: true,
			onFinish: () => {
				setCloningId(null);
			},
		});
	};

	const renderResults = useCallback((queryResult: QueryResult) => {
		if (queryResult.error) {
			return (
				<Alert variant="destructive">
					<AlertDescription>{queryResult.error}</AlertDescription>
				</Alert>
			);
		}

		if (!queryResult.results || !Array.isArray(queryResult.results)) {
			return <p className="text-black/50 px-6 py-4">No data available.</p>;
		}

		const data = queryResult.results as ResultItem[];
		const queryType = queryResult.query_type || "TABLE";

		// Try to use Vega-Lite spec if available
		if (queryResult.vega_spec) {
			try {
				const spec = JSON.parse(queryResult.vega_spec);
				return (
					<div className="w-full overflow-hidden px-4">
						<div className="w-full max-w-full">
							<VegaChart
								spec={spec}
								data={data}
								className="w-full max-w-full"
							/>
						</div>
					</div>
				);
			} catch (err) {
				console.error("Failed to parse Vega spec, falling back to Recharts:", err);
				// Fall through to Recharts rendering
			}
		}

		// Fallback to Recharts rendering
		switch (queryType) {
			case "SCALAR":
				return renderScalarValue(data);
			case "TIMESERIES":
				return renderTimeSeriesChart(data);
			case "DISTRIBUTION":
				return renderDistributionChart(data);
			default:
				return renderTable(data);
		}
	}, []);

	const renderScalarValue = (data: ResultItem[]) => {
		if (!data || data.length === 0) return <p className="text-black">No data available.</p>;
		const [key, value] = Object.entries(data[0])[0];
		return (
			<div className="py-6 text-center">
				<div className="text-3xl font-bold">
					{value === 0 ? "0" : (value ?? "N/A")}
				</div>
				<div className="text-sm text-black mt-1">{key}</div>
			</div>
		);
	};

	const renderTimeSeriesChart = (data: ResultItem[]) => {
		if (!data?.length || !("date" in data[0])) return renderTable(data);

		const numericKeys = Object.keys(data[0]).filter(
			(key) => key !== "date" && !Number.isNaN(Number(data[0][key])) && data[0][key] !== null
		);

		if (!numericKeys.length) return renderTable(data);

		return (
			<div className="h-[300px]">
				<ResponsiveContainer width="100%" height="100%">
					<BarChart data={data} margin={{ top: 20, right: 20, bottom: 60, left: 20 }}>
						<CartesianGrid strokeDasharray="3 3" opacity={0.4} />
						<XAxis
							dataKey="date"
							tick={{ fill: "#374151", fontSize: 10, textAnchor: "end" }}
							angle={-45}
							dy={20}
						/>
						<YAxis
							tick={{ fill: "#374151", fontSize: 10 }}
							allowDecimals={false}
						/>
						<RechartsTooltip
							contentStyle={{
								backgroundColor: "#FFFFFF",
								border: "1px solid #E5E7EB",
								borderRadius: "6px",
								boxShadow: "0px 4px 12px rgba(0, 0, 0, 0.1)",
							}}
							formatter={(value: number) => formatNumber(value)}
						/>
						<Legend />
						{numericKeys.map((key, index) => (
							<Bar
								key={key}
								dataKey={key}
								fill={getDynamicColor(index)}
								name={key.replace(/_/g, " ").charAt(0).toUpperCase() + key.replace(/_/g, " ").slice(1)}
							/>
						))}
					</BarChart>
				</ResponsiveContainer>
			</div>
		);
	};

	const renderDistributionChart = (data: ResultItem[]) => {
		if (!data?.length) return renderTable(data);

		const nameKey = Object.keys(data[0]).find((key) => typeof data[0][key] === "string") || Object.keys(data[0])[0];
		const valueKey = Object.keys(data[0]).find((key) => !Number.isNaN(Number(data[0][key]))) || "count";

		return (
			<div className="h-[300px]">
				<ResponsiveContainer width="100%" height="100%">
					<BarChart data={data} margin={{ top: 20, right: 20, bottom: 60, left: 20 }}>
						<CartesianGrid strokeDasharray="3 3" opacity={0.4} />
						<XAxis
							dataKey={nameKey}
							tick={{ fill: "#374151", fontSize: 10, textAnchor: "end" }}
							angle={-45}
							dy={20}
						/>
						<YAxis
							tick={{ fill: "#374151", fontSize: 10 }}
							allowDecimals={false}
						/>
						<RechartsTooltip
							contentStyle={{
								backgroundColor: "#FFFFFF",
								border: "1px solid #E5E7EB",
								borderRadius: "6px",
								boxShadow: "0px 4px 12px rgba(0, 0, 0, 0.1)",
							}}
							formatter={(value: number) => formatNumber(value)}
						/>
						<Bar dataKey={valueKey} fill="#00D678" />
					</BarChart>
				</ResponsiveContainer>
			</div>
		);
	};

	const renderTable = (data: ResultItem[]) => {
		if (!data?.length) return <p className="text-black">No data available.</p>;

		const allKeys = [...new Set(data.flatMap((item) => Object.keys(item)))];

		// Filter out internal/technical columns (e.g., day_of_week_num when day_of_week exists)
		const visibleKeys = allKeys.filter((key) => {
			// Hide columns ending in _num or _id if there's a corresponding column without suffix
			if (key.endsWith('_num') || key.endsWith('_id')) {
				const baseKey = key.replace(/_(num|id)$/, '');
				if (allKeys.includes(baseKey)) return false;
			}
			return true;
		});

		const numericKeys = visibleKeys.filter(
			(key) => !Number.isNaN(Number(data[0][key])) && data[0][key] !== null
		);
		// Put text columns first, then numeric columns
		const textKeys = visibleKeys.filter((key) => !numericKeys.includes(key));
		const keys = [...textKeys, ...numericKeys];

		return (
			<div className="overflow-x-auto p-4">
				<table className="w-full border-collapse">
					<thead>
						<tr className="border-b border-black/10 bg-black/5">
							{keys.map((key) => (
								<th
									key={key}
									className={`py-2 px-3 text-left text-xs font-medium text-black/60 uppercase tracking-wider ${numericKeys.includes(key) ? "text-right" : ""
										}`}
								>
									{key.replace(/_/g, " ").charAt(0).toUpperCase() + key.replace(/_/g, " ").slice(1)}
								</th>
							))}
						</tr>
					</thead>
					<tbody className="bg-white divide-y divide-black/10">
						{data.map((item, rowIndex) => (
							<tr key={rowIndex} className="hover:bg-black/5">
								{keys.map((key) => (
									<td
										key={`${rowIndex}-${key}`}
										className={`py-2 px-3 text-sm ${numericKeys.includes(key) ? "text-right" : ""
											}`}
									>
										{numericKeys.includes(key)
											? (item[key] != null && !isNaN(Number(item[key])) ? formatNumber(Number(item[key])) : "0")
											: (item[key] ?? "")}
									</td>
								))}
							</tr>
						))}
					</tbody>
				</table>
			</div>
		);
	};

	return (
		<WebsiteLayout
			websiteId={websiteId}
			websiteDomain={websiteDomain}
			currentPath={lensBaseUrl}
			websites={websites}
		>
			<div className="py-4">
				{/* Flash messages for errors/success */}
				<FlashMessageDisplay flash={props.flash} error={props.error} />

				<div className="flex flex-col gap-4">
				{/* Header */}
				<div className="flex items-center gap-2">
					<InfinityIcon className="h-6 w-6" />
					<h1 className="text-2xl font-bold text-black">Ask</h1>
				</div>
				<p className="text-black">Go beyond the dashboard. Ask questions in plain English.</p>

				{!aiConfigured ? (
					/* No-key empty state: prompt to add an OpenAI key */
					<Card className="border-black shadow-sm">
						<CardHeader className="text-center pb-2">
							<div className="mx-auto mb-4 w-16 h-16 bg-black rounded-2xl flex items-center justify-center">
								<Key className="w-8 h-8 text-white" />
							</div>
							<CardTitle className="text-2xl">
								Add your OpenAI key to get started
							</CardTitle>
							<CardDescription className="text-base">
								Ask connects to OpenAI to turn your questions into answers.
								Add your API key in AI settings to enable it.
							</CardDescription>
						</CardHeader>
						<CardContent className="pt-4 flex flex-col items-center gap-3">
							<Button
								asChild
								size="lg"
								className="bg-black hover:bg-black/80 text-white gap-2"
							>
								<Link href="/admin/administration/ai">
									<Key className="w-4 h-4" />
									Add your OpenAI key
								</Link>
							</Button>
						</CardContent>
					</Card>
				) : (
				<>
				{/* Ask AI Chat Component with integrated results */}
				<AskAIChat
					onSubmit={handleAISubmit}
					isLoading={isAsking}
					websiteId={websiteId}
					selectedModel={selectedModel}
					onModelChange={setSelectedModel}
					aiResult={aiResult}
					onSaveResult={handleSaveAIResult}
					isSavingResult={isSaving}
					externalQuestion={pendingQuestion}
					onExternalQuestionConsumed={() => setPendingQuestion(undefined)}
					shouldClearInput={shouldClearInput}
					onInputCleared={() => setShouldClearInput(false)}
					renderResults={(results: Record<string, unknown>[], queryType: string, vegaSpec?: string) => {
						if (!results || results.length === 0) {
							return (
								<div className="text-center py-8 text-black/60">
									No results found
								</div>
							);
						}

						const data = results as ResultItem[];

						// Use Vega-Lite spec if available
						if (vegaSpec) {
							try {
								const spec = typeof vegaSpec === 'string' ? JSON.parse(vegaSpec) : vegaSpec;
								return (
									<div className="w-full overflow-hidden p-4">
										<VegaChart
											spec={spec}
											data={data}
											className="w-full max-w-full"
										/>
									</div>
								);
							} catch (err) {
								console.error("Failed to parse Vega spec:", err);
							}
						}

						// Fallback based on queryType
						switch (queryType) {
							case "SCALAR":
								return (
									<div className="text-center py-8">
										<div className="text-5xl font-bold text-black">
											{(() => {
												const val = Object.values(results[0] as Record<string, unknown>)[0];
												const num = Number(val);
												return val != null && !isNaN(num) ? formatNumber(num) : "0";
											})()}
										</div>
										<div className="text-sm text-black/60 mt-2">
											{Object.keys(results[0] as Record<string, unknown>)[0]}
										</div>
									</div>
								);
							case "TIMESERIES":
								return renderTimeSeriesChart(data);
							case "DISTRIBUTION":
								return renderDistributionChart(data);
							default:
								return renderTable(data);
						}
					}}
				/>

				{/* Saved Questions List */}
				{savedQueries.length > 0 && (
					<div className="w-full mt-8" data-saved-queries>
						<div className="mb-6 flex items-center justify-between">
							<h2 className="text-sm font-medium text-black/70 uppercase tracking-wide">
								Saved Questions ({savedQueries.length})
							</h2>
							<TooltipProvider>
								<Tooltip>
									<TooltipTrigger asChild>
										<Button
											variant="ghost"
											size="sm"
											onClick={handleRefresh}
											disabled={refreshing}
											className="text-black/60 hover:text-black hover:bg-black/5"
										>
											<RefreshCw className={cn("w-4 h-4", refreshing && "animate-spin")} />
										</Button>
									</TooltipTrigger>
									<TooltipContent>
										<p>Refresh all queries</p>
									</TooltipContent>
								</Tooltip>
							</TooltipProvider>
						</div>
						<div className="space-y-6">
							{savedQueries.map((query) => {
								const result = queryResults.get(query.id);
								const isExecuting = refreshing && !result;

								return (
									<div
										key={query.id}
										data-testid="saved-query-card"
										className="bg-white border border-black rounded-lg overflow-hidden"
									>
										{/* Query Header */}
										<div className="px-6 py-4 border-b border-black/10 flex items-center justify-between">
											<div className="flex-1">
												<h3 className="text-base font-medium text-black mb-1">
													{query.title}
												</h3>
												<div className="flex items-center gap-3 text-xs text-black/60">
													<span>{query.model === "gpt-4.1" ? "Fast" : query.model === "gpt-5.2-thinking" ? "Deep" : "Smart"}</span>
													<span>•</span>
													<span>{new Date(query.created_at).toLocaleDateString()}</span>
													{query.vega_spec && (
														<>
															<span>•</span>
															<span className="text-black font-medium">Chart</span>
														</>
													)}
												</div>
											</div>
											<div className="flex items-center gap-2">
												<TooltipProvider>
													<Tooltip>
														<TooltipTrigger asChild>
															<Button
																aria-label="Edit query"
																size="sm"
																variant="ghost"
																onClick={() => handleEditStart(query)}
																className="text-black/50 hover:text-black hover:bg-black/5"
															>
																<Edit3 className="w-4 h-4" />
															</Button>
														</TooltipTrigger>
														<TooltipContent>
															<p>Edit question</p>
														</TooltipContent>
													</Tooltip>
												</TooltipProvider>
												<TooltipProvider>
													<Tooltip>
														<TooltipTrigger asChild>
															<Button
																aria-label="Clone query"
																size="sm"
																variant="ghost"
																onClick={() => handleClone(query.id)}
																disabled={cloningId === query.id || deletingId === query.id}
																className="text-black/50 hover:text-black hover:bg-black/5"
															>
																{cloningId === query.id ? (
																	<Loader2 className="w-4 h-4 animate-spin" />
																) : (
																	<Copy className="w-4 h-4" />
																)}
															</Button>
														</TooltipTrigger>
														<TooltipContent>
															<p>{cloningId === query.id ? "Cloning..." : "Clone question"}</p>
														</TooltipContent>
													</Tooltip>
												</TooltipProvider>
												<TooltipProvider>
													<Tooltip>
														<TooltipTrigger asChild>
															<Button
																aria-label="Delete query"
																size="sm"
																variant="ghost"
																onClick={() => handleDelete(query.id)}
																disabled={deletingId === query.id || cloningId === query.id}
																className="text-black/60 hover:text-red-600 hover:bg-red-50"
															>
																{deletingId === query.id ? (
																	<Loader2 className="w-4 h-4 animate-spin" />
																) : (
																	<Trash2 className="w-4 h-4" />
																)}
															</Button>
														</TooltipTrigger>
														<TooltipContent>
															<p>{deletingId === query.id ? "Deleting..." : "Delete question"}</p>
														</TooltipContent>
													</Tooltip>
												</TooltipProvider>
											</div>
										</div>

										{/* SQL Preview (collapsible) */}
										<div className="px-6 py-3 border-b border-black/10">
											<details className="group">
												<summary className="cursor-pointer text-xs text-black/50 hover:text-black flex items-center font-medium outline-none">
													<svg className="w-3 h-3 mr-1.5 group-open:rotate-90 transition-transform" fill="none" stroke="currentColor" viewBox="0 0 24 24">
														<path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
													</svg>
													View SQL Query
												</summary>
												<div className="mt-3 p-3 bg-black/5 rounded-lg font-mono text-xs leading-relaxed whitespace-pre-wrap text-black/70">
													{query.generated_sql}
												</div>
											</details>
										</div>

										{/* Edit Mode */}
										{editingId === query.id && (
											<div className="px-6 py-4 border-t border-black/10" data-testid="saved-query-edit">
												<div className="space-y-3">
													<div>
														<label className="block text-xs font-medium text-black/70 mb-2">
															Edit Question
														</label>
														<input
															ref={editInputRef}
															type="text"
															value={editForm.data.title}
															onChange={(e) => editForm.setData('title', e.target.value)}
															onKeyDown={(e) => {
																if (e.key === "Enter" && !editForm.processing) {
																	handleEditSave();
																} else if (e.key === "Escape" && !editForm.processing) {
																	handleEditCancel();
																}
															}}
															disabled={editForm.processing}
															className="w-full border border-black/20 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-black focus:ring-1 focus:ring-black disabled:bg-black/5 disabled:cursor-not-allowed"
															placeholder="Enter your question"
														/>
													</div>
													{/* Model selector */}
													<div>
														<label className="block text-xs font-medium text-black/70 mb-2">
															Model
														</label>
														<ModelSelector
															value={editForm.data.model}
															onChange={(m) => editForm.setData('model', m)}
															disabled={editForm.processing}
														/>
													</div>
													<p className="text-xs text-black/60">
														Saving will regenerate the SQL using AI
													</p>
													{/* Error message from form */}
													{editForm.errors.title && (
														<div className="p-3 bg-red-50 border border-red-100 rounded-lg text-red-700 text-sm">
															{editForm.errors.title}
														</div>
													)}

													<div className="flex items-center gap-2">
														<Button
															size="sm"
															onClick={() => handleEditSave()}
															disabled={editForm.processing || !editForm.data.title.trim()}
															className="bg-black text-white hover:bg-black/80 disabled:bg-black/20 disabled:cursor-not-allowed text-sm"
														>
															{editForm.processing ? (
																<div className="flex items-center">
																	<LoadingPulse size="sm" color="green" className="mr-2" />
																	Saving...
																</div>
															) : (
																"Save Changes"
															)}
														</Button>
														<Button
															size="sm"
															variant="ghost"
															onClick={handleEditCancel}
															disabled={editForm.processing}
															className="text-black/60 hover:text-black hover:bg-black/5 disabled:text-black/30 disabled:cursor-not-allowed text-sm"
														>
															Cancel
														</Button>
													</div>
												</div>
											</div>
										)}

										{/* Query Results */}
										<div className="bg-white" data-testid="saved-query-results">
											{isExecuting ? (
												<div className="text-center py-8">
													<div className="animate-spin rounded-full h-6 w-6 border-b-2 border-black mx-auto"></div>
													<p className="text-black/60 mt-2 text-sm">Executing query...</p>
												</div>
											) : result ? (
												renderResults(result)
											) : (
												<div className="text-center py-8">
													<p className="text-black/60 text-sm">Click "Refresh All" to execute this query</p>
												</div>
											)}
										</div>
									</div>
								);
							})}
						</div>
					</div>
				)}
				</>
				)}
				</div>
			</div>
		</WebsiteLayout>
	);
};
