import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Formats numbers to a compact, human-readable format
 * Examples: 4,103 -> 4k, 1,200,000 -> 1M, 50 -> 50
 */
export function formatNumber(value: number): string {
  if (value < 1000) {
    return value.toString();
  }

  if (value < 1000000) {
    return Math.round(value / 1000) + 'k';
  }

  if (value < 1000000000) {
    return Math.round(value / 1000000) + 'M';
  }

  return Math.round(value / 1000000000) + 'B';
}
