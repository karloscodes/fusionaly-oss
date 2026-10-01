package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

func TestLiveVisitors(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	now := time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)

	t.Run("counts distinct visitors with any event in the last 5 minutes", func(t *testing.T) {
		visit(t, db, 1, "alice", "2024-07-01T11:58:00Z", events.EventTypePageView)
		visit(t, db, 1, "alice", "2024-07-01T11:59:00Z", events.EventTypeCustomEvent)
		visit(t, db, 1, "bob", "2024-07-01T11:56:00Z", events.EventTypeCustomEvent)

		live, err := analytics.GetLiveVisitors(db, 1, now)

		require.NoError(t, err)
		assert.Equal(t, int64(2), live)
	})

	t.Run("leaves out older events and other websites", func(t *testing.T) {
		visit(t, db, 2, "carol", "2024-07-01T11:54:00Z", events.EventTypePageView)
		visit(t, db, 3, "dave", "2024-07-01T11:59:00Z", events.EventTypePageView)

		live, err := analytics.GetLiveVisitors(db, 2, now)

		require.NoError(t, err)
		assert.Equal(t, int64(0), live)
	})
}
