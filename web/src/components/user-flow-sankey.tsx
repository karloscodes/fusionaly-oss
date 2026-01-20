import { useEffect, useRef, useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { GitBranch, ZoomIn, ZoomOut, RotateCcw, Maximize2, X, HelpCircle, MousePointer, Move } from "lucide-react";
import { cn } from "@/lib/utils";

interface UserFlowLink {
	source: string;
	target: string;
	value: number;
}

interface VisitorFlowSankeyProps {
	links: UserFlowLink[];
}

interface SankeyNode {
	id: string;
	displayName: string;
	step: number;
	x: number;
	y: number;
	height: number;
	value: number;
}

interface SankeyLink {
	source: SankeyNode;
	target: SankeyNode;
	value: number;
	y0: number;
	y1: number;
	thickness: number;
}

// Color scheme - gradient from entry to exit
const STEP_COLORS = [
	"#3b82f6", // blue-500 - step 1 (entry)
	"#6366f1", // indigo-500 - step 2
	"#8b5cf6", // violet-500 - step 3
	"#a855f7", // purple-500 - step 4
	"#c084fc", // purple-400 - step 5
	"#d946ef", // fuchsia-500 - step 6+
];

const getStepColor = (step: number): string => {
	const index = Math.min(step - 1, STEP_COLORS.length - 1);
	return STEP_COLORS[Math.max(0, index)];
};

// Parse step number and page name from node id (e.g., "step1:/home" -> { step: 1, page: "/home" })
const parseNodeId = (id: string): { step: number; page: string } => {
	const match = id.match(/^step(\d+):(.+)$/);
	if (match) {
		return { step: parseInt(match[1], 10), page: match[2] };
	}
	return { step: 0, page: id };
};

export const VisitorFlowSankey = ({ links }: VisitorFlowSankeyProps) => {
	const svgRef = useRef<SVGSVGElement>(null);
	const containerRef = useRef<HTMLDivElement>(null);
	const [zoom, setZoom] = useState(0.7);
	const [hoveredNode, setHoveredNode] = useState<string | null>(null);
	const [selectedNode, setSelectedNode] = useState<string | null>(null);
	const [hoveredLinkKey, setHoveredLinkKey] = useState<string | null>(null);
	const [selectedLinkKey, setSelectedLinkKey] = useState<string | null>(null);
	const [isPanning, setIsPanning] = useState(false);
	const [isFullscreen, setIsFullscreen] = useState(false);
	const [showHelp, setShowHelp] = useState(false);
	const [viewportHeight, setViewportHeight] = useState(() => (typeof window !== "undefined" ? window.innerHeight : 900));
	const isPanningRef = useRef(false);
	const panStartRef = useRef({ x: 0, y: 0, scrollLeft: 0, scrollTop: 0 });

	// Calculate Sankey layout from links
	const calculateSankeyLayout = (
		links: UserFlowLink[],
		columnWidth: number,
		height: number,
	): { nodes: SankeyNode[]; links: SankeyLink[] } => {
		const nodeMap = new Map<string, SankeyNode>();
		const nodesByStep = new Map<number, string[]>();

		// First pass: identify all unique nodes and their steps
		links.forEach((link) => {
			const sourceInfo = parseNodeId(link.source);
			const targetInfo = parseNodeId(link.target);

			if (!nodeMap.has(link.source)) {
				nodeMap.set(link.source, {
					id: link.source,
					displayName: sourceInfo.page,
					step: sourceInfo.step,
					x: 0,
					y: 0,
					height: 0,
					value: 0,
				});
				if (!nodesByStep.has(sourceInfo.step)) {
					nodesByStep.set(sourceInfo.step, []);
				}
				nodesByStep.get(sourceInfo.step)!.push(link.source);
			}

			if (!nodeMap.has(link.target)) {
				nodeMap.set(link.target, {
					id: link.target,
					displayName: targetInfo.page,
					step: targetInfo.step,
					x: 0,
					y: 0,
					height: 0,
					value: 0,
				});
				if (!nodesByStep.has(targetInfo.step)) {
					nodesByStep.set(targetInfo.step, []);
				}
				nodesByStep.get(targetInfo.step)!.push(link.target);
			}
		});

		// Calculate node values
		const outgoingValues = new Map<string, number>();
		const incomingValues = new Map<string, number>();

		links.forEach((link) => {
			outgoingValues.set(link.source, (outgoingValues.get(link.source) || 0) + link.value);
			incomingValues.set(link.target, (incomingValues.get(link.target) || 0) + link.value);
		});

		nodeMap.forEach((node, id) => {
			const incoming = incomingValues.get(id) || 0;
			const outgoing = outgoingValues.get(id) || 0;
			node.value = Math.max(incoming, outgoing);
		});

		// Sort steps and nodes within each step by value
		const sortedSteps = Array.from(nodesByStep.keys()).sort((a, b) => a - b);

		nodesByStep.forEach((nodeIds) => {
			nodeIds.sort((a, b) => {
				const nodeA = nodeMap.get(a)!;
				const nodeB = nodeMap.get(b)!;
				return nodeB.value - nodeA.value;
			});
		});

		// Calculate node positions
		const padding = 60;
		const bottomPadding = 100;
		const minNodeHeight = 20;
		const nodeGap = 16;
		const leftPadding = 150;

		sortedSteps.forEach((step, stepIndex) => {
			const nodeIds = nodesByStep.get(step) || [];
			const totalValue = nodeIds.reduce((sum, id) => sum + (nodeMap.get(id)?.value || 0), 0);
			const availableHeight = height - padding - bottomPadding - (nodeIds.length - 1) * nodeGap;

			let currentY = padding;
			nodeIds.forEach((nodeId) => {
				const node = nodeMap.get(nodeId)!;
				const proportionalHeight = totalValue > 0 ? (node.value / totalValue) * availableHeight : minNodeHeight;
				const nodeHeight = Math.max(minNodeHeight, proportionalHeight);

				node.x = leftPadding + stepIndex * columnWidth;
				node.y = currentY;
				node.height = nodeHeight;

				currentY += nodeHeight + nodeGap;
			});
		});

		// Calculate link positions
		const sankeyLinks: SankeyLink[] = [];
		const linksBySource = new Map<string, UserFlowLink[]>();

		links.forEach((link) => {
			if (!linksBySource.has(link.source)) {
				linksBySource.set(link.source, []);
			}
			linksBySource.get(link.source)!.push(link);
		});

		const minLinkThickness = 4;
		const maxGap = 8;

		linksBySource.forEach((group, sourceId) => {
			const sourceNode = nodeMap.get(sourceId);
			if (!sourceNode) return;

			const entries = group
				.map((link) => {
					const targetNode = nodeMap.get(link.target);
					if (!targetNode) return null;
					const denominator = Math.max(sourceNode.value, 1);
					const rawHeight = (link.value / denominator) * sourceNode.height;
					const thickness = Math.max(rawHeight, minLinkThickness);
					return { link, targetNode, thickness };
				})
				.filter((entry): entry is { link: UserFlowLink; targetNode: SankeyNode; thickness: number } => Boolean(entry));

			if (entries.length === 0) return;

			entries.sort((a, b) => b.link.value - a.link.value);

			const totalThickness = entries.reduce((sum, entry) => sum + entry.thickness, 0);
			const available = sourceNode.height;
			const gap = entries.length > 1
				? Math.max(0, Math.min(maxGap, (available - totalThickness) / Math.max(entries.length - 1, 1)))
				: 0;
			const totalNeeded = totalThickness + Math.max(entries.length - 1, 0) * gap;
			let offset = Math.max((available - totalNeeded) / 2, 0);

			entries.forEach((entry) => {
				sankeyLinks.push({
					source: sourceNode,
					target: entry.targetNode,
					value: entry.link.value,
					y0: offset,
					y1: 0,
					thickness: entry.thickness,
				});
				offset += entry.thickness + gap;
			});
		});

		// Calculate target y positions
		const linksByTarget = new Map<string, SankeyLink[]>();
		sankeyLinks.forEach((link) => {
			const group = linksByTarget.get(link.target.id);
			if (group) {
				group.push(link);
			} else {
				linksByTarget.set(link.target.id, [link]);
			}
		});

		linksByTarget.forEach((group, targetId) => {
			const targetNode = nodeMap.get(targetId);
			if (!targetNode) return;

			group.sort((a, b) => a.y0 - b.y0);

			const totalThickness = group.reduce((sum, link) => sum + link.thickness, 0);
			const available = targetNode.height;
			const gap = group.length > 1
				? Math.max(0, Math.min(maxGap, (available - totalThickness) / Math.max(group.length - 1, 1)))
				: 0;
			const totalNeeded = totalThickness + Math.max(group.length - 1, 0) * gap;
			let offset = Math.max((available - totalNeeded) / 2, 0);

			group.forEach((link) => {
				link.y1 = offset;
				offset += link.thickness + gap;
			});
		});

		return {
			nodes: Array.from(nodeMap.values()),
			links: sankeyLinks,
		};
	};

	const linkKey = (link: SankeyLink) => `${link.source.id}|${link.target.id}|${link.value}`;

	const generateLinkPath = (link: SankeyLink, nodeWidth: number): string => {
		const { source, target, y0, y1 } = link;
		const linkHeight = link.thickness;

		const x0 = source.x + nodeWidth;
		const x1 = target.x;
		const xi = (x0 + x1) / 2;

		const y0Start = source.y + y0;
		const y0End = y0Start + linkHeight;
		const y1Start = target.y + y1;
		const y1End = y1Start + linkHeight;

		return `
			M ${x0},${y0Start}
			C ${xi},${y0Start} ${xi},${y1Start} ${x1},${y1Start}
			L ${x1},${y1End}
			C ${xi},${y1End} ${xi},${y0End} ${x0},${y0End}
			Z
		`;
	};

	const truncatePageName = (name: string, maxLength = 25): string => {
		if (name.length <= maxLength) return name;
		return `${name.substring(0, maxLength)}...`;
	};

	// Enable click-and-drag panning
	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		const handlePointerDown = (event: PointerEvent) => {
			if (event.button !== 0) return;
			const target = event.target as Element | null;
			// Skip if clicking on buttons or SVG interactive elements (paths, rects)
			if (target?.closest("button")) return;
			if (target?.tagName === "path" || target?.tagName === "rect") return;
			isPanningRef.current = true;
			setIsPanning(true);
			container.classList.add("select-none");
			panStartRef.current = {
				x: event.clientX,
				y: event.clientY,
				scrollLeft: container.scrollLeft,
				scrollTop: container.scrollTop,
			};
			container.setPointerCapture?.(event.pointerId);
		};

		const handlePointerMove = (event: PointerEvent) => {
			if (!isPanningRef.current) return;
			event.preventDefault();
			const dx = event.clientX - panStartRef.current.x;
			const dy = event.clientY - panStartRef.current.y;
			container.scrollLeft = panStartRef.current.scrollLeft - dx;
			container.scrollTop = panStartRef.current.scrollTop - dy;
		};

		const endPan = (event: PointerEvent) => {
			if (!isPanningRef.current) return;
			isPanningRef.current = false;
			setIsPanning(false);
			container.classList.remove("select-none");
			try {
				container.releasePointerCapture?.(event.pointerId);
			} catch {
				// Ignore release errors
			}
		};

		container.addEventListener("pointerdown", handlePointerDown);
		container.addEventListener("pointermove", handlePointerMove);
		container.addEventListener("pointerup", endPan);
		container.addEventListener("pointerleave", endPan);
		container.addEventListener("pointercancel", endPan);

		return () => {
			container.removeEventListener("pointerdown", handlePointerDown);
			container.removeEventListener("pointermove", handlePointerMove);
			container.removeEventListener("pointerup", endPan);
			container.removeEventListener("pointerleave", endPan);
			container.removeEventListener("pointercancel", endPan);
		};
	}, []);

	useEffect(() => {
		if (typeof window === "undefined") return;
		const handleResize = () => setViewportHeight(window.innerHeight);
		handleResize();
		window.addEventListener("resize", handleResize);
		return () => window.removeEventListener("resize", handleResize);
	}, []);

	useEffect(() => {
		if (!isFullscreen) return;
		const handleKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") setIsFullscreen(false);
		};
		window.addEventListener("keydown", handleKeyDown);
		const previousOverflow = typeof document !== "undefined" ? document.body.style.overflow : undefined;
		if (typeof document !== "undefined") {
			document.body.style.overflow = "hidden";
		}
		return () => {
			window.removeEventListener("keydown", handleKeyDown);
			if (typeof document !== "undefined" && previousOverflow !== undefined) {
				document.body.style.overflow = previousOverflow;
			}
		};
	}, [isFullscreen]);

	const handleReset = () => {
		setZoom(0.7);
		setSelectedNode(null);
		setSelectedLinkKey(null);
		setHoveredNode(null);
		setHoveredLinkKey(null);
		if (containerRef.current) {
			containerRef.current.scrollTop = 0;
			containerRef.current.scrollLeft = 0;
		}
	};

	const handleNodeClick = (nodeId: string) => {
		setSelectedNode((prev) => (prev === nodeId ? null : nodeId));
		setSelectedLinkKey(null);
		setHoveredLinkKey(null);
	};

	if (links.length === 0) {
		return (
			<Card className={cn("rounded-lg border border-black", isFullscreen && "h-screen w-screen fixed inset-0 z-50 flex flex-col")}>
				<CardContent className={cn("p-6", isFullscreen && "flex-1 flex flex-col overflow-hidden")}>
					<div className="flex items-center gap-2 mb-4">
						<GitBranch className="w-4 h-4" />
						<span className="font-medium">Visitor Flows</span>
					</div>
					<p className="text-gray-500">
						No user flow data available for the selected time period.
					</p>
				</CardContent>
			</Card>
		);
	}

	const baseHeight = 1500;
	const scale = Math.max(zoom, 0.35);
	const containerHeight = isFullscreen ? Math.max(viewportHeight - 220, 480) : 600;
	const leftPadding = 150;
	const rightPadding = 180;
	const baseColumnSpacing = 280;
	const columnSpacing = Math.max(150, baseColumnSpacing * scale);
	const baseNodeWidth = 18;
	const nodeBodyWidth = Math.max(12, baseNodeWidth * Math.min(scale, 1.1));

	const effectiveHeight = baseHeight * zoom;
	const { nodes, links: sankeyLinks } = calculateSankeyLayout(links, columnSpacing, effectiveHeight);

	// Calculate actual number of columns
	const maxStep = nodes.length > 0 ? Math.max(...nodes.map(n => n.step)) : 1;
	const minStep = nodes.length > 0 ? Math.min(...nodes.map(n => n.step)) : 1;
	const numColumns = maxStep - minStep + 1;

	const baseWidth = leftPadding + Math.max(0, numColumns - 1) * columnSpacing + nodeBodyWidth + rightPadding;
	const effectiveWidth = baseWidth;

	const selectedLink = selectedLinkKey !== null
		? sankeyLinks.find((link) => linkKey(link) === selectedLinkKey) ?? null
		: null;

	const isLinkConnectedToNode = (link: SankeyLink, nodeId: string): boolean => {
		return link.source.id === nodeId || link.target.id === nodeId;
	};

	const isNodeConnectedToSelectedLink = (node: SankeyNode): boolean => {
		if (!selectedLink) return false;
		return node.id === selectedLink.source.id || node.id === selectedLink.target.id;
	};

	const content = (
		<Card className={cn("rounded-lg border border-black", isFullscreen && "h-screen w-screen fixed inset-0 z-50 flex flex-col")}>
			<CardContent className={cn("p-6", isFullscreen && "flex-1 flex flex-col overflow-hidden")}>
				<div className="mb-4 flex items-center justify-between gap-2">
					<div className="flex items-center gap-2">
						<GitBranch className="w-4 h-4" />
						<span className="font-medium">Visitor Flows</span>
						<span className="text-xs text-gray-500 ml-2">
							Entry pages → Navigation → Exit pages
						</span>
					</div>
					{!isFullscreen && (
						<button
							type="button"
							onClick={() => setIsFullscreen(true)}
							className="p-1.5 hover:bg-gray-100 rounded transition-colors"
							title="Expand Visitor Flows"
						>
							<Maximize2 className="w-4 h-4" />
						</button>
					)}
				</div>
				<div
					ref={containerRef}
					className={cn("relative overflow-auto border border-gray-200 rounded bg-white", isPanning ? "cursor-grabbing select-none" : "cursor-grab", isFullscreen && "flex-1")}
					style={!isFullscreen ? { height: `${containerHeight}px`, maxHeight: `${containerHeight}px` } : undefined}
				>
					{/* Controls */}
					<div className="absolute top-2 right-2 z-10 flex gap-1 bg-white/90 backdrop-blur-sm border border-gray-300 rounded-md shadow-sm p-1">
						<button
							type="button"
							onClick={() => setShowHelp(!showHelp)}
							className={cn("p-1.5 rounded transition-colors", showHelp ? "bg-blue-100 text-blue-700" : "hover:bg-gray-100")}
							title="How to use this chart"
						>
							<HelpCircle className="w-4 h-4" />
						</button>
						<div className="w-px bg-gray-300 mx-0.5" />
						<button
							type="button"
							onClick={() => setZoom((prev) => Math.min(2, prev * 1.2))}
							className="p-1.5 hover:bg-gray-100 rounded transition-colors"
							title="Zoom In"
						>
							<ZoomIn className="w-4 h-4" />
						</button>
						<button
							type="button"
							onClick={() => setZoom((prev) => Math.max(0.3, prev / 1.2))}
							className="p-1.5 hover:bg-gray-100 rounded transition-colors"
							title="Zoom Out"
						>
							<ZoomOut className="w-4 h-4" />
						</button>
						<button
							type="button"
							onClick={handleReset}
							className="p-1.5 hover:bg-gray-100 rounded transition-colors"
							title="Reset View"
						>
							<RotateCcw className="w-4 h-4" />
						</button>
						{isFullscreen && (
							<button
								type="button"
								onClick={() => setIsFullscreen(false)}
								className="p-1.5 hover:bg-gray-100 rounded transition-colors"
								title="Close (Esc)"
							>
								<X className="w-4 h-4" />
							</button>
						)}
					</div>

					{/* Help overlay */}
					{showHelp && (
						<div className="absolute top-14 right-2 z-20 w-64 bg-white border border-gray-300 rounded-lg shadow-lg p-4">
							<div className="flex items-center justify-between mb-3">
								<h4 className="font-semibold text-sm">How to Read This Chart</h4>
								<button type="button" onClick={() => setShowHelp(false)} className="p-1 hover:bg-gray-100 rounded">
									<X className="w-3 h-3" />
								</button>
							</div>
							<div className="space-y-3 text-xs text-gray-600">
								<div>
									<div className="font-medium text-gray-900 mb-1">What it shows</div>
									<p>How visitors navigate your site. Each bar is a page URL at a specific step in the journey.</p>
								</div>
								<div>
									<div className="font-medium text-gray-900 mb-1">Reading left to right</div>
									<p><strong>Left:</strong> Entry pages (where visitors land)<br/>
									<strong>Middle:</strong> Pages visited next<br/>
									<strong>Right:</strong> Later pages in the journey</p>
								</div>
								<div>
									<div className="font-medium text-gray-900 mb-1">Stream width</div>
									<p>Thicker streams = more visitors taking that path</p>
								</div>
								<div>
									<div className="font-medium text-gray-900 mb-1 flex items-center gap-1">
										<MousePointer className="w-3 h-3" /> Interactions
									</div>
									<ul className="list-disc list-inside space-y-0.5 pl-1">
										<li><strong>Click</strong> a stream to see full details below</li>
										<li><strong>Hover</strong> to preview visitor counts</li>
									</ul>
								</div>
								<div>
									<div className="font-medium text-gray-900 mb-1 flex items-center gap-1">
										<Move className="w-3 h-3" /> Navigation
									</div>
									<ul className="list-disc list-inside space-y-0.5 pl-1">
										<li><strong>Drag</strong> to pan around</li>
										<li><strong>+/-</strong> buttons to zoom</li>
									</ul>
								</div>
								<div>
									<div className="font-medium text-gray-900 mb-1">Color Legend</div>
									<div className="flex flex-wrap gap-1.5 mt-1">
										{STEP_COLORS.slice(0, 4).map((color, i) => (
											<div key={color} className="flex items-center gap-1">
												<div className="w-3 h-3 rounded" style={{ backgroundColor: color }} />
												<span className="text-[10px]">Step {i + 1}</span>
											</div>
										))}
									</div>
								</div>
							</div>
						</div>
					)}

					<svg
						ref={svgRef}
						width={effectiveWidth}
						height={effectiveHeight}
						viewBox={`0 0 ${effectiveWidth} ${effectiveHeight}`}
						onClick={(e) => {
							if (e.target === e.currentTarget) {
								setSelectedNode(null);
								setSelectedLinkKey(null);
								setHoveredNode(null);
								setHoveredLinkKey(null);
							}
						}}
					>
						{/* Step labels at top */}
						{Array.from(new Set(nodes.map(n => n.step))).sort((a, b) => a - b).map((step, idx) => {
							const stepNodes = nodes.filter(n => n.step === step);
							if (stepNodes.length === 0) return null;
							const x = stepNodes[0].x;
							return (
								<text
									key={`step-label-${step}`}
									x={x + nodeBodyWidth / 2}
									y={30}
									textAnchor="middle"
									className="text-xs font-medium"
									style={{ fontSize: "11px" }}
									fill="#6b7280"
								>
									{idx === 0 ? "Entry" : idx === numColumns - 1 ? "Exit" : `Step ${step}`}
								</text>
							);
						})}

						{/* Render links */}
						<g>
							{sankeyLinks.map((link, i) => {
								const color = getStepColor(link.source.step);
								const key = linkKey(link);
								const isSelected = selectedLinkKey === key;
								const isHovered = hoveredLinkKey === key;
								const activeNode = selectedNode || hoveredNode;

								let opacity = 0.5;
								if (isSelected || isHovered) {
									opacity = 0.85;
								} else if (selectedLinkKey && selectedLinkKey !== key) {
									opacity = 0.15;
								} else if (activeNode) {
									opacity = isLinkConnectedToNode(link, activeNode) ? 0.75 : 0.15;
								}

								return (
									<path
										key={`link-${i}`}
										d={generateLinkPath(link, nodeBodyWidth)}
										fill={color}
										fillOpacity={opacity}
										stroke={isSelected ? "#000" : isHovered ? "#374151" : "none"}
										strokeWidth={isSelected || isHovered ? 1.5 : 0}
										className="transition-all duration-150 cursor-pointer"
										onMouseEnter={() => setHoveredLinkKey(key)}
										onMouseLeave={() => setHoveredLinkKey((prev) => prev === key ? null : prev)}
										onClick={(e) => {
											e.stopPropagation();
											setSelectedLinkKey((prev) => prev === key ? null : key);
											setSelectedNode(null);
										}}
									>
										<title>
											{`${link.source.displayName} → ${link.target.displayName}\n${link.value.toLocaleString()} visitors`}
										</title>
									</path>
								);
							})}
						</g>

						{/* Render nodes */}
						<g>
							{nodes.map((node, i) => {
								const color = getStepColor(node.step);
								const isSelectedNode = selectedNode === node.id;
								const isHoveredNode = hoveredNode === node.id;
								const isConnectedToLink = selectedLink ? isNodeConnectedToSelectedLink(node) : false;
								const activeNode = selectedNode || hoveredNode;

								let nodeOpacity = 0.9;
								if (isSelectedNode || isHoveredNode || isConnectedToLink) {
									nodeOpacity = 1;
								} else if (selectedLinkKey && !isConnectedToLink) {
									nodeOpacity = 0.3;
								} else if (activeNode && activeNode !== node.id) {
									const isConnected = sankeyLinks.some(l =>
										(l.source.id === activeNode && l.target.id === node.id) ||
										(l.target.id === activeNode && l.source.id === node.id)
									);
									nodeOpacity = isConnected ? 0.9 : 0.3;
								}

								return (
									<g key={`node-${i}`}>
										<rect
											x={node.x}
											y={node.y}
											width={nodeBodyWidth}
											height={node.height}
											fill={color}
											stroke={isSelectedNode || isHoveredNode ? "#000" : "#fff"}
											strokeWidth={isSelectedNode || isHoveredNode ? 2 : 1}
											rx={3}
											className="transition-all cursor-pointer"
											style={{ opacity: nodeOpacity }}
											onMouseEnter={() => setHoveredNode(node.id)}
											onMouseLeave={() => setHoveredNode((prev) => prev === node.id ? null : prev)}
											onClick={(e) => {
												e.stopPropagation();
												handleNodeClick(node.id);
											}}
										>
											<title>{`${node.displayName}\n${node.value.toLocaleString()} visitors`}</title>
										</rect>

										{/* Node label */}
										{node.height > 16 && (
											<text
												x={node.x + nodeBodyWidth + 8}
												y={node.y + node.height / 2}
												textAnchor="start"
												alignmentBaseline="middle"
												className="text-xs font-medium"
												style={{ fontSize: "11px", pointerEvents: 'none', opacity: nodeOpacity }}
												fill="#1f2937"
											>
												{truncatePageName(node.displayName)}
											</text>
										)}

										{/* Visitor count */}
										{node.height > 35 && (
											<text
												x={node.x + nodeBodyWidth + 8}
												y={node.y + node.height / 2 + 14}
												textAnchor="start"
												alignmentBaseline="middle"
												className="text-xs"
												style={{ fontSize: "10px", pointerEvents: 'none', opacity: nodeOpacity }}
												fill="#6b7280"
											>
												{node.value.toLocaleString()} visitors
											</text>
										)}
									</g>
								);
							})}
						</g>
					</svg>
				</div>

				{/* Selection details panel */}
				<div className="mt-3">
					{selectedLink && (
						<div className="rounded-lg border border-blue-200 bg-gradient-to-r from-blue-50 to-indigo-50 px-4 py-3">
							<div className="flex items-center justify-between mb-2">
								<div className="flex items-center gap-2">
									<div className="w-2 h-2 rounded-full bg-blue-500" />
									<span className="font-semibold text-blue-900 text-sm">Selected Transition</span>
								</div>
								<div className="bg-white/70 rounded px-3 py-1">
									<span className="font-bold text-blue-600 text-lg">{selectedLink.value.toLocaleString()}</span>
									<span className="ml-1 text-blue-500 text-sm">visitors</span>
								</div>
							</div>
							<div className="bg-white/50 rounded-lg p-3 space-y-2">
								<div className="flex items-start gap-2">
									<span className="text-blue-400 text-xs font-medium shrink-0 w-14">From:</span>
									<span className="font-medium text-blue-800 break-all">{selectedLink.source.displayName}</span>
								</div>
								<div className="flex items-start gap-2">
									<span className="text-blue-400 text-xs font-medium shrink-0 w-14">To:</span>
									<span className="font-medium text-blue-800 break-all">{selectedLink.target.displayName}</span>
								</div>
							</div>
						</div>
					)}
					{selectedNode && !selectedLink && (
						<div className="rounded-lg border border-green-200 bg-gradient-to-r from-green-50 to-emerald-50 px-4 py-3">
							<div className="flex items-center gap-2 mb-2">
								<div className="w-2 h-2 rounded-full bg-green-500" />
								<span className="font-semibold text-green-900 text-sm">Selected Page</span>
							</div>
							<div className="text-green-800 font-medium text-lg mb-2">
								{nodes.find(n => n.id === selectedNode)?.displayName || selectedNode}
							</div>
							{(() => {
								const node = nodes.find(n => n.id === selectedNode);
								if (!node) return null;

								const incomingLinks = sankeyLinks.filter(l => l.target.id === selectedNode);
								const outgoingLinks = sankeyLinks.filter(l => l.source.id === selectedNode);

								return (
									<div className="grid grid-cols-3 gap-3 text-xs">
										<div className="bg-white/70 rounded px-2 py-1.5">
											<div className="text-green-600 font-semibold text-base">{node.value.toLocaleString()}</div>
											<div className="text-green-500">total visitors</div>
										</div>
										{incomingLinks.length > 0 && (
											<div className="bg-white/70 rounded px-2 py-1.5">
												<div className="text-green-600 font-semibold text-base">{incomingLinks.length}</div>
												<div className="text-green-500">source pages</div>
											</div>
										)}
										{outgoingLinks.length > 0 && (
											<div className="bg-white/70 rounded px-2 py-1.5">
												<div className="text-green-600 font-semibold text-base">{outgoingLinks.length}</div>
												<div className="text-green-500">destinations</div>
											</div>
										)}
										{incomingLinks.length === 0 && (
											<div className="bg-blue-100/50 rounded px-2 py-1.5 border border-blue-200">
												<div className="text-blue-700 font-medium">Entry Page</div>
												<div className="text-blue-500 text-[10px]">Visitors land here first</div>
											</div>
										)}
										{outgoingLinks.length === 0 && (
											<div className="bg-purple-100/50 rounded px-2 py-1.5 border border-purple-200">
												<div className="text-purple-700 font-medium">Exit Page</div>
												<div className="text-purple-500 text-[10px]">Last page visited</div>
											</div>
										)}
									</div>
								);
							})()}
						</div>
					)}
					{!selectedLink && !selectedNode && (
						<div className="flex items-center gap-3 text-xs text-gray-500 px-1">
							<div className="flex items-center gap-1.5">
								<MousePointer className="w-3 h-3" />
								<span>Click streams or pages for details</span>
							</div>
							<div className="flex items-center gap-1.5">
								<Move className="w-3 h-3" />
								<span>Drag to pan</span>
							</div>
							<button
								type="button"
								onClick={() => setShowHelp(true)}
								className="flex items-center gap-1 text-blue-600 hover:text-blue-700 font-medium"
							>
								<HelpCircle className="w-3 h-3" />
								<span>Help</span>
							</button>
						</div>
					)}
				</div>
			</CardContent>
		</Card>
	);

	return content;
};
