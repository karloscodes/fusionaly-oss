import { ComponentType } from "react";

interface PageHeaderProps {
	title: string;
	icon: ComponentType<{ className?: string }>;
	rightContent?: React.ReactNode;
	leftContent?: React.ReactNode;
}

export function PageHeader({
	title,
	icon: Icon,
	rightContent,
	leftContent,
}: PageHeaderProps) {
	return (
		<div className="flex flex-wrap justify-between items-center gap-4 mb-4">
			<div className="flex items-center gap-2.5">
				{leftContent && (
					<div className="flex items-center">{leftContent}</div>
				)}
				<Icon className="h-6 w-6 text-black" />
				<h1 className="text-2xl font-bold text-black">{title}</h1>
			</div>
			{rightContent && (
				<div className="flex items-center gap-4">{rightContent}</div>
			)}
		</div>
	);
}
