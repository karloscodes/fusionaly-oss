import { useEffect } from "react";
import { Dashboard as DashboardComponent } from "@/components/dashboard";
import { WebsiteLayout } from "@/components/website-layout";
import { saveRecentWebsiteAccess } from "./Websites";

interface Website {
	id: number;
	domain: string;
}

interface DashboardProps extends Record<string, unknown> {
	current_website_id?: number;
	website_domain?: string;
	websites?: Website[];
}

// Dashboard page receives all analytics data as flat Inertia props
// and passes them directly to DashboardComponent
// Insights and comparison metrics are loaded via deferred props for faster initial render
// The DashboardComponent handles showing loading states for deferred data
const Dashboard = ({ current_website_id, website_domain, websites, ...props }: DashboardProps) => {
	const websiteId = current_website_id || 0;
	const websiteDomain = website_domain || "";

	// Track this website as recently accessed
	useEffect(() => {
		if (websiteId > 0) {
			saveRecentWebsiteAccess(websiteId);
		}
	}, [websiteId]);

	return (
		<WebsiteLayout
			websiteId={websiteId}
			websiteDomain={websiteDomain}
			currentPath={`/admin/websites/${websiteId}/dashboard`}
			websites={websites}
		>
			<DashboardComponent {...props} current_website_id={current_website_id} />
		</WebsiteLayout>
	);
};

export default Dashboard;
