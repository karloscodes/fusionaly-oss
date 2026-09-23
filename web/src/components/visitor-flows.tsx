import { useEffect, useMemo, useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cssVarColor, onThemeChange } from "@/lib/theme";
import { tabClass } from "@/lib/tab-class";
import type { UserFlowLink } from "../types";

// How many pages each column keeps when showing top paths.
const TOP_PER_STEP = 4;

// Layout (SVG units; the SVG scales to the card width).
const WIDTH = 1100;
const NODE_WIDTH = 12;
const GAP = 18;
const TOP = 34;
const BOTTOM = 8;
const LEFT = 150;
const RIGHT = 180;
const MAX_NODE = 64; // thickest node, so a few visitors don't fill the whole chart
const MIN_HEIGHT = 140;

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

function layout(links: UserFlowLink[]) {
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

	// Fit the busiest column in a 320-unit area, but never draw a node thicker
	// than MAX_NODE; then size the chart to what is drawn.
	const fitArea = 320;
	const maxValue = Math.max(...[...nodes.values()].map((n) => n.value), 1);
	const scale = Math.min(
		MAX_NODE / maxValue,
		...columns.map((c) => (fitArea - GAP * (c.length - 1)) / c.reduce((s, n) => s + n.value, 0)),
	);
	const columnHeight = (c: FlowNode[]) => c.reduce((s, n) => s + Math.max(3, n.value * scale), 0) + GAP * (c.length - 1);
	const height = Math.max(MIN_HEIGHT, TOP + BOTTOM + Math.max(...columns.map(columnHeight)));
	const span = steps.length > 1 ? (WIDTH - LEFT - RIGHT) / (steps.length - 1) : 0;
	const xs = steps.map((_, i) => LEFT + i * span);

	const outY = new Map<string, number>();
	const inY = new Map<string, number>();
	columns.forEach((c, ci) => {
		let y = TOP;
		for (const n of c) {
			n.x = xs[ci];
			n.y = y;
			n.h = Math.max(3, n.value * scale);
			outY.set(n.id, y);
			inY.set(n.id, y);
			y += n.h + GAP;
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

	return { nodes: [...nodes.values()], paths, heads, height, lastStep: steps[steps.length - 1] };
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

	const shown = useMemo(() => (showAll ? links : topPaths(links)), [links, showAll]);
	const flow = useMemo(() => layout(shown), [shown]);
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
	const selectedNode = flow.nodes.find((n) => n.id === selected);
	const nextSteps = shown.filter((l) => l.source === selected).sort((a, b) => b.value - a.value);

	const toggle = (id: string) => setSelected((cur) => (cur === id ? null : id));

	return (
		<Card className="rounded-xl border border-black">
			<CardContent className="p-4 sm:p-6">
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

				{shown.length === 0 ? (
					<div className="h-48 flex items-center justify-center">
						<p className="text-sm text-gray-500">No visitor flows for this time period yet.</p>
					</div>
				) : (
					<>
						<div className="overflow-x-auto">
							<svg
								viewBox={`0 0 ${WIDTH} ${flow.height}`}
								className="block w-full min-w-[720px] h-auto"
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
									const tx = last ? n.x + NODE_WIDTH + 8 : n.x - 8;
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
											<text x={tx} y={n.y + n.h / 2 - 2} textAnchor={anchor} fontSize={12} fill={colors.ink} stroke={colors.surface} strokeWidth={4} paintOrder="stroke" fontFamily="var(--c-mono)">
												{n.name}
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
			</CardContent>
		</Card>
	);
};
