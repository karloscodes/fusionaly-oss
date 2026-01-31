import React from "react";
import { usePage } from "@inertiajs/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Sparkles, ExternalLink, MessageSquare, Activity, Save } from "lucide-react";
import { WebsiteLayout } from "@/components/website-layout";

interface LensPageProps {
	current_website_id?: number;
	website_domain?: string;
	websites?: { id: number; domain: string; created_at: string }[];
	[key: string]: unknown;
}

export const Lens: React.FC = () => {
	const { props } = usePage<LensPageProps>();
	const websiteId = props.current_website_id || 0;
	const websiteDomain = props.website_domain || "";
	const websites = props.websites || [];
	const lensBaseUrl = `/admin/websites/${websiteId}/lens`;

	const features = [
		{
			icon: MessageSquare,
			title: "Ask AI",
			description: "\"Why did traffic drop last Tuesday?\" Type a question, get an answer. No SQL, no exports, no guessing."
		},
		{
			icon: Activity,
			title: "Activity Feed",
			description: "Live updates when something actually happens. A signup, a sale, a traffic spike. Just the moments that matter."
		},
		{
			icon: Save,
			title: "Saved Questions",
			description: "Build your own reports. Save the questions you care about. Run them whenever you need them."
		}
	];

	return (
		<WebsiteLayout
			websiteId={websiteId}
			websiteDomain={websiteDomain}
			currentPath={lensBaseUrl}
			websites={websites}
		>
			<div className="py-8">
				<div className="max-w-2xl mx-auto">
					<Card className="border border-black/10 shadow-sm">
						<CardHeader className="text-center pb-2">
							<div className="mx-auto mb-4 w-16 h-16 bg-[#238636] rounded-2xl flex items-center justify-center">
								<Sparkles className="w-8 h-8 text-white" />
							</div>
							<CardTitle className="text-2xl">
								Ask questions. Get answers.
							</CardTitle>
							<CardDescription className="text-base">
								Three tools to dig deeper into your analytics.
							</CardDescription>
						</CardHeader>
						<CardContent className="space-y-6">
							<div className="grid gap-4">
								{features.map((feature, index) => (
									<div key={index} className="flex gap-3 p-3 bg-white rounded-lg border border-black/10">
										<div className="shrink-0">
											<feature.icon className="w-5 h-5 text-black" />
										</div>
										<div>
											<h3 className="font-medium text-black">{feature.title}</h3>
											<p className="text-sm text-black/60">{feature.description}</p>
										</div>
									</div>
								))}
							</div>

							<div className="pt-4 flex flex-col items-center gap-3">
								<Button
									asChild
									size="lg"
									className="bg-black hover:bg-black/80 text-white gap-2"
								>
									<a
										href="https://fusionaly.com/#pricing"
										target="_blank"
										rel="noopener noreferrer"
									>
										<Sparkles className="w-4 h-4" />
										Upgrade to Pro
										<ExternalLink className="w-4 h-4" />
									</a>
								</Button>
							</div>
						</CardContent>
					</Card>
				</div>
			</div>
		</WebsiteLayout>
	);
};
