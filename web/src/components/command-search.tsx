import { useEffect, useMemo, useRef, useState } from "react";
import { router, usePage } from "@inertiajs/react";
import { Search } from "lucide-react";
import { cn } from "@/lib/utils";

interface NameCount {
	name: string;
	count: number;
}

interface SearchResult {
	group: string;
	label: string;
	hint?: string;
	href: string;
}

const PER_GROUP = 5;

// Global settings pages, reachable from any page.
const SETTINGS: SearchResult[] = [
	{ group: "Settings", label: "Ingestion", href: "/admin/administration/ingestion" },
	{ group: "Settings", label: "Agents", href: "/admin/administration/agents" },
	{ group: "Settings", label: "Account", href: "/admin/administration/account" },
	{ group: "Settings", label: "System", href: "/admin/administration/system" },
];

// ⌘K / Ctrl+K search in the top bar. It jumps to the Home feed, a site, a
// section of the current site, a settings page, or a page, referrer or event
// (the Events page, filtered). Pages, referrers and events come from the
// dashboard's props, so they show up while the dashboard is open.
export const CommandSearch = ({ websiteId, websites }: { websiteId?: number; websites?: { id: number; domain: string }[] }) => {
	const { props } = usePage<{
		top_urls?: NameCount[];
		top_referrers?: NameCount[];
		top_custom_events?: NameCount[];
		websites?: { id: number; domain: string }[];
	}>();
	const siteList = websites ?? props.websites ?? [];
	const [query, setQuery] = useState("");
	const [open, setOpen] = useState(false);
	const [active, setActive] = useState(0);
	const inputRef = useRef<HTMLInputElement>(null);
	const boxRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
				e.preventDefault();
				inputRef.current?.focus();
				setOpen(true);
			}
		};
		const onClick = (e: MouseEvent) => {
			if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
		};
		window.addEventListener("keydown", onKey);
		document.addEventListener("mousedown", onClick);
		return () => {
			window.removeEventListener("keydown", onKey);
			document.removeEventListener("mousedown", onClick);
		};
	}, []);

	const results = useMemo(() => {
		const q = query.trim().toLowerCase();
		const matches = (text: string) => !q || text.toLowerCase().includes(q);
		const events = `/admin/websites/${websiteId}/events`;
		const fromData = (group: string, rows: NameCount[] | undefined, param: string): SearchResult[] =>
			(rows || [])
				.filter((r) => matches(r.name))
				.slice(0, PER_GROUP)
				.map((r) => ({ group, label: r.name, hint: `${r.count} visitors`, href: `${events}?${param}=${encodeURIComponent(r.name)}` }));

		const sections: SearchResult[] = [
			{ group: "Go to", label: "Home feed", href: "/admin" },
			...(websiteId
				? [
						{ group: "Go to", label: "Dashboard", href: `/admin/websites/${websiteId}/dashboard` },
						{ group: "Go to", label: "Events", href: events },
						{ group: "Go to", label: "Site settings", href: `/admin/websites/${websiteId}/edit` },
					]
				: []),
		].filter((r) => matches(r.label));
		const settings = SETTINGS.filter((r) => matches(r.label) || matches("settings"));

		const sites: SearchResult[] = siteList
			.filter((w) => w.id !== websiteId && matches(w.domain))
			.slice(0, PER_GROUP)
			.map((w) => ({ group: "Sites", label: w.domain, href: `/admin/websites/${w.id}/dashboard` }));

		// With no query, keep the list short: sections and sites only.
		if (!q) return [...sections, ...sites, ...settings];
		return [
			...fromData("Pages", props.top_urls, "url"),
			...fromData("Referrers", props.top_referrers, "referrer"),
			...fromData("Events", props.top_custom_events, "event_key"),
			...sites,
			...sections,
			...settings,
		];
	}, [query, props.top_urls, props.top_referrers, props.top_custom_events, websiteId, siteList]);

	const go = (r: SearchResult) => {
		setOpen(false);
		setQuery("");
		inputRef.current?.blur();
		router.visit(r.href);
	};

	const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
		if (e.key === "ArrowDown") {
			e.preventDefault();
			setActive((i) => Math.min(i + 1, results.length - 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setActive((i) => Math.max(i - 1, 0));
		} else if (e.key === "Enter" && results[active]) {
			e.preventDefault();
			go(results[active]);
		} else if (e.key === "Escape") {
			setOpen(false);
			inputRef.current?.blur();
		}
	};

	return (
		<div ref={boxRef} className="relative hidden md:block">
			<label className="flex items-center gap-2 w-64 lg:w-72 h-8 px-2.5 rounded-lg border border-gray-200 bg-gray-50 text-gray-500">
				<Search className="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
				<input
					ref={inputRef}
					type="search"
					value={query}
					onChange={(e) => {
						setQuery(e.target.value);
						setActive(0);
						setOpen(true);
					}}
					onFocus={() => setOpen(true)}
					onKeyDown={onKeyDown}
					placeholder="Jump to a site, page, event…"
					aria-label="Jump to a page, referrer, event, site or section"
					role="combobox"
					aria-expanded={open}
					aria-controls="command-search-results"
					className="flex-1 min-w-0 bg-transparent text-[13px] text-gray-900 placeholder:text-gray-500 outline-none"
				/>
				<kbd className="font-mono text-[11px] px-1.5 border border-gray-200 rounded bg-white text-gray-500">⌘K</kbd>
			</label>

			{open && (
				<ul
					id="command-search-results"
					role="listbox"
					className="absolute right-0 top-full mt-1.5 w-80 max-h-96 overflow-y-auto rounded-xl border border-gray-200 bg-white p-1.5 shadow-lg z-50"
				>
					{results.length === 0 && <li className="px-2.5 py-2 text-[13px] text-gray-500">No matches.</li>}
					{results.map((r, i) => (
						<li key={`${r.group}:${r.label}`} role="presentation">
							{(i === 0 || results[i - 1].group !== r.group) && (
								<p className="px-2.5 pt-2 pb-1 text-[10.5px] font-semibold uppercase tracking-[0.08em] text-gray-500">{r.group}</p>
							)}
							<button
								type="button"
								role="option"
								aria-selected={i === active}
								onMouseEnter={() => setActive(i)}
								onClick={() => go(r)}
								className={cn(
									"w-full flex items-center justify-between gap-3 h-8 px-2.5 rounded-lg text-left text-[13px] text-gray-900",
									i === active && "bg-gray-100"
								)}
							>
								<span className="truncate">{r.label}</span>
								{r.hint && <span className="font-mono text-[11px] text-gray-500 whitespace-nowrap">{r.hint}</span>}
							</button>
						</li>
					))}
				</ul>
			)}
		</div>
	);
};
