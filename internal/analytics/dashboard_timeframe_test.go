package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
	"fusionaly/internal/settings"
	"fusionaly/internal/testsupport"
)

func TestDashboardTimeFrame(t *testing.T) {
	t.Run("the default range is the last 30 days, today included, from local midnight", func(t *testing.T) {
		dbManager, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "range.test")
		db := dbManager.GetConnection()
		madrid, err := time.LoadLocation("Europe/Madrid")
		require.NoError(t, err)

		tf, err := analytics.DashboardTimeFrame(db, int(site.ID), "Europe/Madrid", "", "")

		require.NoError(t, err)
		now := time.Now().In(madrid)
		want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, madrid).AddDate(0, 0, -29)
		assert.True(t, tf.From.Equal(want), "from %s, want %s", tf.From.In(madrid), want)
	})

	t.Run("the public page counts the same days as the owner's dashboard", func(t *testing.T) {
		dbManager, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "range.test")
		db := dbManager.GetConnection()
		require.NoError(t, settings.SaveTimezone(db, "Asia/Tokyo"))

		private, err := analytics.DashboardTimeFrame(db, int(site.ID), "Asia/Tokyo", "", "")
		require.NoError(t, err)
		public, err := analytics.DashboardTimeFrame(db, int(site.ID), settings.Timezone(db), "", "")
		require.NoError(t, err)

		assert.True(t, private.From.Equal(public.From))
		assert.Equal(t, private.BucketSize, public.BucketSize)
		assert.WithinDuration(t, private.To, public.To, time.Second)
	})
}
