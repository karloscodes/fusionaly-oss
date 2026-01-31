import { ReactNode, useState, useEffect } from "react";
import { Link, router } from "@inertiajs/react";
import { AlertTriangle } from "lucide-react";

interface AdminLayoutProps {
	children: ReactNode;
	currentPath?: string;
	badge?: ReactNode;
}

// Fusionaly Logo Component - Text wordmark with green underscore
const FusionalyLogo = () => (
	<span className="text-lg font-semibold font-mono">
		fusionaly<span className="text-[#00D678]">_</span>
	</span>
);

interface SystemHealth {
	healthy: boolean;
	warning: string;
}

export function AdminLayout({ children, currentPath, badge }: AdminLayoutProps) {
	const [health, setHealth] = useState<SystemHealth | null>(null);

	useEffect(() => {
		// Fetch system health status
		fetch("/admin/api/system/health")
			.then((res) => res.json())
			.then((data: SystemHealth) => setHealth(data))
			.catch(() => {
				// Silently fail - don't show warning if health check fails
			});
	}, []);

	const handleLogout = (e: React.MouseEvent<HTMLAnchorElement>) => {
		e.preventDefault();
		router.post("/logout");
	};

	// Check if current path matches (handle query params)
	const isCurrentPath = (path: string) => {
		if (!currentPath) return false;
		const currentWithoutQuery = currentPath.split("?")[0];
		return currentWithoutQuery === path || currentWithoutQuery.startsWith(path + "/");
	};

	return (
		<div className="min-h-screen bg-white">
			{/* Navigation Banner */}
			<nav className="border-b border-black/10">
				<div className="max-w-7xl mx-auto px-4">
					<div className="flex h-14 items-center justify-between">
						{/* Left side: Logo + App name */}
						<div className="flex items-center space-x-4">
							<Link
								href="/admin"
								className="flex items-center gap-2 text-black hover:text-black transition-colors"
							>
								<FusionalyLogo />
								{badge}
							</Link>
						</div>

						{/* Right side: Health warning + Settings + Logout */}
						<div className="flex items-center space-x-4">
							{/* System health warning indicator */}
							{health && !health.healthy && (
								<Link
									href="/admin/administration/system"
									className="flex items-center gap-1 text-amber-600 hover:text-amber-700 transition-colors"
									title={health.warning}
								>
									<AlertTriangle className="h-5 w-5" />
									<span className="text-sm font-medium hidden sm:inline">Issue</span>
								</Link>
							)}
							<Link
								href="/admin/administration/ingestion"
								className="relative text-sm font-medium transition-colors hover:text-black/60 py-4 text-black"
							>
								Settings
								{/* Active indicator - black underline */}
								{isCurrentPath("/admin/administration") && (
									<span className="absolute bottom-0 left-0 right-0 h-0.5 bg-black" />
								)}
							</Link>
							<a
								href="#"
								id="logout"
								onClick={handleLogout}
								className="text-sm font-medium transition-colors hover:text-black/60 text-black"
							>
								Logout
							</a>
						</div>
					</div>
				</div>
			</nav>

			{/* Main Content */}
			<main className="max-w-7xl mx-auto px-4">
				{children}
			</main>
		</div>
	);
}
