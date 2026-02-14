import type { ReactNode } from "react";
import { Settings, Database, User, Server, Bot } from "lucide-react";
import { Link } from "@inertiajs/react";
import { AdminLayout } from "@/components/admin-layout";

interface AdministrationLayoutProps {
	children: ReactNode;
	currentPage: "ingestion" | "ai" | "account" | "system" | "agents";
}

interface NavItem {
	id: string;
	label: string;
	href: string;
	icon: typeof Settings;
}

const navItems: NavItem[] = [
	{
		id: "ingestion",
		label: "Ingestion",
		href: "/admin/administration/ingestion",
		icon: Database,
	},
	{
		id: "agents",
		label: "Agents",
		href: "/admin/administration/agents",
		icon: Bot,
	},
	{
		id: "account",
		label: "Account",
		href: "/admin/administration/account",
		icon: User,
	},
	{
		id: "system",
		label: "System",
		href: "/admin/administration/system",
		icon: Server,
	},
];

export function AdministrationLayout({
	children,
	currentPage,
}: AdministrationLayoutProps) {
	return (
		<AdminLayout currentPath="/admin/administration/ingestion">
			<div className="flex flex-col md:flex-row min-h-screen -mx-4">
				{/* Sidebar - horizontal on mobile, vertical on desktop */}
				<aside className="w-full md:w-64 border-b md:border-b-0 md:border-r border-gray-200 bg-white">
					<div className="md:sticky md:top-0 py-4 md:py-6 px-4">
						<h2 className="text-lg font-semibold text-gray-900 mb-3 md:mb-4 px-3">
							Administration
						</h2>
						<nav className="flex md:flex-col gap-1 md:gap-0 md:space-y-1 overflow-x-auto">
							{navItems.map((item) => {
								const Icon = item.icon;
								const isActive = currentPage === item.id;
								return (
									<Link
										key={item.id}
										href={item.href}
										className={`flex items-center gap-2 md:gap-3 px-3 py-2 text-sm font-medium rounded-md transition-colors whitespace-nowrap ${
											isActive
												? "bg-black text-white"
												: "text-gray-700 hover:bg-gray-100"
										}`}
									>
										<Icon className="h-4 w-4" />
										{item.label}
									</Link>
								);
							})}
						</nav>
					</div>
				</aside>

				{/* Main content */}
				<main className="flex-1 bg-white">
					<div className="max-w-5xl mx-auto py-6 md:py-8 px-4 md:px-8">{children}</div>
				</main>
			</div>
		</AdminLayout>
	);
}
