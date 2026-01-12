import { useState } from "react";
import { format } from "date-fns";
import { useForm, router } from "@inertiajs/react";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
	DialogFooter,
	DialogClose,
	DialogDescription,
} from "@/components/ui/dialog";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Calendar } from "@/components/ui/calendar";
import { Pencil, Trash2, Flag, Calendar as CalendarIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import type { Annotation } from "@/types";

interface AnnotationManagerProps {
	websiteId: number;
	initialDate?: string; // ISO date string for pre-filling when clicked from chart
	open?: boolean;
	onOpenChange?: (open: boolean) => void;
}

interface AnnotationFormProps {
	websiteId: number;
	annotation?: Annotation;
	onClose: () => void;
	initialDate?: string; // ISO date string for pre-filling
}

// Exported for use in chart annotation clicks
export interface AnnotationDetailDialogProps {
	websiteId: number;
	annotation: Annotation;
	open: boolean;
	onOpenChange: (open: boolean) => void;
	readOnly?: boolean; // Hide edit/delete buttons (for public view)
}

export const AnnotationForm = ({ websiteId, annotation, onClose, initialDate }: AnnotationFormProps) => {
	// Determine the initial date: annotation > initialDate prop > now
	const getInitialDate = () => {
		if (annotation?.annotation_date) {
			return new Date(annotation.annotation_date);
		}
		if (initialDate) {
			return new Date(initialDate);
		}
		return new Date();
	};

	const initialDateObj = getInitialDate();
	const [selectedDate, setSelectedDate] = useState<Date>(initialDateObj);
	const [hours, setHours] = useState(initialDateObj.getHours().toString().padStart(2, "0"));
	const [minutes, setMinutes] = useState(initialDateObj.getMinutes().toString().padStart(2, "0"));

	const isEditing = !!annotation;
	const formAction = isEditing
		? `/admin/websites/${websiteId}/annotations/${annotation.id}`
		: `/admin/websites/${websiteId}/annotations`;

	const form = useForm({
		title: annotation?.title || "",
		description: annotation?.description || "",
		color: annotation?.color || "#f97316",
		annotation_type: "general",
		annotation_date: "",
	});

	// Combine date and time for the hidden form field
	const getAnnotationDateTime = () => {
		const dateTime = new Date(selectedDate);
		dateTime.setHours(parseInt(hours), parseInt(minutes), 0, 0);
		return dateTime.toISOString().slice(0, 16);
	};

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		form.setData("annotation_date", getAnnotationDateTime());
		form.post(formAction, {
			onSuccess: () => onClose(),
		});
	};

	// Generate hour and minute options
	const hourOptions = Array.from({ length: 24 }, (_, i) => i.toString().padStart(2, "0"));
	const minuteOptions = Array.from({ length: 12 }, (_, i) => (i * 5).toString().padStart(2, "0"));

	return (
		<form onSubmit={handleSubmit} className="space-y-4">

			<div className="space-y-2">
				<Label htmlFor="title">Title *</Label>
				<Input
					id="title"
					value={form.data.title}
					onChange={(e) => form.setData("title", e.target.value)}
					placeholder="e.g., v2.0 Release, Marketing Campaign"
					required
				/>
				{form.errors.title && (
					<p className="text-sm text-red-500">{form.errors.title}</p>
				)}
			</div>

			<div className="space-y-2">
				<Label htmlFor="description">Description</Label>
				<Textarea
					id="description"
					value={form.data.description}
					onChange={(e) => form.setData("description", e.target.value)}
					placeholder="Optional description..."
					rows={3}
				/>
			</div>

			<div className="space-y-2">
				<Label>Date & Time *</Label>
				<Popover>
					<PopoverTrigger asChild>
						<Button
							variant="outline"
							className={cn(
								"w-full justify-start text-left font-normal",
								!selectedDate && "text-muted-foreground"
							)}
						>
							<CalendarIcon className="mr-2 h-4 w-4" />
							{selectedDate ? (
								<span>{format(selectedDate, "PPP")} at {hours}:{minutes}</span>
							) : (
								<span>Pick date and time</span>
							)}
						</Button>
					</PopoverTrigger>
					<PopoverContent className="w-auto p-0" align="start">
						<Calendar
							mode="single"
							selected={selectedDate}
							onSelect={(date) => date && setSelectedDate(date)}
							initialFocus
						/>
						<div className="border-t p-3">
							<div className="flex items-center justify-center gap-2">
								<select
									value={hours}
									onChange={(e) => setHours(e.target.value)}
									className="h-9 rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
								>
									{hourOptions.map((h) => (
										<option key={h} value={h}>{h}</option>
									))}
								</select>
								<span className="text-lg font-medium">:</span>
								<select
									value={minutes}
									onChange={(e) => setMinutes(e.target.value)}
									className="h-9 rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
								>
									{minuteOptions.map((m) => (
										<option key={m} value={m}>{m}</option>
									))}
								</select>
							</div>
						</div>
					</PopoverContent>
				</Popover>
			</div>

			<div className="space-y-2">
				<Label htmlFor="color">Color</Label>
				<div className="flex items-center gap-2">
					<Input
						id="color"
						name="color"
						type="color"
						value={form.data.color}
						onChange={(e) => form.setData("color", e.target.value)}
						className="w-12 h-10 p-1 cursor-pointer"
					/>
					<Input
						value={form.data.color}
						onChange={(e) => form.setData("color", e.target.value)}
						className="flex-1"
						placeholder="#6366f1"
					/>
				</div>
			</div>

			<DialogFooter className="gap-2 sm:gap-0">
				<DialogClose asChild>
					<Button type="button" variant="outline" onClick={onClose}>
						Cancel
					</Button>
				</DialogClose>
				<Button type="submit" disabled={form.processing}>
					{form.processing ? "Saving..." : (isEditing ? "Update" : "Create") + " Annotation"}
				</Button>
			</DialogFooter>
		</form>
	);
};

