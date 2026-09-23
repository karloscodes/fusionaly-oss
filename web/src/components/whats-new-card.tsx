import { Link } from "@inertiajs/react";
import { Card, CardContent } from "@/components/ui/card";
import { itemTypeGlyphs, feedTime } from "@/components/feed-item-style";
import { cn } from "@/lib/utils";

export interface WhatsNewItem {
	id: number;
	itemType: string;
	title: string;
	description: string;
	detectedAt: string;
}

// This site's latest activity feed items, next to the chart. The full,
// cross-site feed lives on Home.
export const WhatsNewCard = ({ items }: { items: WhatsNewItem[] }) => (
	<Card className="rounded-xl border border-black">
		<CardContent className="p-4 sm:p-5 flex flex-col h-full">
			<div className="flex items-baseline gap-2.5 mb-2">
				<h2 className="text-base font-semibold text-gray-900">What's new</h2>
				<span className="font-mono text-[11px] text-gray-500">past 7 days</span>
			</div>

			{items.length === 0 ? (
				<p className="text-sm text-gray-500 py-4">
					Nothing new this week. Small sites stay quiet until something real happens.
				</p>
			) : (
				<ol className="flex flex-col">
					{items.map((item, i) => {
						const g = itemTypeGlyphs[item.itemType] || { glyph: "•", tone: "text-gray-900" };
						return (
							<li
								key={item.id}
								className={cn("grid grid-cols-[24px_1fr_auto] gap-3 py-2.5 items-start", i > 0 && "border-t border-gray-200")}
							>
								<span
									className={cn("w-6 h-6 rounded flex items-center justify-center font-mono text-xs font-bold bg-[var(--databar)]", g.tone)}
									aria-hidden="true"
								>
									{g.glyph}
								</span>
								<p className="text-[13.5px] leading-snug text-gray-500">
									<strong className="font-semibold text-gray-900">{item.title}</strong> {item.description}
								</p>
								<time className="font-mono text-[11px] text-gray-500 whitespace-nowrap" dateTime={item.detectedAt}>
									{feedTime(item.detectedAt)}
								</time>
							</li>
						);
					})}
				</ol>
			)}

			<Link href="/admin" className="mt-auto pt-3 text-[13px] font-medium text-gray-900 underline decoration-[rgb(var(--c-accent))] underline-offset-4">
				Open the activity feed
			</Link>
		</CardContent>
	</Card>
);
