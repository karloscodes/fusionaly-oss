import React from "react";
import { cn } from "@/lib/utils";

interface LoadingPulseProps {
  message?: string;
  color?: "green" | "blue" | "gray";
  size?: "sm" | "md" | "lg";
  className?: string;
}

export const LoadingPulse: React.FC<LoadingPulseProps> = ({
  message,
  color = "green",
  size = "md",
  className,
}) => {
  const colorClasses = {
    green: "bg-green-500",
    blue: "bg-blue-500",
    gray: "bg-black/50",
  };

  const sizeClasses = {
    sm: "w-2 h-2",
    md: "w-3 h-3",
    lg: "w-4 h-4",
  };

  const textSizeClasses = {
    sm: "text-xs",
    md: "text-sm",
    lg: "text-base",
  };

  return (
    <div className={cn("flex items-center gap-3", className)}>
      <div className="relative flex items-center justify-center">
        {/* Outer pulsing ring */}
        <div
          className={cn(
            "absolute rounded-full animate-ping opacity-75",
            colorClasses[color],
            size === "sm" ? "w-3 h-3" : size === "md" ? "w-4 h-4" : "w-5 h-5"
          )}
        />
        {/* Inner solid dot */}
        <div
          className={cn(
            "relative rounded-full",
            colorClasses[color],
            sizeClasses[size]
          )}
        />
      </div>
      {message && (
        <span className={cn("font-medium text-black/70", textSizeClasses[size])}>
          {message}
        </span>
      )}
    </div>
  );
};
