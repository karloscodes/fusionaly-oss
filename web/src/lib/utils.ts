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

/**
 * Returns the short symbol of a currency code: "$" for USD, "€" for EUR.
 * A symbol of letters (CHF) gets a space, so it does not touch the number.
 * An unknown code returns the code itself.
 */
export function currencySymbol(currency = "USD"): string {
  try {
    const parts = new Intl.NumberFormat(undefined, {
      style: "currency",
      currency,
      currencyDisplay: "narrowSymbol",
    }).formatToParts(0);
    const symbol = parts.find((part) => part.type === "currency")?.value ?? currency;
    return /^[A-Za-z]+$/.test(symbol) ? `${symbol} ` : symbol;
  } catch {
    return `${currency} `;
  }
}

/**
 * Formats an amount with its currency, in the locale of the browser.
 * Example: 99, "USD" -> "$99.00"
 */
export function formatMoney(amount: number, currency: string): string {
  try {
    return new Intl.NumberFormat(undefined, { style: "currency", currency }).format(amount);
  } catch {
    return `${amount.toFixed(2)} ${currency}`;
  }
}
