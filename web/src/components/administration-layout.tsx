import type { ReactNode } from "react";
import { Settings, Database, User, Server } from "lucide-react";
import { Link } from "@inertiajs/react";
import { AdminLayout } from "@/components/admin-layout";

interface AdministrationLayoutProps {
	children: ReactNode;
	currentPage: "ingestion" | "ai" | "account" | "system";
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
			<div className="flex min-h-screen -mx-4">
				{/* Sidebar */}
				<aside className="w-64 border-r border-gray-200 bg-white">
					<div className="sticky top-0 py-6 px-4">
						<h2 className="text-lg font-semibold text-gray-900 mb-4 px-3">
							Administration
						</h2>
						<nav className="space-y-1">
							{navItems.map((item) => {
								const Icon = item.icon;
								const isActive = currentPage === item.id;
								return (
									<Link
										key={item.id}
										href={item.href}
										className={`flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-md transition-colors ${
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
					<div className="max-w-5xl mx-auto py-8 px-8">{children}</div>
				</main>
			</div>
		</AdminLayout>
	);
}
