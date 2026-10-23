import { useEffect, useMemo, useRef, useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cssVarColor, onThemeChange } from "@/lib/theme";
import { tabClass } from "@/lib/tab-class";
import type { UserFlowLink } from "../types";

// How many pages each column keeps when showing top paths.
const TOP_PER_STEP = 4;

// Layout, in pixels: the SVG draws at 1:1, so labels stay at 12px on any card.
// Each column gets the room its longest label needs. Long page names are cut
// until the chart fits the card, down to MIN_LABEL characters; below that the
// chart scrolls sideways.
const NODE_WIDTH = 12;
const GAP = 16; // between the slots of one column
const LABEL_SLOT = 34; // a node's slot is never shorter than its two label lines
const TOP = 44;
const BOTTOM = 16;
const CHAR_WIDTH = 7.3; // the 12px monospace font
const MAX_LABEL = 30; // characters of a page name; the full name shows on hover
const MIN_LABEL = 16;
const LABEL_PAD = 10; // between a node and its label
const FLOW_ROOM = 24; // flow visible beside the longest label of a column
const MIN_SPAN = 120; // between two columns
const MAX_SPAN = 320; // a wide card centers the chart instead of spreading it further
const MAX_NODE = 110; // thickest node, so a few visitors don't fill the whole chart
const MIN_HEIGHT = 300; // room to breathe even with one path

const shortName = (name: string, limit: number) => (name.length > limit ? `${name.slice(0, limit - 1)}…` : name);

interface FlowNode {
	id: string;
	step: number;
	name: string;
	inValue: number;
	outValue: number;
	value: number;
	x: number;
	y: number;
	h: number;
}

// Node ids look like "step1:example.com/home". Labels drop the domain.
function parseNodeId(id: string): { step: number; name: string } {
	const m = id.match(/^step(\d+):(.+)$/);
	const page = m ? m[2] : id;
	const path = page.replace(/^[^/]+(?=\/)/, "");
	return { step: m ? Number(m[1]) : 0, name: path || page };
}

// Keep the busiest pages per step, and only the links between kept pages.
function topPaths(links: UserFlowLink[]): UserFlowLink[] {
	const valueOf = new Map<string, number>();
	for (const l of links) {
		valueOf.set(l.source, (valueOf.get(l.source) || 0) + l.value);
		valueOf.set(l.target, Math.max(valueOf.get(l.target) || 0, l.value));
	}
	const byStep = new Map<number, string[]>();
	for (const id of valueOf.keys()) {
		const { step } = parseNodeId(id);
		byStep.set(step, [...(byStep.get(step) || []), id]);
	}
	const kept = new Set<string>();
	for (const ids of byStep.values()) {
		ids.sort((a, b) => (valueOf.get(b) || 0) - (valueOf.get(a) || 0));
		ids.slice(0, TOP_PER_STEP).forEach((id) => kept.add(id));
	}
	return links.filter((l) => kept.has(l.source) && kept.has(l.target));
}