// Dialog component for viewing/editing/deleting an annotation (triggered from chart clicks)
export const AnnotationDetailDialog = ({
	websiteId,
	annotation,
	open,
	onOpenChange,
	readOnly = false,
}: AnnotationDetailDialogProps) => {
	const [isEditing, setIsEditing] = useState(false);
	const [confirmDelete, setConfirmDelete] = useState(false);
	const [isDeleting, setIsDeleting] = useState(false);

	const formatDate = (dateString: string) => {
		const date = new Date(dateString);
		return date.toLocaleDateString(undefined, {
			weekday: "long",
			year: "numeric",
			month: "long",
			day: "numeric",
			hour: "2-digit",
			minute: "2-digit",
		});
	};

	const handleClose = () => {
		setIsEditing(false);
		setConfirmDelete(false);
		onOpenChange(false);
	};

	const handleDeleteClick = () => {
		if (confirmDelete) {
			setIsDeleting(true);
			router.post(`/admin/websites/${websiteId}/annotations/${annotation.id}/delete`, {}, {
				onSuccess: () => handleClose(),
				onFinish: () => setIsDeleting(false),
			});
		} else {
			setConfirmDelete(true);
		}
	};

	return (
		<Dialog open={open} onOpenChange={(isOpen) => {
			if (!isOpen) {
				setIsEditing(false);
				setConfirmDelete(false);
			}
			onOpenChange(isOpen);
		}}>
			<DialogContent className="sm:max-w-md">
				{!isEditing ? (
					<>
						<DialogHeader>
							<div className="flex items-center gap-3">
								<div
									className="w-3 h-3 rounded-full flex-shrink-0"
									style={{ backgroundColor: annotation.color }}
								/>
								<DialogTitle className="text-lg">{annotation.title}</DialogTitle>
							</div>
							<DialogDescription className="sr-only">
								Annotation details for {annotation.title}
							</DialogDescription>
						</DialogHeader>
						<div className="space-y-4 py-2">
							{annotation.description && (
								<p className="text-sm text-gray-600 leading-relaxed">
									{annotation.description}
								</p>
							)}
							<div className="flex items-center gap-2 text-sm text-gray-500">
								<CalendarIcon className="w-4 h-4" />
								<span>{formatDate(annotation.annotation_date)}</span>
							</div>
						</div>
						<DialogFooter className={readOnly ? "justify-end" : "flex-row justify-between sm:justify-between gap-2"}>
							{!readOnly && (
								<Button
									variant={confirmDelete ? "destructive" : "ghost"}
									size="sm"
									className={confirmDelete ? "" : "text-red-600 hover:text-red-700 hover:bg-red-50"}
									onClick={handleDeleteClick}
									disabled={isDeleting}
								>
									<Trash2 className="w-4 h-4 mr-1.5" />
									{isDeleting ? "Deleting..." : confirmDelete ? "Are you sure?" : "Delete"}
								</Button>
							)}
							<div className="flex gap-2">
								<Button
									variant="outline"
									size="sm"
									onClick={handleClose}
								>
									Close
								</Button>
								{!readOnly && (
									<Button
										size="sm"
										onClick={() => setIsEditing(true)}
									>
										<Pencil className="w-4 h-4 mr-1.5" />
										Edit
									</Button>
								)}
							</div>
						</DialogFooter>
					</>
				) : (
					<>
						<DialogHeader>
							<DialogTitle>Edit Annotation</DialogTitle>
							<DialogDescription className="sr-only">
								Edit the annotation details
							</DialogDescription>
						</DialogHeader>
						<AnnotationForm
							websiteId={websiteId}
							annotation={annotation}
							onClose={handleClose}
						/>
					</>
				)}
			</DialogContent>
		</Dialog>
	);
};

// Simple "Add Annotation" button component with optional external control
export const AnnotationManager = ({ websiteId, initialDate, open, onOpenChange }: AnnotationManagerProps) => {
	const [internalOpen, setInternalOpen] = useState(false);

	// Use controlled state if provided, otherwise use internal state
	const isOpen = open !== undefined ? open : internalOpen;
	const setIsOpen = onOpenChange || setInternalOpen;

	return (
		<Dialog open={isOpen} onOpenChange={setIsOpen}>
			<DialogTrigger asChild>
				<Button variant="outline" size="sm" className="gap-1">
					<Flag className="w-4 h-4" />
					Add Annotation
				</Button>
			</DialogTrigger>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Add Timeline Annotation</DialogTitle>
					<DialogDescription className="sr-only">
						Create a new annotation for your timeline
					</DialogDescription>
				</DialogHeader>
				<AnnotationForm
					websiteId={websiteId}
					onClose={() => setIsOpen(false)}
					initialDate={initialDate}
				/>
			</DialogContent>
		</Dialog>
	);
};

export default AnnotationManager;
