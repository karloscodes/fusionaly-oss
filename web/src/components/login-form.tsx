import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FlashMessageDisplay } from "@/components/ui/flash-message";
import { useForm } from "@inertiajs/react";
import type { FlashMessage } from "@/types";

interface LoginFormProps extends React.ComponentPropsWithoutRef<"div"> {
	flash?: FlashMessage;
	error?: string;
}

export function LoginForm({
	className,
	flash,
	error,
	...props
}: LoginFormProps) {
	const form = useForm({
		email: "",
		password: "",
		_tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
	});

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		form.post("/login");
	};

	return (
		<div className={cn("flex flex-col gap-6", className)} {...props}>
			<Card>
				<CardHeader>
					<CardTitle className="text-2xl">Login</CardTitle>
					<CardDescription>
						Enter your email below to login to your account
					</CardDescription>
				</CardHeader>
				<CardContent>
					<FlashMessageDisplay
						flash={flash}
						error={error}
						className="mb-6"
					/>

					<form onSubmit={handleSubmit}>
						<div className="flex flex-col gap-6">
							<div className="grid gap-2">
								<Label htmlFor="email">Email</Label>
								<Input
									id="email"
									name="email"
									type="email"
									placeholder="me@example.com"
									value={form.data.email}
									onChange={(e) => form.setData("email", e.target.value)}
									required
								/>
								{form.errors.email && (
									<p className="text-sm text-red-500">{form.errors.email}</p>
								)}
							</div>
							<div className="grid gap-2">
								<div className="flex items-center">
									<Label htmlFor="password">Password</Label>
									<a
										href="#"
										className="ml-auto inline-block text-sm underline-offset-4 hover:underline"
									>
										Forgot your password?
									</a>
								</div>
								<Input
									id="password"
									name="password"
									type="password"
									value={form.data.password}
									onChange={(e) => form.setData("password", e.target.value)}
									required
								/>
								{form.errors.password && (
									<p className="text-sm text-red-500">{form.errors.password}</p>
								)}
							</div>
							<Button type="submit" className="w-full" disabled={form.processing}>
								{form.processing ? "Logging in..." : "Login"}
							</Button>
						</div>
					</form>
				</CardContent>
			</Card>
		</div>
	);
}
