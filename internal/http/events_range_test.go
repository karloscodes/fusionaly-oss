package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateDateRange(t *testing.T) {
	madrid, _ := time.LoadLocation("Europe/Madrid")
	// 00:30 on 15 October in Madrid is still 14 October in UTC.
	now := time.Date(2026, 10, 15, 0, 30, 0, 0, madrid)

	t.Run("today starts at midnight in the owner's zone", func(t *testing.T) {
		from, to := calculateDateRange("today", now)

		assert.Equal(t, time.Date(2026, 10, 14, 22, 0, 0, 0, time.UTC), from)
		assert.Equal(t, now.UTC(), to)
	})

	t.Run("yesterday is the owner's whole day before", func(t *testing.T) {
		from, to := calculateDateRange("yesterday", now)

		assert.Equal(t, time.Date(2026, 10, 13, 22, 0, 0, 0, time.UTC), from)
		assert.Equal(t, time.Date(2026, 10, 14, 21, 59, 59, 999999999, time.UTC), to)
	})

	t.Run("this month starts on the 1st in the owner's zone", func(t *testing.T) {
		from, _ := calculateDateRange("this_month", now)

		assert.Equal(t, time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC), from)
	})

	t.Run("the range is in UTC, as the events are stored", func(t *testing.T) {
		from, to := calculateDateRange("last_7_days", now)

		assert.Equal(t, time.UTC, from.Location())
		assert.Equal(t, time.UTC, to.Location())
	})
}
