package analytics_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

// storeEvents sends the events through the event processor in time order, as
// live traffic arrives, so the summary tables count them.
func storeEvents(t *testing.T, db *gorm.DB, evts ...events.Event) {
	t.Helper()
	sorted := slices.Clone(evts)
	slices.SortStableFunc(sorted, func(a, b events.Event) int { return a.Timestamp.Compare(b.Timestamp) })
	for _, e := range sorted {
		require.NoError(t, db.Create(&events.IngestedEvent{
			WebsiteID: e.WebsiteID, UserSignature: e.UserSignature, Hostname: e.Hostname, Pathname: e.Pathname,
			ReferrerHostname: events.DirectOrUnknownReferrer, EventType: e.EventType, CustomEventName: e.CustomEventName, CustomEventMeta: e.CustomEventMeta,
			Timestamp: e.Timestamp, UserAgent: "Mozilla/5.0", CreatedAt: e.Timestamp,
		}).Error)
	}
	_, err := events.ProcessUnprocessedEvents(testsupport.NewTestDBManager(db), testsupport.GetLogger(), 100)
	require.NoError(t, err)
}
