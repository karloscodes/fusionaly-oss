package visitors

import (
	"fmt"
	"testing"
	"time"

	ctestsupport "github.com/karloscodes/cartridge/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"fusionaly/internal/settings"
)

func newSaltDB(t *testing.T) (*ctestsupport.TestDBManager, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:salt_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settings.Setting{}))
	t.Cleanup(func() {
		saltDay, saltValue = "", ""
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	saltDay, saltValue = "", ""
	return ctestsupport.NewTestDBManager(db), db
}

// restart drops what the process remembers, as a restart does.
func restart() { saltDay, saltValue = "", "" }

func TestDailySalt(t *testing.T) {
	monday := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)

	t.Run("is the same all day", func(t *testing.T) {
		dbManager, _ := newSaltDB(t)

		morning, err := dailySaltOn(dbManager, monday)
		require.NoError(t, err)
		night, err := dailySaltOn(dbManager, monday.Add(14*time.Hour+59*time.Minute))
		require.NoError(t, err)

		assert.Len(t, morning, 64)
		assert.Equal(t, morning, night)
	})

	t.Run("survives a restart", func(t *testing.T) {
		dbManager, _ := newSaltDB(t)
		before, err := dailySaltOn(dbManager, monday)
		require.NoError(t, err)

		restart()
		after, err := dailySaltOn(dbManager, monday.Add(time.Hour))
		require.NoError(t, err)

		assert.Equal(t, before, after)
	})

	t.Run("changes at midnight UTC and forgets the old salt", func(t *testing.T) {
		dbManager, db := newSaltDB(t)
		mondaySalt, err := dailySaltOn(dbManager, monday)
		require.NoError(t, err)

		tuesdaySalt, err := dailySaltOn(dbManager, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		assert.NotEqual(t, mondaySalt, tuesdaySalt)
		var stored []string
		require.NoError(t, db.Model(&settings.Setting{}).Pluck("value", &stored).Error)
		assert.Equal(t, []string{"2026-10-20:" + tuesdaySalt}, stored)
	})

	t.Run("gives the same visitor another signature the next day", func(t *testing.T) {
		dbManager, _ := newSaltDB(t)
		mondaySalt, err := dailySaltOn(dbManager, monday)
		require.NoError(t, err)
		tuesdaySalt, err := dailySaltOn(dbManager, monday.Add(24*time.Hour))
		require.NoError(t, err)

		mondayID := BuildUniqueVisitorId("example.com", "1.2.3.4", "Mozilla/5.0", mondaySalt)
		tuesdayID := BuildUniqueVisitorId("example.com", "1.2.3.4", "Mozilla/5.0", tuesdaySalt)

		assert.NotEqual(t, mondayID, tuesdayID)
	})
}
