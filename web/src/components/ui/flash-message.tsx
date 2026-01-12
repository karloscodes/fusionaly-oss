import { Alert, AlertDescription } from "@/components/ui/alert";
import type { FlashMessage } from "@/types";

interface FlashMessageDisplayProps {
  flash?: FlashMessage | null;
  error?: string | null;
  className?: string;
  showSuccessMessage?: boolean;
  successMessage?: string;
}

export function FlashMessageDisplay({
  flash,
  error,
  className = "mb-4",
  showSuccessMessage = false,
  successMessage = "Operation completed successfully!"
}: FlashMessageDisplayProps) {
  // Don't render if no flash message, error, or success message
  if (!flash?.message && !error && !showSuccessMessage) {
    return null;
  }

  return (
    <div className={`space-y-3 ${className}`}>
      {/* Flash message */}
      {flash?.message && (
        <Alert
          variant={flash.type === "error" ? "destructive" : "default"}
        >
          <AlertDescription>{flash.message}</AlertDescription>
        </Alert>
      )}

      {/* Error message (only show if no flash message) */}
      {error && !flash?.message && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Success message */}
      {showSuccessMessage && (
        <Alert>
          <AlertDescription>{successMessage}</AlertDescription>
        </Alert>
      )}
    </div>
  );
}
