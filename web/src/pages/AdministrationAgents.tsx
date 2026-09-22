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
import { Input } from "@/components/ui/input";
import {
	Key,
	Copy,
	Check,
	RefreshCw,
	Plug,
} from "lucide-react";
import type { FlashMessage } from "@/types";
import { AdministrationLayout } from "@/components/administration-layout";

interface AdministrationAgentsProps {
	flash?: FlashMessage;
	error?: string;
	agent_api_key?: string;
	agent_api_key_exists?: boolean;
	[key: string]: unknown;
}

// Exported for Pro to wrap with its own layout
export const AdministrationAgentsContent: FC = () => {
	const { props } = usePage<AdministrationAgentsProps>();
	const { flash, error, agent_api_key, agent_api_key_exists } = props;
	const [localFlash, setLocalFlash] = useState<FlashMessage | null>(null);
	const [apiKeyCopied, setApiKeyCopied] = useState(false);
	const [apiKeyLoading, setApiKeyLoading] = useState(false);
	const [fullApiKey, setFullApiKey] = useState<string | null>(null);

	const handleGetApiKey = async () => {
		setApiKeyLoading(true);
		try {
			const response = await fetch("/admin/api/agent-api-key");
			if (response.ok) {
				const data = await response.json();
				setFullApiKey(data.api_key);
			} else {
				setLocalFlash({
					type: "error",
					message: "Failed to retrieve API key",
				});
				setTimeout(() => setLocalFlash(null), 5000);
			}
		} catch {
			setLocalFlash({
				type: "error",
				message: "Failed to retrieve API key",
			});
			setTimeout(() => setLocalFlash(null), 5000);
		} finally {
			setApiKeyLoading(false);
		}
	};

	const handleCopyApiKey = async () => {
		if (!fullApiKey) return;

		try {
			await navigator.clipboard.writeText(fullApiKey);
			setApiKeyCopied(true);
			setTimeout(() => setApiKeyCopied(false), 2000);
		} catch {
			setLocalFlash({
				type: "error",
				message: "Failed to copy to clipboard",
			});
			setTimeout(() => setLocalFlash(null), 3000);
		}
	};

	const handleRegenerateApiKey = () => {
		if (!window.confirm("Are you sure you want to regenerate the API key? The old key will stop working immediately.")) {
			return;
		}
		setFullApiKey(null);
		router.post("/admin/system/agent-api-key/regenerate", {}, {
			preserveScroll: true,
		});
	};

	const displayFlash = flash || localFlash;

	return (
		<div className="space-y-6">
			<div>
				<h1 className="text-2xl font-bold text-gray-900">Agent API</h1>
				<p className="text-gray-600 mt-1">
					Ask your analytics from Claude, Cursor, or any MCP client. Read-only.
				</p>
			</div>

			<FlashMessageDisplay flash={displayFlash} error={error} />

			{/* Agent API Key */}
			<Card className="border-black shadow-sm">
				<CardHeader className="pb-4">
					<div className="flex justify-between items-center">
						<CardTitle className="text-lg flex items-center gap-2">
							<Key className="h-5 w-5" /> API Key
						</CardTitle>
					</div>
					<CardDescription>
						API key for AI agents to query your analytics data via SQL.
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4">
					<div className="bg-gray-100 p-4 rounded-lg border border-gray-200">
						<p className="text-sm text-gray-700">
							Read-only API key for AI agents to query your analytics.{" "}
							<a
								href="https://fusionaly.com/docs/agent-api/"
								target="_blank"
								rel="noopener noreferrer"
								className="underline"
							>
								Setup instructions →
							</a>
						</p>
					</div>

					<div className="flex items-center gap-2">
						<Input
							type="text"
							value={fullApiKey || agent_api_key || "No API key generated"}
							readOnly
							className="font-mono text-sm flex-1"
						/>
						{agent_api_key_exists && !fullApiKey && (
							<Button
								onClick={handleGetApiKey}
								disabled={apiKeyLoading}
								variant="outline"
								size="sm"
								className="border-black text-black hover:bg-gray-100"
							>
								{apiKeyLoading ? "Loading..." : "Reveal"}
							</Button>
						)}
						{fullApiKey && (
							<Button
								onClick={handleCopyApiKey}
								variant="outline"
								size="sm"
								className="border-black text-black hover:bg-gray-100"
							>
								{apiKeyCopied ? (
									<Check className="h-4 w-4" />
								) : (
									<Copy className="h-4 w-4" />
								)}
							</Button>
						)}
					</div>

					<div className="flex gap-2">
						{!agent_api_key_exists && (
							<Button
								onClick={handleGetApiKey}
								disabled={apiKeyLoading}
								className="bg-black hover:bg-gray-800 text-white rounded-md"
							>
								{apiKeyLoading ? "Generating..." : "Generate API Key"}
							</Button>
						)}
						{agent_api_key_exists && (
							<Button
								onClick={handleRegenerateApiKey}
								variant="outline"
								className="border-black text-black hover:bg-gray-100"
							>
								<RefreshCw className="h-4 w-4 mr-2" />
								Regenerate
							</Button>
						)}
					</div>
				</CardContent>
			</Card>

			<ConnectCard />
		</div>
	);
};