function layout(links: UserFlowLink[], available: number) {
	const nodes = new Map<string, FlowNode>();
	const node = (id: string) => {
		if (!nodes.has(id)) {
			const { step, name } = parseNodeId(id);
			nodes.set(id, { id, step, name, inValue: 0, outValue: 0, value: 0, x: 0, y: 0, h: 0 });
		}
		return nodes.get(id)!;
	};
	for (const l of links) {
		node(l.source).outValue += l.value;
		node(l.target).inValue += l.value;
	}

	const steps = [...new Set([...nodes.values()].map((n) => n.step))].sort((a, b) => a - b);
	for (const n of nodes.values()) n.value = Math.max(n.inValue, n.outValue);
	const columns = steps.map((s) =>
		[...nodes.values()].filter((n) => n.step === s).sort((a, b) => b.value - a.value),
	);

	// Fit the busiest column in a 440px area, but never draw a node thicker
	// than MAX_NODE; then size the chart to what is drawn.
	const fitArea = 440;
	const maxValue = Math.max(...[...nodes.values()].map((n) => n.value), 1);
	const scale = Math.min(
		MAX_NODE / maxValue,
		...columns.map((c) => (fitArea - GAP * (c.length - 1)) / c.reduce((s, n) => s + n.value, 0)),
	);
	for (const n of nodes.values()) n.h = Math.max(3, n.value * scale);
	const slot = (n: FlowNode) => Math.max(n.h, LABEL_SLOT);
	const columnHeight = (c: FlowNode[]) => c.reduce((s, n) => s + slot(n), 0) + GAP * (c.length - 1);
	const height = Math.max(MIN_HEIGHT, TOP + BOTTOM + Math.max(...columns.map(columnHeight)));

	// Labels sit left of their node, except in the last column, where they
	// sit right. A gap between two columns holds the labels of the right one.
	const last = columns.length - 1;
	const fit = (limit: number) => {
		const labelWidth = (c: FlowNode[]) => Math.max(...c.map((n) => shortName(n.name, limit).length)) * CHAR_WIDTH + LABEL_PAD;
		const left = labelWidth(columns[0]) + 4;
		const right = labelWidth(columns[last]) + 4;
		const minSpans = columns.map((c, i) => (i === 0 ? 0 : i === last ? MIN_SPAN : Math.max(MIN_SPAN, labelWidth(c) + FLOW_ROOM)));
		const minWidth = left + right + columns.length * NODE_WIDTH + minSpans.reduce((s, w) => s + w, 0);
		return { limit, left, right, minSpans, minWidth };
	};
	let sized = fit(MAX_LABEL);
	while (available > 0 && sized.minWidth > available && sized.limit > MIN_LABEL) sized = fit(sized.limit - 1);
	const { limit, left, right, minSpans, minWidth } = sized;
	const width = Math.max(minWidth, available);
	const extra = last > 0 ? (width - minWidth) / last : 0;
	const spans = minSpans.map((w, i) => (i === 0 ? 0 : Math.min(w + extra, Math.max(w, MAX_SPAN))));
	const used = left + right + columns.length * NODE_WIDTH + spans.reduce((s, w) => s + w, 0);
	const xs: number[] = [];
	spans.forEach((w, i) => xs.push(i === 0 ? (width - used) / 2 + left : xs[i - 1] + NODE_WIDTH + w));

	const outY = new Map<string, number>();
	const inY = new Map<string, number>();
	// Center each column in the drawing area, so a short column does not sit
	// at the top of a tall chart. A small node sits in the middle of its slot.
	columns.forEach((c, ci) => {
		let y = TOP + (height - TOP - BOTTOM - columnHeight(c)) / 2;
		for (const n of c) {
			n.x = xs[ci];
			n.y = y + (slot(n) - n.h) / 2;
			outY.set(n.id, n.y);
			inY.set(n.id, n.y);
			y += slot(n) + GAP;
		}
	});

	const paths = [...links]
		.sort((a, b) => b.value - a.value)
		.map((l) => {
			const s = nodes.get(l.source)!;
			const t = nodes.get(l.target)!;
			const w = l.value * scale;
			const y0 = outY.get(s.id)! + w / 2;
			const y1 = inY.get(t.id)! + w / 2;
			outY.set(s.id, outY.get(s.id)! + w);
			inY.set(t.id, inY.get(t.id)! + w);
			const x0 = s.x + NODE_WIDTH;
			const x1 = t.x;
			const xm = (x0 + x1) / 2;
			return { link: l, d: `M${x0},${y0} C${xm},${y0} ${xm},${y1} ${x1},${y1}`, width: Math.max(1.5, w) };
		});

	const heads = steps.map((step, i) => ({
		x: xs[i] + NODE_WIDTH / 2,
		label: i === 0 ? "ENTRY" : i === steps.length - 1 ? "EXIT" : `STEP ${step}`,
	}));

	return { nodes: [...nodes.values()], paths, heads, width, height, limit, lastStep: steps[steps.length - 1] };
}

const visitors = (n: number) => `${n} ${n === 1 ? "visitor" : "visitors"}`;

function useFlowColors() {
	const read = () => ({
		accent: cssVarColor("--c-accent") || "#00D1FF",
		ink: cssVarColor("--c-gray-900") || "#18181b",
		muted: cssVarColor("--c-gray-500") || "#71717a",
		surface: cssVarColor("--c-white") || "#ffffff",
	});
	const [colors, setColors] = useState(read);
	useEffect(() => {
		setColors(read());
		return onThemeChange(() => setColors(read()));
	}, []);
	return colors;
}

