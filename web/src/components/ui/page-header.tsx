interface PageHeaderProps {
	title: string;
	description?: string; // one muted line under the title
	rightContent?: React.ReactNode;
	leftContent?: React.ReactNode;
}

// Page title as in the dashboard design: bold title, a muted line under it,
// actions on the right.
export function PageHeader({ title, description, rightContent, leftContent }: PageHeaderProps) {
	return (
		<div className="flex flex-wrap justify-between items-end gap-4 mb-4">
			<div className="flex items-center gap-2.5">
				{leftContent && <div className="flex items-center">{leftContent}</div>}
				<div>
					<h1 className="text-2xl font-bold text-gray-900">{title}</h1>
					{description && <p className="text-sm text-gray-500 mt-1">{description}</p>}
				</div>
			</div>
			{rightContent && <div className="flex items-center gap-3">{rightContent}</div>}
		</div>
	);
}
