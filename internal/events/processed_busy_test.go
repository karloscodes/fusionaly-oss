package events

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/karloscodes/cartridge/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProcessUnprocessedEventsWhileBusy(t *testing.T) {
	t.Run("leaves the events for the next run instead of setting them aside", func(t *testing.T) {
		manager := sqlite.NewManager(sqlite.Config{
			Path:         filepath.Join(t.TempDir(), "busy.db"),
			MaxOpenConns: 4,
			MaxIdleConns: 4,
			WriteWait:    50 * time.Millisecond,
		})
		t.Cleanup(func() { _ = manager.Close() })
		db, err := manager.Connect()
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&IngestedEvent{}))
		for i := 0; i < 3; i++ {
			require.NoError(t, db.Create(&IngestedEvent{WebsiteID: 1, Hostname: "example.com", Pathname: "/",
				Timestamp: time.Now().UTC(), Processed: statusUnprocessed}).Error)
		}
		started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		go func() {
			defer close(done)
			_ = manager.Write(context.Background(), func(tx *gorm.DB) error {
				close(started)
				<-release
				return nil
			})
		}()
		<-started
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		result, err := ProcessUnprocessedEvents(manager, logger, 2)
		close(release)
		<-done

		require.NoError(t, err)
		assert.Empty(t, result.ProcessedEvents)
		var waiting, failed int64
		db.Model(&IngestedEvent{}).Where("processed = ?", statusUnprocessed).Count(&waiting)
		db.Model(&IngestedEvent{}).Where("processed = ?", statusFailed).Count(&failed)
		assert.Equal(t, int64(3), waiting, "busy events stay unprocessed")
		assert.Equal(t, int64(0), failed, "a busy database is not a bad event")
	})

	t.Run("does not set events aside when the database frees up mid-run", func(t *testing.T) {
		// The burst ends after two writes. Retrying the events one by one
		// would then succeed in setting them aside as failed, for good.
		manager := sqlite.NewManager(sqlite.Config{Path: filepath.Join(t.TempDir(), "burst.db")})
		t.Cleanup(func() { _ = manager.Close() })
		db, err := manager.Connect()
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&IngestedEvent{}))
		for i := 0; i < 3; i++ {
			require.NoError(t, db.Create(&IngestedEvent{WebsiteID: 1, Hostname: "example.com", Pathname: "/",
				Timestamp: time.Now().UTC(), Processed: statusUnprocessed}).Error)
		}
		burst := &busyFirst{Manager: manager}
		burst.left.Store(2)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		_, err = ProcessUnprocessedEvents(burst, logger, 2)

		require.NoError(t, err)
		var failed int64
		db.Model(&IngestedEvent{}).Where("processed = ?", statusFailed).Count(&failed)
		assert.Equal(t, int64(0), failed, "a busy database is not a bad event")
	})
}

// busyFirst answers the first writes with sqlite.ErrBusy, as a full write
// queue does, then lets writes through.
type busyFirst struct {
	*sqlite.Manager
	left atomic.Int32
}

func (b *busyFirst) Write(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if b.left.Add(-1) >= 0 {
		return sqlite.ErrBusy
	}
	return b.Manager.Write(ctx, fn)
}
