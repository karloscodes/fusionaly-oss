import { useState } from "react";
import { usePage, Link } from "@inertiajs/react";
import { Button } from "@/components/ui/button";
import { Check, Copy, ArrowRight, Code, BookOpen, Zap } from "lucide-react";
import { AdminLayout } from "@/components/admin-layout";

interface WebsiteSetupProps {
	website: {
		id: number;
		domain: string;
	};
	flash?: any;
	error?: string;
	[key: string]: any;
}

export function WebsiteSetup() {
	const { props } = usePage<WebsiteSetupProps>();
	const { website } = props;
	const [copied, setCopied] = useState(false);

	const scriptTag = `<script defer src="${window.location.origin}/y/api/v1/sdk.js" data-website-id="${website.id}"></script>`;

	const handleCopy = async () => {
		try {
			await navigator.clipboard.writeText(scriptTag);
			setCopied(true);
			setTimeout(() => setCopied(false), 2000);
		} catch (err) {
			console.error("Failed to copy:", err);
		}
	};

	return (
		<AdminLayout>
			<div className="py-12">
				<div className="max-w-2xl mx-auto">
					{/* Success Header */}
					<div className="text-center mb-10">
						<div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-6">
							<Check className="h-8 w-8 text-green-600" />
						</div>
						<h1 className="text-3xl font-bold text-black mb-3">
							Website Created!
						</h1>
						<p className="text-lg text-black/60">
							<span className="font-semibold text-black">{website.domain}</span> is ready to start collecting analytics data.
						</p>
					</div>

					{/* Installation Card */}
					<div className="bg-white border border-black rounded-lg overflow-hidden mb-8">
						<div className="px-6 py-4 border-b border-black/10">
							<div className="flex items-center gap-2">
								<Code className="h-5 w-5 text-black" />
								<h2 className="font-semibold text-black">Install the Tracking Script</h2>
							</div>
						</div>
						<div className="p-6 space-y-4">
							<p className="text-black/70">
								Copy and paste this script into your website's HTML, preferably in the <code className="bg-black/5 px-1.5 py-0.5 rounded text-sm">&lt;head&gt;</code> section:
							</p>

							<div className="relative">
								<div className="bg-black p-4 rounded-lg font-mono text-sm overflow-x-auto">
									<code className="text-green-400 break-all">
										{scriptTag}
									</code>
								</div>
								<Button
									variant="outline"
									size="sm"
									onClick={handleCopy}
									className="absolute top-2 right-2 bg-black/80 border-black/70 text-white hover:bg-black/70 hover:text-white"
								>
									{copied ? (
										<>
											<Check className="h-4 w-4 mr-1" />
											Copied!
										</>
									) : (
										<>
											<Copy className="h-4 w-4 mr-1" />
											Copy
										</>
									)}
								</Button>
							</div>

							<div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
								<p className="text-sm text-blue-800">
									<strong>Tip:</strong> The script loads asynchronously with <code className="bg-blue-100 px-1 rounded">defer</code>, so it won't slow down your page load.
								</p>
							</div>
						</div>
					</div>

					{/* Additional Options */}
					<div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
						{/* Documentation Card */}
						<div className="bg-white border border-black/10 rounded-lg p-5 hover:border-black/40 transition-colors">
							<div className="flex items-start gap-3">
								<div className="p-2 bg-black/5 rounded-lg">
									<BookOpen className="h-5 w-5 text-black/70" />
								</div>
								<div>
									<h3 className="font-semibold text-black mb-1">Documentation</h3>
									<p className="text-sm text-black/60 mb-3">
										Learn about advanced tracking options, custom events, and more.
									</p>
									<a
										href="https://fusionaly.com/docs"
										target="_blank"
										rel="noopener noreferrer"
										className="text-sm font-medium text-black hover:underline inline-flex items-center gap-1"
									>
										Read the docs
										<ArrowRight className="h-3 w-3" />
									</a>
								</div>
							</div>
						</div>

						{/* Subdomain Tracking Card */}
						<div className="bg-white border border-black/10 rounded-lg p-5 hover:border-black/40 transition-colors">
							<div className="flex items-start gap-3">
								<div className="p-2 bg-black/5 rounded-lg">
									<Zap className="h-5 w-5 text-black/70" />
								</div>
								<div>
									<h3 className="font-semibold text-black mb-1">Subdomain Tracking</h3>
									<p className="text-sm text-black/60 mb-3">
										Track users across subdomains for a unified view.
									</p>
									<Link
										href={`/admin/websites/${website.id}/edit`}
										className="text-sm font-medium text-black hover:underline inline-flex items-center gap-1"
									>
										Configure settings
										<ArrowRight className="h-3 w-3" />
									</Link>
								</div>
							</div>
						</div>
					</div>

					{/* Actions */}
					<div className="flex flex-col sm:flex-row gap-3 justify-center">
						<Button
							asChild
							className="bg-black hover:bg-black/80 text-white"
						>
							<Link href={`/admin/websites/${website.id}/dashboard`}>
								Go to Dashboard
								<ArrowRight className="h-4 w-4 ml-2" />
							</Link>
						</Button>
						<Button
							asChild
							variant="outline"
							className="border-black/20"
						>
							<Link href="/admin">
								View All Websites
							</Link>
						</Button>
					</div>
				</div>
			</div>
		</AdminLayout>
	);
}

export default WebsiteSetup;
