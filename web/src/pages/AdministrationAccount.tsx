import { useEffect } from "react";
import type { FC } from "react";
import { usePage, useForm } from "@inertiajs/react";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { FlashMessageDisplay } from "@/components/ui/flash-message";
import { Input } from "@/components/ui/input";
import { Key } from "lucide-react";
import type { FlashMessage } from "@/types";
import { AdministrationLayout } from "@/components/administration-layout";

interface AdministrationAccountProps {
	flash?: FlashMessage;
	error?: string;
	[key: string]: unknown;
}

// Exported for Pro to wrap with its own layout
export const AdministrationAccountContent: FC = () => {
	const { props } = usePage<AdministrationAccountProps>();
	const { flash, error } = props;

	// Password change form
	const passwordForm = useForm({
		current_password: "",
		new_password: "",
		confirm_password: "",
	});

	// Handle password form submission
	const handlePasswordSubmit = (e: React.FormEvent) => {
		e.preventDefault();

		if (passwordForm.data.new_password !== passwordForm.data.confirm_password) {
			passwordForm.setError("confirm_password", "Passwords do not match");
			return;
		}

		if (passwordForm.data.new_password.length < 8) {
			passwordForm.setError("new_password", "Password must be at least 8 characters long");
			return;
		}

		passwordForm.post("/admin/account/change-password", {
			preserveScroll: true,
			onSuccess: () => {
				passwordForm.reset();
			},
		});
	};

	// Clear form errors on input change
	useEffect(() => {
		if (passwordForm.errors.confirm_password || passwordForm.errors.new_password) {
			passwordForm.clearErrors();
		}
	}, [passwordForm.data.new_password, passwordForm.data.confirm_password]);

	return (
		<div className="space-y-6">
			<div>
				<h1 className="text-2xl font-bold text-gray-900">Account Settings</h1>
				<p className="text-gray-600 mt-1">
					Manage your password
				</p>
			</div>

			<FlashMessageDisplay flash={flash} error={error} />

			{/* Password Change Section */}
			<Card className="border-black shadow-sm">
				<CardHeader className="pb-4">
					<div className="flex justify-between items-center">
						<CardTitle className="text-lg flex items-center gap-2">
							<Key className="h-5 w-5" /> Change Password
						</CardTitle>
					</div>
					<CardDescription>
						Update your account password for secure access.
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4">
					<form onSubmit={handlePasswordSubmit} className="space-y-4">
						<div className="space-y-2">
							<label className="text-sm font-medium text-gray-700">
								Current Password
							</label>
							<Input
								type="password"
								value={passwordForm.data.current_password}
								onChange={(e) => passwordForm.setData("current_password", e.target.value)}
								required
								className="w-full"
								disabled={passwordForm.processing}
							/>
							{passwordForm.errors.current_password && (
								<p className="text-sm text-red-600">{passwordForm.errors.current_password}</p>
							)}
						</div>

						<div className="space-y-2">
							<label className="text-sm font-medium text-gray-700">
								New Password
							</label>
							<Input
								type="password"
								value={passwordForm.data.new_password}
								onChange={(e) => passwordForm.setData("new_password", e.target.value)}
								required
								minLength={8}
								className="w-full"
								disabled={passwordForm.processing}
							/>
							<p className="text-xs text-gray-500">
								Minimum 8 characters required.
							</p>
							{passwordForm.errors.new_password && (
								<p className="text-sm text-red-600">{passwordForm.errors.new_password}</p>
							)}
						</div>

						<div className="space-y-2">
							<label className="text-sm font-medium text-gray-700">
								Confirm New Password
							</label>
							<Input
								type="password"
								value={passwordForm.data.confirm_password}
								onChange={(e) => passwordForm.setData("confirm_password", e.target.value)}
								required
								minLength={8}
								className="w-full"
								disabled={passwordForm.processing}
							/>
							{passwordForm.errors.confirm_password && (
								<p className="text-sm text-red-600">{passwordForm.errors.confirm_password}</p>
							)}
						</div>

						<Button
							type="submit"
							disabled={passwordForm.processing}
							className="bg-black hover:bg-gray-800 text-white rounded-md"
						>
							{passwordForm.processing ? "Updating..." : "Update Password"}
						</Button>
					</form>
				</CardContent>
			</Card>
		</div>
	);
};

// Default export wraps content with OSS layout (unchanged behavior)
export const AdministrationAccount: FC = () => (
	<AdministrationLayout currentPage="account">
		<AdministrationAccountContent />
	</AdministrationLayout>
);
