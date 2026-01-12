import type { Event } from "@/types";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "./ui/table";
import { cn } from "@/lib/utils";

interface EventsTableProps {
	events: Event[];
	isLoading?: boolean;
	groupBySessions?: boolean;
}

interface SessionGroup {
	user: string;
	sessionStart: Date;
	events: Event[];
}

export function EventsTable({ events, isLoading = false, groupBySessions = false }: EventsTableProps) {
	// Format timestamp as relative time (e.g., "2 mins ago")
	const formatRelativeTime = (timestamp: string) => {
		const date = new Date(timestamp);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffSecs = Math.floor(diffMs / 1000);
		const diffMins = Math.floor(diffSecs / 60);
		const diffHours = Math.floor(diffMins / 60);
		const diffDays = Math.floor(diffHours / 24);

		if (diffSecs < 60) {
			return "just now";
		}
		if (diffMins < 60) {
			return `${diffMins} min${diffMins !== 1 ? 's' : ''} ago`;
		}
		if (diffHours < 24) {
			return `${diffHours} hour${diffHours !== 1 ? 's' : ''} ago`;
		}
		if (diffDays < 7) {
			return `${diffDays} day${diffDays !== 1 ? 's' : ''} ago`;
		}
		// For older events, show date
		return date.toLocaleDateString(undefined, {
			month: "short",
			day: "numeric",
			year: diffDays > 365 ? "numeric" : undefined,
		});
	};

	// Format full timestamp for tooltip
	const formatFullTimestamp = (timestamp: string) => {
		const date = new Date(timestamp);
		return date.toLocaleString(undefined, {
			year: "numeric",
			month: "short",
			day: "numeric",
			hour: "numeric",
			minute: "2-digit",
			second: "2-digit",
			hour12: true,
		});
	};

	// Format URL with truncation
	const formatUrl = (url: string) => {
		const truncated = url.length > 40 ? `${url.slice(0, 37)}...` : url;
		return (
			<span
				className="text-gray-800 hover:text-black transition-colors"
				title={url}
			>
				{truncated}
			</span>
		);
	};

	// Truncate timestamp to 30-minute buckets (matching backend session logic)
	const truncateToHalfHour = (date: Date): Date => {
		const rounded = new Date(date);
		const minutes = rounded.getMinutes();
		rounded.setMinutes(minutes < 30 ? 0 : 30);
		rounded.setSeconds(0);
		rounded.setMilliseconds(0);
		return rounded;
	};

	// Group events by user and 30-minute session windows
	const groupEventsBySessions = (events: Event[]): SessionGroup[] => {
		const sessionMap = new Map<string, SessionGroup>();

		for (const event of events) {
			const eventDate = new Date(event.timestamp);
			const sessionTime = truncateToHalfHour(eventDate);
			const sessionKey = `${event.user}-${sessionTime.getTime()}`;

			if (sessionMap.has(sessionKey)) {
				sessionMap.get(sessionKey)!.events.push(event);
			} else {
				sessionMap.set(sessionKey, {
					user: event.user,
					sessionStart: sessionTime,
					events: [event],
				});
			}
		}

		// Convert to array and sort by session start time (newest first)
		return Array.from(sessionMap.values()).sort(
			(a, b) => b.sessionStart.getTime() - a.sessionStart.getTime()
		);
	};

	const sessionGroups = groupBySessions ? groupEventsBySessions(events) : [];

	// Generate loading rows with unique keys
	const loadingRows = Array.from({ length: 10 }, () => crypto.randomUUID()).map(
		(id) => (
			<TableRow key={id} className="animate-pulse">
				<TableCell colSpan={6} className="py-2">
					<div className="h-4 bg-gray-200 rounded w-full" />
				</TableCell>
			</TableRow>
		),
	);

	return (
		<div className="w-full overflow-auto">
			<Table>
				<TableHeader>
					<TableRow className="bg-gray-50 border-b border-gray-200">
						<TableHead className="w-[110px] py-2 px-4 text-gray-800 font-semibold whitespace-nowrap">
							Time
						</TableHead>
						<TableHead className="w-[160px] py-2 px-4 text-gray-800 font-semibold whitespace-nowrap">
							User
						</TableHead>
						<TableHead className="py-2 px-4 text-gray-800 font-semibold whitespace-nowrap">
							URL
						</TableHead>
						<TableHead className="w-[180px] py-2 px-4 text-gray-800 font-semibold whitespace-nowrap">
							Referrer
						</TableHead>
						<TableHead className="w-[180px] py-2 px-4 text-gray-800 font-semibold whitespace-nowrap">
							Event Key
						</TableHead>
						<TableHead className="w-[110px] py-2 px-4 text-gray-800 font-semibold text-center whitespace-nowrap">
							Type
						</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{isLoading ? (
						loadingRows
					) : events.length === 0 ? (
						<TableRow>
							<TableCell colSpan={6} className="py-4 text-center text-gray-500">
								No events found
							</TableCell>
						</TableRow>
					) : groupBySessions ? (
						sessionGroups.map((session, sessionIdx) => (
							<>
								{/* Session header row */}
								<TableRow key={`session-${sessionIdx}`} className="bg-gray-100 border-t border-gray-300">
									<TableCell colSpan={6} className="py-2 px-4">
										<div className="flex items-center gap-3 text-sm font-medium text-gray-800">
											<span className="font-semibold">{session.user}</span>
											<span className="text-gray-500">•</span>
											<span className="text-gray-600">
												Session started {formatRelativeTime(session.sessionStart.toISOString())}
											</span>
											<span className="text-gray-500">•</span>
											<span className="text-gray-600">
												{session.events.length} event{session.events.length !== 1 ? 's' : ''}
											</span>
										</div>
									</TableCell>
								</TableRow>
								{/* Session events */}
								{session.events.map((event, eventIdx) => (
									<TableRow
										key={`session-${sessionIdx}-event-${eventIdx}`}
										className="text-sm hover:bg-gray-50 transition-colors"
									>
										<TableCell className="py-2 px-4 text-gray-700 whitespace-nowrap pl-8">
											<span title={formatFullTimestamp(event.timestamp)} className="cursor-help">
												{formatRelativeTime(event.timestamp)}
											</span>
										</TableCell>
										<TableCell className="py-2 px-4 text-gray-700">
											{/* Empty for grouped view since user is in header */}
										</TableCell>
										<TableCell className="py-2 px-4">
											{formatUrl(event.raw_url)}
										</TableCell>
										<TableCell className="py-2 px-4 text-gray-700">
											{event.referrer === "__direct_or_unknown__" ? (
												<span className="text-gray-500 italic" title="Direct or Unknown">
													Direct
												</span>
											) : (
												<span className="text-gray-800 hover:text-black transition-colors" title={event.referrer}>
													{event.referrer.replace(/^https?:\/\/(www\.)?/, "").split("/")[0]}
												</span>
											)}
										</TableCell>
										<TableCell className={cn("py-2 px-4 text-gray-700", "max-w-xs truncate")}>
											{event.custom_event_key ? (
												<span className="font-medium text-gray-800 block truncate" title={event.custom_event_key}>
													{event.custom_event_key}
												</span>
											) : (
												<span className="text-gray-400 italic">—</span>
											)}
										</TableCell>
										<TableCell className="py-2 px-4 text-center whitespace-nowrap">
											<span
												className={`px-2 py-0.5 rounded-full text-xs font-medium ${
													event.event_type === 1 ? "bg-gray-100 text-gray-700" : "bg-black text-white"
												}`}
											>
												{event.event_type === 1 ? "Page View" : "Event"}
											</span>
										</TableCell>
									</TableRow>
								))}
							</>
						))
					) : (
						events.map((event) => (
							<TableRow
								key={`event-${event.timestamp}-${event.raw_url}`}
								className="text-sm hover:bg-gray-50 transition-colors"
							>
								<TableCell className="py-2 px-4 text-gray-700 whitespace-nowrap">
									<span title={formatFullTimestamp(event.timestamp)} className="cursor-help">
										{formatRelativeTime(event.timestamp)}
									</span>
								</TableCell>
								<TableCell className="py-2 px-4 text-gray-700">
									{event.user}
								</TableCell>
								<TableCell className="py-2 px-4">
									{formatUrl(event.raw_url)}
								</TableCell>
								<TableCell className="py-2 px-4 text-gray-700">
									{event.referrer === "__direct_or_unknown__" ? (
										<span
											className="text-gray-500 italic"
											title="Direct or Unknown"
										>
											Direct
										</span>
									) : (
										<span
											className="text-gray-800 hover:text-black transition-colors"
											title={event.referrer}
										>
											{
												event.referrer
													.replace(/^https?:\/\/(www\.)?/, "")
													.split("/")[0]
											}
										</span>
									)}
								</TableCell>
								<TableCell
									className={cn("py-2 px-4 text-gray-700", "max-w-xs truncate")}
								>
									{event.custom_event_key ? (
										<span
											className="font-medium text-gray-800 block truncate"
											title={event.custom_event_key}
										>
											{event.custom_event_key}
										</span>
									) : (
										<span className="text-gray-400 italic">—</span>
									)}
								</TableCell>
								<TableCell className="py-2 px-4 text-center whitespace-nowrap">
									<span
										className={`px-2 py-0.5 rounded-full text-xs font-medium ${
											event.event_type === 1
												? "bg-gray-100 text-gray-700"
												: "bg-black text-white"
										}`}
									>
										{event.event_type === 1 ? "Page View" : "Event"}
									</span>
								</TableCell>
							</TableRow>
						))
					)}
				</TableBody>
			</Table>
		</div>
	);
}
