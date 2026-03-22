import { ReactNode, useState, useEffect, useRef } from "react";
import { Link, router } from "@inertiajs/react";
import { AlertTriangle } from "lucide-react";
import avatarUrl from "@/assets/avatar.jpg";

interface AdminLayoutProps {
	children: ReactNode;
	currentPath?: string;
	badge?: ReactNode;
	hideFeedback?: boolean;
}

// Fusionaly Logo Component - Text wordmark with green underscore
const FusionalyLogo = () => (
	<span className="text-lg font-semibold font-mono">
		fusionaly<span className="text-[#00D678]">_</span>
	</span>
);

export interface NotificationItem {
	id: string;
	title: string;
	body: string;
	expires: string;
}

interface SystemHealth {
	healthy: boolean;
	warning: string;
	notifications: NotificationItem[];
}

function getDismissedIds(): string[] {
	return JSON.parse(localStorage.getItem("fusionaly_dismissed_notifications") || "[]");
}

function getNotificationsByState(notifications: NotificationItem[]): { unread: NotificationItem[]; read: NotificationItem[] } {
	const now = new Date().toISOString().slice(0, 10);
	const dismissed = getDismissedIds();
	const valid = notifications.filter((n) => n.expires >= now);
	return {
		unread: valid.filter((n) => !dismissed.includes(n.id)),
		read: valid.filter((n) => dismissed.includes(n.id)),
	};
}

function dismissNotification(id: string) {
	const dismissed = getDismissedIds();
	if (!dismissed.includes(id)) {
		localStorage.setItem("fusionaly_dismissed_notifications", JSON.stringify([...dismissed, id]));
	}
}


export function FeedbackWidget({ notifications = [] }: { notifications: NotificationItem[] }) {
	const [open, setOpen] = useState(false);
	const [state, setState] = useState(() => getNotificationsByState(notifications));
	const [expandedRead, setExpandedRead] = useState<string | null>(null);
	const hasNotifications = state.unread.length > 0;
	const ref = useRef<HTMLDivElement>(null);

	// Sync when notifications arrive from async health fetch
	useEffect(() => {
		if (notifications.length > 0) {
			setState(getNotificationsByState(notifications));
		}
	}, [notifications.length]);

	useEffect(() => {
		if (!open) return;
		const handler = (e: MouseEvent) => {
			if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
		};
		document.addEventListener("mousedown", handler);
		return () => document.removeEventListener("mousedown", handler);
	}, [open]);

	const handleDismiss = (id: string) => {
		dismissNotification(id);
		setState((prev) => {
			const dismissed = prev.unread.find((n) => n.id === id);
			return {
				unread: prev.unread.filter((n) => n.id !== id),
				read: dismissed ? [dismissed, ...prev.read] : prev.read,
			};
		});
	};

	return (
		<div ref={ref} className="fixed bottom-8 right-8 z-50 flex flex-col items-end gap-3">
			<div className="group">
				{open && (
					<div className="bg-white border border-black/10 shadow-lg rounded-xl p-4 w-80 text-sm mb-3 max-h-96 overflow-y-auto">
						{state.unread.map((n, i) => (
							<div key={n.id} className={i > 0 ? "mt-3 pt-3 border-t border-black/10" : ""}>
								<div className="flex items-start justify-between gap-2">
									<p className="font-semibold text-sm text-black mb-1">{n.title}</p>
									<button onClick={() => handleDismiss(n.id)} className="text-black/30 hover:text-black text-xs shrink-0">✕</button>
								</div>
								<p className="text-black/70 text-xs leading-relaxed whitespace-pre-wrap break-all">{n.body}</p>
							</div>
						))}
						{state.read.map((n, i) => (
							<div
								key={n.id}
								className={`${state.unread.length > 0 || i > 0 ? "mt-3 pt-3 border-t border-black/10" : ""} cursor-pointer`}
								onClick={() => setExpandedRead(expandedRead === n.id ? null : n.id)}
							>
								<p className="text-sm text-black/40 hover:text-black/60 transition-colors">{n.title}</p>
								{expandedRead === n.id && (
									<p className="text-black/40 text-xs leading-relaxed whitespace-pre-wrap break-all mt-1">{n.body}</p>
								)}
							</div>
						))}
						{(state.unread.length > 0 || state.read.length > 0) && <div className="mt-3 pt-3 border-t border-black/10" />}
						<p className="text-black/70 text-sm leading-relaxed mb-3">
							Something broken or an idea?
						</p>
						<a
							href="https://github.com/karloscodes/fusionaly-oss/issues/new"
							target="_blank"
							rel="noopener noreferrer"
							className="font-semibold text-sm hover:underline"
						>
							open an issue →
						</a>
					</div>
				)}
				<div className="flex justify-end">
					<div className="relative">
						<div
							onClick={() => setOpen(!open)}
							className="rounded-full overflow-hidden w-14 h-14 ring-2 ring-white shadow-lg hover:scale-105 transition-transform cursor-pointer"
						>
							<img
								src={avatarUrl}
								alt="Carlos"
								className="w-full h-full object-cover"
								style={{ objectPosition: "center 1%" }}
							/>
						</div>
						{hasNotifications && (
							<span className="absolute top-0 right-0 w-4 h-4 bg-green-500 rounded-full border-2 border-white animate-pulse" />
						)}
					</div>
				</div>
			</div>
		</div>
	);
}

export function AdminLayout({ children, currentPath, badge, hideFeedback }: AdminLayoutProps) {
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
					<div className="flex h-14 items-center justify-between">
						<div className="flex items-center space-x-4">
							<Link
								href="/admin"
								className="flex items-center gap-2 text-gray-900 hover:text-black transition-colors"
							>
								<FusionalyLogo />
								{badge}
							</Link>
						</div>

						<div className="flex items-center space-x-4">
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
								className="relative text-sm font-medium transition-colors hover:text-gray-600 py-4 text-gray-900"
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
								className="text-sm font-medium transition-colors hover:text-gray-600 text-gray-900"
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

			{!hideFeedback && <FeedbackWidget notifications={health?.notifications ?? []} />}
		</div>
	);
}
