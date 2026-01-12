import * as React from "react"
import { Calendar as CalendarIcon } from "lucide-react"
import { DateRange } from "react-day-picker"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import { useEffect } from "react"

export interface DateRangePickerProps extends React.HTMLAttributes<HTMLDivElement> {
  /** The initial date range */
  initialDateRange?: DateRange
  /** Callback when date range changes */
  onRangeChange?: (range: DateRange | undefined) => void
  /** Callback when range is applied */
  onRangeApply?: (range: DateRange) => void
  /** Whether the picker is disabled */
  disabled?: boolean
  /** Number of months to display */
  numberOfMonths?: number
  /** Custom placeholder text */
  placeholder?: string
  /** Error message */
  error?: string
  /** Custom format for the display date */
  dateDisplayFormat?: string
}

export function DateRangePicker({
  initialDateRange,
  onRangeChange,
  onRangeApply,
  disabled = false,
  numberOfMonths = 2,
  placeholder = "Select date range",
  error,
  className,
  ...props
}: DateRangePickerProps) {
  const [date, setDate] = React.useState<DateRange | undefined>(initialDateRange)

  // Update internal state when initialDateRange changes
  useEffect(() => {
    if (initialDateRange !== undefined) {
      setDate(initialDateRange)
    }
  }, [initialDateRange])

  const handleDateChange = (newDate: DateRange | undefined) => {
    setDate(newDate)
    onRangeChange?.(newDate)
  }

  const handleApply = () => {
    if (date?.from && date?.to) {
      onRangeApply?.(date)
    }
  }

  const formatDateRange = (range: DateRange | undefined): string => {
    if (!range?.from) return placeholder

    const formatter = new Intl.DateTimeFormat('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    })

    if (!range.to) return formatter.format(range.from)
    return `${formatter.format(range.from)} - ${formatter.format(range.to)}`
  }

  const disableAfter = new Date();

  return (
    <div className={cn("grid gap-2", className)} {...props}>
      <Button // The button is still useful for triggering the dialog if needed
        id="date-range-picker"
        variant="outline"
        size="sm"
        className={cn(
          "w-full justify-start text-left font-normal",
          error && "border-red-500",
          !date && "text-muted-foreground",
          disabled && "opacity-50 cursor-not-allowed"
        )}
        disabled={true}
      >
        <CalendarIcon className="mr-2 h-4 w-4" />
        {formatDateRange(date)}
      </Button>

      <div className="space-y-4 p-4 border rounded"> {/* Calendar is always rendered */}
        <Calendar
          initialFocus
          mode="range"
          defaultMonth={date?.from || new Date()}
          selected={date}
          onSelect={handleDateChange}
          numberOfMonths={numberOfMonths}
          className="border rounded"
          showOutsideDays={false}
          disabled={{ after: disableAfter }}
        />
        <div className="flex items-center justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setDate(undefined)} // Clear the date
          >
            Clear
          </Button>
          <Button
            size="sm"
            onClick={handleApply}
            disabled={!date?.from || !date?.to}
          >
            Apply Range
          </Button>
        </div>
      </div>
      {error && <p className="text-sm text-red-500">{error}</p>}
    </div>
  )
}