// connectSteps are the install commands per AI client. They install the same
// plugin (skills + MCP) from the fusionaly-oss repo; see plugin/README.md.
const connectSteps = [
	{
		client: "Claude Code",
		note: "type these one at a time",
		commands: [
			"/plugin marketplace add karloscodes/fusionaly-oss",
			"/plugin install fusionaly",
			"/fusionaly:connect",
		],
	},
	{
		client: "Codex",
		note: "then ask Codex to \"connect Fusionaly\"",
		commands: [
			"codex plugin marketplace add karloscodes/fusionaly-oss",
			"codex plugin add fusionaly@fusionaly",
		],
	},
	{
		client: "Gemini CLI",
		note: "asks for the URL and key",
		commands: ["gemini extensions install https://github.com/karloscodes/fusionaly-oss"],
	},
];

// ConnectCard shows how to connect an AI client to this server's MCP endpoint.
const ConnectCard: FC = () => {
	const mcpURL = `${window.location.origin}/mcp`;

	return (
		<Card className="border-black shadow-sm">
			<CardHeader className="pb-4">
				<CardTitle className="text-lg flex items-center gap-2">
					<Plug className="h-5 w-5" /> Connect your AI client
				</CardTitle>
				<CardDescription>
					Your client asks this server; only the answer reaches your AI provider.
				</CardDescription>
			</CardHeader>
			<CardContent className="space-y-4 text-sm text-gray-700">
				<div className="space-y-1">
					<p className="font-medium text-gray-900">MCP server URL</p>
					<code className="block bg-gray-100 border border-gray-200 rounded px-3 py-2 font-mono break-all">{mcpURL}</code>
					<p>Send the API key above as <code className="font-mono">Authorization: Bearer &lt;key&gt;</code>.</p>
				</div>
				{connectSteps.map((step) => (
					<div key={step.client} className="space-y-1">
						<p className="font-medium text-gray-900">
							{step.client} <span className="font-normal text-gray-500">({step.note})</span>
						</p>
						{step.commands.map((command) => (
							<code key={command} className="block bg-gray-100 border border-gray-200 rounded px-3 py-2 font-mono break-all">
								{command}
							</code>
						))}
					</div>
				))}
				<p>
					Cursor, Claude Desktop, VS Code, and other clients:{" "}
					<a
						href="https://github.com/karloscodes/fusionaly-oss/tree/main/plugin#install"
						target="_blank"
						rel="noopener noreferrer"
						className="underline"
					>
						config for each client →
					</a>
				</p>
			</CardContent>
		</Card>
	);
};

// Default export wraps content with OSS layout
export const AdministrationAgents: FC = () => (
	<AdministrationLayout currentPage="agents">
		<AdministrationAgentsContent />
	</AdministrationLayout>
);
