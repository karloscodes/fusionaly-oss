import { ReactNode, useState, useEffect } from "react";
import { Link, router } from "@inertiajs/react";
import { AlertTriangle } from "lucide-react";
import { ThemeSwitcher } from "@/components/theme-switcher";
import { CommandSearch } from "@/components/command-search";
import { Byline } from "@/components/byline";

interface AdminLayoutProps {
	children: ReactNode;
	currentPath?: string;
	badge?: ReactNode;
}

// Fusionaly Logo Component - Text wordmark with green underscore
const FusionalyLogo = () => (
	<span className="text-lg font-semibold font-mono">
		fusionaly<span className="text-[rgb(var(--c-accent))]">_</span>
	</span>
);

interface SystemHealth {
	healthy: boolean;
	warning: string;
}

export function AdminLayout({ children, currentPath, badge }: AdminLayoutProps) {
	const [health, setHealth] = useState<SystemHealth | null>(null);

	useEffect(() => {
		fetch("/admin/api/system/health")
			.then((res) => res.json())
			.then((data: SystemHealth) => setHealth(data))
			.catch(() => {});
	}, []);

	const handleLogout = (e: React.MouseEvent<HTMLAnchorElement>) => {
		e.preventDefault();
		router.post("/logout");
	};

	const isCurrentPath = (path: string) => {
		if (!currentPath) return false;
		const currentWithoutQuery = currentPath.split("?")[0];
		return currentWithoutQuery === path || currentWithoutQuery.startsWith(path + "/");
	};

	return (
		<div className="min-h-screen bg-white">
			{/* Navigation Banner */}
			<nav className="border-b border-gray-200">
				<div className="max-w-7xl mx-auto px-4">
					<div className="flex h-14 items-center justify-between gap-2">
						<div className="flex items-center space-x-4 min-w-0 flex-1">
							<Link
								href="/admin"
								className="flex items-center gap-2 text-gray-900 hover:text-black transition-colors shrink-0"
							>
								<FusionalyLogo />
								{badge}
							</Link>
						</div>

						<div className="flex items-center space-x-2 sm:space-x-4 min-w-0 shrink">
							<CommandSearch />
							<ThemeSwitcher />
							{health && !health.healthy && (
								<Link
									href="/admin/administration/system"
									className="flex items-center gap-1 text-amber-600 hover:text-amber-700 transition-colors shrink-0"
									title={health.warning}
								>
									<AlertTriangle className="h-5 w-5" />
									<span className="text-sm font-medium hidden sm:inline">Issue</span>
								</Link>
							)}
							<Link
								href="/admin/administration/ingestion"
								className="relative text-sm font-medium transition-colors hover:text-gray-600 py-4 text-gray-900 shrink-0"
							>
								Settings
								{isCurrentPath("/admin/administration") && (
									<span className="absolute bottom-0 left-0 right-0 h-0.5 bg-black" />
								)}
							</Link>
							<a
								href="#"
								id="logout"
								onClick={handleLogout}
								className="text-sm font-medium transition-colors hover:text-gray-600 text-gray-900 shrink-0"
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
			<Byline />
		</div>
	);
}