export const VisitorFlows = ({ links }: { links: UserFlowLink[] }) => {
	const [showAll, setShowAll] = useState(false);
	const [selected, setSelected] = useState<string | null>(null);
	const colors = useFlowColors();

	const box = useRef<HTMLDivElement>(null);
	const [available, setAvailable] = useState(0);
	useEffect(() => {
		if (!box.current) return;
		const observer = new ResizeObserver(([entry]) => setAvailable(Math.floor(entry.contentRect.width)));
		observer.observe(box.current);
		return () => observer.disconnect();
	}, []);

	const shown = useMemo(() => (showAll ? links : topPaths(links)), [links, showAll]);
	const flow = useMemo(() => (shown.length > 0 ? layout(shown, available) : null), [shown, available]);
	const hiddenCount = links.length - topPaths(links).length;

	// What the selection touches: the node and every node linked to it.
	const touched = new Set<string>();
	if (selected) {
		touched.add(selected);
		for (const l of shown) {
			if (l.source === selected) touched.add(l.target);
			if (l.target === selected) touched.add(l.source);
		}
	}
	const selectedNode = flow?.nodes.find((n) => n.id === selected);
	const nextSteps = shown.filter((l) => l.source === selected).sort((a, b) => b.value - a.value);

	const toggle = (id: string) => setSelected((cur) => (cur === id ? null : id));

	return (
		<Card className="rounded-xl border border-black">
			<CardContent className="p-4 sm:p-6">
				<div ref={box}>
					<div className="mb-3 flex flex-wrap items-center gap-2">
						<h2 className="text-base font-semibold text-gray-900">Visitor Flows</h2>
						<span className="text-xs text-gray-500">Entry pages → Navigation → Exit pages</span>
						<span className="flex-1" />
						{hiddenCount > 0 && (
							<div className="flex gap-1">
								<button
									type="button"
									aria-pressed={!showAll}
									onClick={() => setShowAll(false)}
									className={tabClass(!showAll)}
								>
									Top paths
								</button>
								<button
									type="button"
									aria-pressed={showAll}
									onClick={() => setShowAll(true)}
									className={tabClass(showAll)}
								>
									All paths
								</button>
							</div>
						)}
					</div>

					{!flow ? (
						<div className="h-48 flex items-center justify-center">
							<p className="text-sm text-gray-500">No visitor flows for this time period yet.</p>
						</div>
					) : (
						<>
							<div className="overflow-x-auto -mx-4 px-4">
								<svg
									width={flow.width}
									height={flow.height}
									viewBox={`0 0 ${flow.width} ${flow.height}`}
									className="block max-w-none"
									role="img"
									aria-label="Visitor flows from entry pages to exit pages"
								>
									{flow.heads.map((h) => (
										<text key={h.label} x={h.x} y={14} textAnchor="middle" fontSize={11} letterSpacing="0.08em" fill={colors.muted} fontFamily="var(--c-mono)">
											{h.label}
										</text>
									))}
									{flow.paths.map((p) => {
										const on = selected && (p.link.source === selected || p.link.target === selected);
										return (
											<path
												key={`${p.link.source}->${p.link.target}`}
												d={p.d}
												fill="none"
												stroke={colors.accent}
												strokeWidth={p.width}
												strokeOpacity={selected ? (on ? 0.75 : 0.07) : 0.28}
											>
												<title>{`${parseNodeId(p.link.source).name} → ${parseNodeId(p.link.target).name}: ${visitors(p.link.value)}`}</title>
											</path>
										);
									})}
									{flow.nodes.map((n) => {
										const last = n.step === flow.lastStep;
										const tx = last ? n.x + NODE_WIDTH + LABEL_PAD : n.x - LABEL_PAD;
										const anchor = last ? "start" : "end";
										const dim = selected && !touched.has(n.id);
										return (
											<g
												key={n.id}
												tabIndex={0}
												role="button"
												aria-pressed={selected === n.id}
												aria-label={`${n.name}, ${visitors(n.value)}`}
												onClick={() => toggle(n.id)}
												onKeyDown={(e) => {
													if (e.key === "Enter" || e.key === " ") {
														e.preventDefault();
														toggle(n.id);
													}
												}}
												className="cursor-pointer focus:outline-none"
											>
												<rect
													x={n.x}
													y={n.y}
													width={NODE_WIDTH}
													height={n.h}
													rx={2}
													fill={colors.accent}
													fillOpacity={dim ? 0.3 : 1}
												/>
												<title>{`${n.name}: ${visitors(n.value)}`}</title>
												<text x={tx} y={n.y + n.h / 2 - 2} textAnchor={anchor} fontSize={12} fill={colors.ink} stroke={colors.surface} strokeWidth={4} paintOrder="stroke" fontFamily="var(--c-mono)">
													{shortName(n.name, flow.limit)}
												</text>
												<text x={tx} y={n.y + n.h / 2 + 13} textAnchor={anchor} fontSize={11} fill={colors.muted} stroke={colors.surface} strokeWidth={4} paintOrder="stroke" fontFamily="var(--c-mono)">
													{n.value}
												</text>
											</g>
										);
									})}
								</svg>
							</div>

							<div className="mt-3 flex flex-wrap items-baseline gap-x-[18px] gap-y-1 min-h-[42px] rounded-lg border border-gray-200 bg-white px-3 py-2.5 text-[13px] text-gray-500" aria-live="polite">
								{selectedNode ? (
									<>
										<span>
											<b className="font-mono font-medium text-gray-900">{selectedNode.name}</b> · {visitors(selectedNode.value)}
										</span>
										{selectedNode.inValue > 0 && <span>{selectedNode.inValue} arrived from the previous step</span>}
										{nextSteps.length > 0 ? (
											<span>
												next:{" "}
												{nextSteps.map((l, i) => (
													<span key={l.target}>
														{i > 0 && " · "}
														<b className="font-mono font-medium text-gray-900">{parseNodeId(l.target).name}</b> {l.value}
													</span>
												))}
											</span>
										) : (
											<span>left the site here</span>
										)}
									</>
								) : (
									<span>Select a page to trace where its visitors came from and where they went.</span>
								)}
							</div>
						</>
					)}
				</div>
			</CardContent>
		</Card>
	);
};
