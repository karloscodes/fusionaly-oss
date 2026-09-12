package events

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"fusionaly/internal/websites"
)

// lockedWriteError produces a genuine SQLite contention error: one connection
// holds an exclusive transaction while another tries to write.
func lockedWriteError(t *testing.T) error {
	t.Helper()

	dsn := "file:busy_test?mode=memory&cache=shared&_busy_timeout=0"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS probe (id INTEGER PRIMARY KEY)").Error)

	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	locker, err := pool.Conn(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { locker.Close() })

	_, err = locker.ExecContext(t.Context(), "BEGIN EXCLUSIVE")
	require.NoError(t, err)

	writer, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { writer.Close() })

	_, err = writer.Exec("INSERT INTO probe (id) VALUES (1)")
	require.Error(t, err, "write against an exclusively locked database should fail")

	return err
}

func TestClassifyWriteError(t *testing.T) {
	t.Run("tags a real locked write as busy", func(t *testing.T) {
		err := lockedWriteError(t)

		assert.ErrorIs(t, classifyWriteError(err), ErrStorageBusy)
	})

	t.Run("tags a busy write wrapped by cartridge's retry loop", func(t *testing.T) {
		// PerformWrite returns the driver error inside two layers of context.
		busy := lockedWriteError(t)
		wrapped := fmt.Errorf("failed to store ingested event: %w",
			fmt.Errorf("transaction failed after 10 retries: %w", busy))

		assert.ErrorIs(t, classifyWriteError(wrapped), ErrStorageBusy)
	})

	t.Run("tags the lock messages the driver emits in production", func(t *testing.T) {
		// The in-memory harness above yields the shared-cache variant. A file
		// database under WAL yields these two, taken from sqlite3.ErrBusy and
		// sqlite3.ErrLocked.
		for _, msg := range []string{"database is locked", "database table is locked"} {
			assert.ErrorIs(t, classifyWriteError(errors.New(msg)), ErrStorageBusy, msg)
		}
	})

	t.Run("passes other write failures through untouched", func(t *testing.T) {
		original := errors.New("no such table: ingested_events")

		err := classifyWriteError(original)

		assert.Equal(t, original, err)
		assert.NotErrorIs(t, err, ErrStorageBusy)
	})

	t.Run("does not treat a domain containing a lock word as a busy database", func(t *testing.T) {
		// Regression: the busy check used to sniff for a bare "busy" or
		// "locked", and the tracked domain lands in this error text, so
		// busybee.com answered 503 DATABASE_BUSY instead of 400
		// WEBSITE_NOT_FOUND and the SDK retried an event it could never store.
		for _, domain := range []string{"busybee.com", "lockedin.com"} {
			notFound := websites.NewWebsiteNotFoundError(domain)

			err := classifyWriteError(notFound)

			assert.NotErrorIs(t, err, ErrStorageBusy, domain)

			var stillTyped *websites.WebsiteNotFoundError
			assert.ErrorAs(t, err, &stillTyped, "the handler must still see the typed error")
		}
	})
}
