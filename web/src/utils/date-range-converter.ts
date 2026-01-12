// Utility to convert predefined time ranges to explicit from/to dates
// This implements Option 1: homogenize parameters by converting all ranges to from/to format

export interface DateRange {
  from: string; // YYYY-MM-DD format
  to: string;   // YYYY-MM-DD format
}

export function convertRangeToDateRange(rangeValue: string): DateRange {
  const now = new Date();

  // Helper to format date to YYYY-MM-DD
  const formatDate = (date: Date): string => {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  };

  // Helper to get start of day
  const startOfDay = (date: Date): Date => {
    const result = new Date(date);
    result.setHours(0, 0, 0, 0);
    return result;
  };

  // Helper to get end of day
  const endOfDay = (date: Date): Date => {
    const result = new Date(date);
    result.setHours(23, 59, 59, 999);
    return result;
  };

  switch (rangeValue) {
    case 'today': {
      const today = startOfDay(now);
      return {
        from: formatDate(today),
        to: formatDate(today)
      };
    }

    case 'yesterday': {
      const yesterday = new Date(now);
      yesterday.setDate(now.getDate() - 1);
      const yesterdayStart = startOfDay(yesterday);
      return {
        from: formatDate(yesterdayStart),
        to: formatDate(yesterdayStart)
      };
    }

    case 'last_7_days': {
      const sevenDaysAgo = new Date(now);
      sevenDaysAgo.setDate(now.getDate() - 7);
      const from = startOfDay(sevenDaysAgo);
      const to = endOfDay(now);
      return {
        from: formatDate(from),
        to: formatDate(to)
      };
    }

    case 'last_30_days': {
      const thirtyDaysAgo = new Date(now);
      thirtyDaysAgo.setDate(now.getDate() - 30);
      const from = startOfDay(thirtyDaysAgo);
      const to = endOfDay(now);
      return {
        from: formatDate(from),
        to: formatDate(to)
      };
    }

    case 'last_90_days': {
      const ninetyDaysAgo = new Date(now);
      ninetyDaysAgo.setDate(now.getDate() - 90);
      const from = startOfDay(ninetyDaysAgo);
      const to = endOfDay(now);
      return {
        from: formatDate(from),
        to: formatDate(to)
      };
    }

    case 'month_to_date': {
      const firstOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
      const from = startOfDay(firstOfMonth);
      const to = endOfDay(now);
      return {
        from: formatDate(from),
        to: formatDate(to)
      };
    }

    case 'last_month': {
      const lastMonth = new Date(now.getFullYear(), now.getMonth() - 1, 1);
      const firstOfLastMonth = startOfDay(lastMonth);

      // Last day of last month
      const lastDayOfLastMonth = new Date(now.getFullYear(), now.getMonth(), 0);
      const endOfLastMonth = endOfDay(lastDayOfLastMonth);

      return {
        from: formatDate(firstOfLastMonth),
        to: formatDate(endOfLastMonth)
      };
    }

    case 'year_to_date': {
      const firstOfYear = new Date(now.getFullYear(), 0, 1);
      const from = startOfDay(firstOfYear);
      const to = endOfDay(now);
      return {
        from: formatDate(from),
        to: formatDate(to)
      };
    }

    case 'last_12_months': {
      const twelveMonthsAgo = new Date(now);
      twelveMonthsAgo.setFullYear(now.getFullYear() - 1);
      const from = startOfDay(twelveMonthsAgo);
      const to = endOfDay(now);
      return {
        from: formatDate(from),
        to: formatDate(to)
      };
    }

    case 'all_time': {
      const fiveYearsAgo = new Date(now);
      fiveYearsAgo.setFullYear(now.getFullYear() - 5);
      const from = startOfDay(fiveYearsAgo);
      const to = endOfDay(now);
      return {
        from: formatDate(from),
        to: formatDate(to)
      };
    }

    default: {
      // Default to last 7 days
      const sevenDaysAgo = new Date(now);
      sevenDaysAgo.setDate(now.getDate() - 7);
      const from = startOfDay(sevenDaysAgo);
      const to = endOfDay(now);
      return {
        from: formatDate(from),
        to: formatDate(to)
      };
    }
  }
}

// Helper to determine if current URL has custom date range
export function isCustomDateRange(): boolean {
  const searchParams = new URLSearchParams(window.location.search);
  return !!(searchParams.get('from') && searchParams.get('to'));
}

// Helper to get current date range from URL
export function getCurrentDateRangeFromUrl(): DateRange | null {
  const searchParams = new URLSearchParams(window.location.search);
  const from = searchParams.get('from');
  const to = searchParams.get('to');

  if (from && to) {
    return { from, to };
  }

  return null;
}

// Helper to get current range parameter from URL
export function getCurrentRangeFromUrl(): string | null {
  const searchParams = new URLSearchParams(window.location.search);
  return searchParams.get('range');
}

// Helper to identify which predefined range matches a given date range
export function identifyRangeFromDates(from: string, to: string): string | null {
  const predefinedRanges = [
    'today',
    'yesterday',
    'last_7_days',
    'last_30_days',
    'last_90_days',
    'month_to_date',
    'last_month',
    'year_to_date',
    'last_12_months',
    'all_time'
  ];

  for (const range of predefinedRanges) {
    const expectedDateRange = convertRangeToDateRange(range);
    if (expectedDateRange.from === from && expectedDateRange.to === to) {
      return range;
    }
  }

  return null; // No matching predefined range found
} 
