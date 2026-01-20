import { ReactNode } from "react";
import { Link, router } from "@inertiajs/react";

interface AdminLayoutProps {
	children: ReactNode;
	currentPath?: string;
}

// Fusionaly Logo Component
const FusionalyLogo = ({ className }: { className?: string }) => (
	<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 64 64" fill="none" className={className}>
		<path
			fill="currentColor"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinejoin="round"
			d="M57 24.5c0-7.4438-6.729-13.5-15-13.5-3.8437 0-7.343 1.319-10 3.4663-2.657-2.1473-6.1563-3.4663-10-3.4663-8.271 0-15 6.0562-15 13.5 0 5.9385 4.2883 10.9832 10.2199 12.7849-.1334.7225-.2199 1.4591-.2199 2.2151 0 7.4438 6.729 13.5 15 13.5s15-6.0562 15-13.5c0-.756-.0865-1.4926-.2199-2.2151C52.7117 35.4832 57 30.4385 57 24.5Zm-48 0c0-6.3413 5.8315-11.5 13-11.5 3.2584 0 6.2325 1.0735 8.5164 2.8318-2.191 2.3473-3.5164 5.3699-3.5164 8.6682 0 .756.0865 1.4926.2199 2.2151-4.4852 1.3624-8.0266 4.5771-9.4875 8.6353C12.676 33.782 9.019 29.5162 9.019 24.5Zm26 0c0 .5961-.0674 1.1772-.1667 1.7491-.9186-.1588-1.864-.2491-2.8333-.2491s-1.9147.0903-2.8333.2491c-.0994-.5719-.1667-1.153-.1667-1.7491 0-2.7874 1.1282-5.3452 3-7.338 1.8718 1.9927 3 4.5505 3 7.338Zm-3 7.338c-1.004-1.0689-1.7841-2.3041-2.3005-3.6467.7481-.1188 1.5143-.1913 2.3005-.1913s1.5524.0724 2.3005.1913c-.5164 1.3426-1.2966 2.5778-2.3005 3.6467Zm4.2676-3.1884c3.7682 1.1635 6.7463 3.814 8.033 7.1591-.7481.1188-1.5143.1913-2.3005.1913-3.2584 0-6.2325-1.0735-8.5164-2.8318 1.2291-1.3167 2.182-2.8465 2.7839-4.5186Zm-5.7512 4.5186c-2.2838 1.7582-5.258 2.8318-8.5164 2.8318-.7862 0-1.5524-.0724-2.3005-.1913 1.2866-3.3451 4.2648-5.9957 8.033-7.1591.6019 1.6721 1.5549 3.2019 2.7839 4.5186Zm14.4836 6.3318c0 6.3413-5.8315 11.5-13 11.5s-13-5.1587-13-11.5c0-.5961.0674-1.1772.1667-1.7491.9186.1588 1.864.2491 2.8333.2491 3.8437 0 7.343-1.319 10-3.4663 2.657 2.1473 6.1563 3.4663 10 3.4663.9692 0 1.9147-.0903 2.8333-.2491.0994.5719.1667 1.153.1667 1.7491Zm1.2676-4.1496c-1.4609-4.0582-5.0023-7.2729-9.4875-8.6353.1334-.7225.2199-1.4591.2199-2.2151 0-3.2983-1.3253-6.3209-3.5164-8.6682 2.2838-1.7582 5.258-2.8318 8.5164-2.8318 7.1685 0 13 5.1587 13 11.5 0 5.0175-3.657 9.2833-8.7324 10.8504Z"
		/>
	</svg>
);

export function AdminLayout({ children, currentPath }: AdminLayoutProps) {
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
			<nav className="border-b border-gray-200">
				<div className="max-w-7xl mx-auto px-4">
					<div className="flex h-14 items-center justify-between">
						{/* Left side: Logo + App name */}
						<div className="flex items-center space-x-4">
							<Link
								href="/admin"
								className="flex items-center gap-2 text-gray-900 hover:text-black transition-colors"
							>
								<FusionalyLogo className="w-6 h-6" />
								<span className="text-lg font-bold tracking-tight">Fusionaly</span>
							</Link>
						</div>

						{/* Right side: Settings + Logout */}
						<div className="flex items-center space-x-4">
							<Link
								href="/admin/administration/ingestion"
								className={`relative text-sm font-medium transition-colors hover:text-gray-900 py-4 ${
									isCurrentPath("/admin/administration")
										? "text-gray-900"
										: "text-gray-500"
								}`}
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
								className="text-sm font-medium transition-colors hover:text-black text-gray-500"
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
