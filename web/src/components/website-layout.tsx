import { ReactNode, useState, useEffect, useRef } from "react";
import { Link, router } from "@inertiajs/react";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft, Settings, ChevronDown, Check } from "lucide-react";

interface Website {
	id: number;
	domain: string;
}

interface WebsiteLayoutProps {
	children: ReactNode;
	websiteId: number;
	websiteDomain: string;
	currentPath?: string;
	websites?: Website[];
}

// Define website-scoped sub-navigation
const getWebsiteNavRoutes = (websiteId: number) => [
	{ path: `/admin/websites/${websiteId}/dashboard`, name: "Dashboard" },
	{ path: `/admin/websites/${websiteId}/events`, name: "Events" },
	{ path: `/admin/websites/${websiteId}/lens`, name: "Ask", badge: "AI" },
];

// Get the current page type from path (dashboard, events, lens, edit)
const getCurrentPageType = (path: string | undefined): string => {
	if (!path) return "dashboard";
	const pathWithoutQuery = path.split("?")[0];
	if (pathWithoutQuery.endsWith("/events")) return "events";
	if (pathWithoutQuery.endsWith("/lens")) return "lens";
	if (pathWithoutQuery.endsWith("/edit")) return "edit";
	return "dashboard";
};

export function WebsiteLayout({
	children,
	websiteId,
	websiteDomain,
	currentPath,
	websites = [],
}: WebsiteLayoutProps) {
	const [isDropdownOpen, setIsDropdownOpen] = useState(false);
	const dropdownRef = useRef<HTMLDivElement>(null);

	const handleLogout = (e: React.MouseEvent<HTMLAnchorElement>) => {
		e.preventDefault();
		router.post("/logout");
	};

	const navRoutes = getWebsiteNavRoutes(websiteId);

	// Check if current path matches (handle query params)
	const isCurrentPath = (path: string) => {
		if (!currentPath) return false;
		const currentWithoutQuery = currentPath.split("?")[0];
		return currentWithoutQuery === path;
	};

	// Close dropdown when clicking outside
	useEffect(() => {
		const handleClickOutside = (event: MouseEvent) => {
			if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
				setIsDropdownOpen(false);
			}
		};
		document.addEventListener("mousedown", handleClickOutside);
		return () => document.removeEventListener("mousedown", handleClickOutside);
	}, []);

	// Handle website switch - navigate to same page type on new website
	const handleWebsiteSwitch = (newWebsiteId: number) => {
		const pageType = getCurrentPageType(currentPath);
		const basePath = `/admin/websites/${newWebsiteId}`;
		const newPath = pageType ? `${basePath}/${pageType}` : basePath;
		setIsDropdownOpen(false);
		router.visit(newPath);
	};

	return (
		<div className="min-h-screen bg-white">
			{/* Navigation Banner */}
			<nav className="border-b border-gray-200">
				<div className="max-w-7xl mx-auto px-4">
					<div className="flex h-14 items-center justify-between">
						{/* Left side: Back + Website name + Sub-nav */}
						<div className="flex items-center space-x-4">
							{/* Back to websites list */}
							<Link
								href="/admin"
								className="flex items-center text-gray-500 hover:text-gray-900 transition-colors"
								title="Back to websites"
							>
								<ArrowLeft className="w-4 h-4" />
							</Link>

							{/* Website domain with dropdown selector and settings icon */}
							<div className="flex items-center gap-2">
								{/* Website Selector Dropdown */}
								<div className="relative" ref={dropdownRef}>
									<button
										onClick={() => websites.length > 1 && setIsDropdownOpen(!isDropdownOpen)}
										className={`flex items-center gap-1.5 text-sm font-semibold text-gray-900 ${
											websites.length > 1 ? "hover:text-black cursor-pointer" : ""
										}`}
									>
										<span>{websiteDomain}</span>
										{websites.length > 1 && (
											<ChevronDown className={`w-3.5 h-3.5 transition-transform ${isDropdownOpen ? "rotate-180" : ""}`} />
										)}
									</button>

									{/* Dropdown Menu */}
									{isDropdownOpen && websites.length > 1 && (
										<div className="absolute top-full left-0 mt-2 w-56 bg-white border border-gray-200 rounded-lg shadow-lg z-50 py-1">
											<div className="px-3 py-2 text-xs font-medium text-gray-500 border-b border-gray-100">
												Switch website
											</div>
											{websites.map((site) => (
												<button
													key={site.id}
													onClick={() => handleWebsiteSwitch(site.id)}
													className={`w-full px-3 py-2 text-left text-sm hover:bg-gray-50 flex items-center justify-between ${
														site.id === websiteId ? "bg-gray-50" : ""
													}`}
												>
													<span className={site.id === websiteId ? "font-medium" : ""}>
														{site.domain}
													</span>
													{site.id === websiteId && (
														<Check className="w-4 h-4 text-green-600" />
													)}
												</button>
											))}
											<div className="border-t border-gray-100 mt-1 pt-1">
												<Link
													href="/admin"
													className="block w-full px-3 py-2 text-left text-sm text-gray-600 hover:bg-gray-50 hover:text-gray-900"
													onClick={() => setIsDropdownOpen(false)}
												>
													View all websites
												</Link>
											</div>
										</div>
									)}
								</div>

								<Link
									href={`/admin/websites/${websiteId}/edit`}
									className="text-gray-500 hover:text-gray-900 transition-colors"
									title="Website settings"
								>
									<Settings className="w-4 h-4" />
								</Link>
							</div>

							{/* Separator */}
							<span className="text-gray-500/30">|</span>

							{/* Sub-navigation with active underline */}
							{navRoutes.map((route) => (
								<Link
									key={route.path}
									href={route.path}
									className={`relative text-sm font-medium transition-colors hover:text-gray-600 py-4 text-gray-900`}
								>
									<span className="relative inline-flex items-center">
										{route.name}
										{route.badge && (
											<Badge
												variant="default"
												className="ml-1.5 bg-black text-white hover:bg-black/90 text-[9px] px-1 py-0 h-3.5 font-semibold"
											>
												{route.badge}
											</Badge>
										)}
									</span>
									{/* Active indicator - black underline */}
									{isCurrentPath(route.path) && (
										<span className="absolute bottom-0 left-0 right-0 h-0.5 bg-black" />
									)}
								</Link>
							))}
						</div>

						{/* Right side: Settings + Logout */}
						<div className="flex items-center space-x-4">
							<Link
								href="/admin/administration/ingestion"
								className={`relative text-sm font-medium transition-colors hover:text-gray-600 py-4 text-gray-900`}
							>
								Settings
								{/* Active indicator - black underline */}
								{currentPath?.startsWith("/admin/administration") && (
									<span className="absolute bottom-0 left-0 right-0 h-0.5 bg-black" />
								)}
							</Link>
							<a
								href="#"
								id="logout"
								onClick={handleLogout}
								className="text-sm font-medium transition-colors hover:text-gray-600 text-gray-900"
							>
								Logout
							</a>
						</div>
					</div>
				</div>
			</nav>

			{/* Main Content */}
			<main className="max-w-7xl mx-auto px-4">{children}</main>
		</div>
	);
}
