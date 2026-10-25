//go:build load

package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/karloscodes/cartridge"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/database"
	"fusionaly/internal/events"
	"fusionaly/internal/jobs"
	"fusionaly/internal/websites"
)

// The processor starts a run every FUSIONALY_JOB_INTERVAL_SECONDS (60 by
// default), and a run takes at most events.MaxEventsPerRun events. This test
// measures how long one run takes on a full backlog, and how slow ingestion
// gets while a shorter interval drains a backlog. It uses a database file set
// up like production: WAL, one write connection, ten read connections, a
// site with a month of history, and the planner statistics that each start
// of the server writes.
//
// Runs with no pause drain faster, about 850 events/s on a 4-core N100, but
// they keep the writer busy 97% of the time. When the machine slows down for
// a moment, events then wait more than 5 s for the writer and get a 503. So
// the processor runs on an interval, and does not loop until the queue is
// empty.
//
// It takes about three minutes, so the load build tag keeps it out of make test:
//
//	FUSIONALY_ENV=test go test -tags load ./internal/jobs -run TestBacklogLoad -v
func TestBacklogLoad(t *testing.T) {
	const backlog = 20000
	const ingestPerSecond = 200

	t.Run("ingestion with no backlog", func(t *testing.T) {
		dbManager, logger, website := newProductionLikeDB(t)

		latencies, busy := ingestAtRate(dbManager, logger, website, ingestPerSecond, 10*time.Second, nil)

		t.Logf("ingestion: %s, %d busy", summarize(latencies), busy)
		require.Zero(t, busy)
	})

	t.Run("one run on a full backlog", func(t *testing.T) {
		dbManager, logger, website := newProductionLikeDB(t)
		queueBacklog(t, dbManager, website, events.MaxEventsPerRun)
		job := jobs.NewEventProcessorJob(dbManager, logger)

		start := time.Now()
		require.NoError(t, job.Run())
		took := time.Since(start)

		t.Logf("one run: %d events in %s, %d left", events.MaxEventsPerRun, took.Round(time.Millisecond), queued(dbManager))
		require.Zero(t, queued(dbManager))
	})

	t.Run("a run every 15 seconds drains the backlog while ingestion continues", func(t *testing.T) {
		drainWhileIngesting(t, backlog, ingestPerSecond, 15*time.Second)
	})
}

// drainWhileIngesting queues a backlog, then starts a run every interval, as
// the scheduler does. Ingestion goes on at perSecond until the processor
// catches up.
func drainWhileIngesting(t *testing.T, backlog, perSecond int, interval time.Duration) {
	db, logger, website := newProductionLikeDB(t)
	queueBacklog(t, db, website, backlog)
	dbManager := &timedWrites{DBManager: db}
	job := jobs.NewEventProcessorJob(dbManager, logger)
	var runs []time.Duration
	var runErr error
	drained := make(chan struct{})
	began := time.Now()

	go func() {
		defer close(drained)
		for {
			start := time.Now()
			waiting := queued(dbManager)
			if runErr = job.Run(); runErr != nil {
				return
			}
			runs = append(runs, time.Since(start))
			// Caught up: this run took every event that waited.
			if waiting < events.MaxEventsPerRun {
				return
			}
			time.Sleep(interval - time.Since(start))
		}
	}()
	latencies, busy := ingestAtRate(dbManager, logger, website, perSecond, 0, drained)
	elapsed := time.Since(began)

	require.NoError(t, runErr)
	var processed int64
	require.NoError(t, db.GetConnection().Model(&events.IngestedEvent{}).Where("processed = 1").Count(&processed).Error)
	var held time.Duration
	for _, turn := range dbManager.turns {
		held += turn
	}
	t.Logf("processor: %d runs, %s; drained %d events in %s, %.0f events/s",
		len(runs), summarize(runs), processed, elapsed.Round(time.Second), float64(processed)/elapsed.Seconds())
	t.Logf("ingestion: %s, %d busy", summarize(latencies), busy)
	t.Logf("write turns: %s; writer busy %.0f%% of the time", summarize(dbManager.turns), 100*held.Seconds()/elapsed.Seconds())
	require.Zero(t, busy, "no event got a 503")
}

// newProductionLikeDB opens a new database file with the settings of
// production and adds one website.
func newProductionLikeDB(t *testing.T) (*database.DBManager, *slog.Logger, websites.Website) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		DatabaseName:         filepath.Join(t.TempDir(), "load.db"),
		DatabaseMaxOpenConns: 10,
		DatabaseMaxIdleConns: 5,
	}
	dbManager := database.NewDBManager(cfg, logger)
	require.NoError(t, dbManager.Init())
	t.Cleanup(func() { _ = dbManager.Close() })
	require.NoError(t, dbManager.MigrateDatabase())
	db := dbManager.GetConnection()
	website := websites.Website{Domain: "load.test", CreatedAt: time.Now().UTC()}
	require.NoError(t, db.Create(&website).Error)

	// The month before the test: 300,000 page views, five a visitor.
	const history = 300000
	start := time.Now().UTC().AddDate(0, 0, -31).Format("2006-01-02 15:04:05")
	require.NoError(t, db.Exec(`
		INSERT INTO events (website_id, user_signature, hostname, pathname, referrer_hostname, referrer_pathname,
			event_type, custom_event_name, custom_event_meta, timestamp, session_start, created_at)
		WITH RECURSIVE n(i) AS (SELECT 0 UNION ALL SELECT i + 1 FROM n WHERE i < ? - 1),
			t(i, at) AS (SELECT i, strftime('%Y-%m-%d %H:%M:%S+00:00', ?, '+' || (i * 2592000 / ?) || ' seconds') FROM n)
		SELECT ?, 'old-' || (i / 5), ?, '/page-' || (i % 50), 'google.com', '', ?, '', '', at, at, at FROM t`,
		history, start, history, website.ID, website.Domain, events.EventTypePageView).Error)
	// Without statistics the planner looks up a visitor through the index on
	// (website_id, timestamp), and reads the site's events one by one. The
	// database manager writes them at each start of the server with this.
	require.NoError(t, db.Exec("PRAGMA optimize=0x10002").Error)
	// The first run after an upgrade rebuilds the counts of older versions,
	// once. A server that has run before has done that already.
	require.NoError(t, jobs.NewEventProcessorJob(dbManager, logger).Run())
	return dbManager, logger, website
}

var browsers = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (X11; Linux x86_64; rv:143.0) Gecko/20100101 Firefox/143.0",
	"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
}

// queueBacklog writes n page views to the queue directly, as if the
// processor was down while they came in: five page views a visitor, on 50
// pages, in the last hour.
func queueBacklog(t *testing.T, dbManager *database.DBManager, website websites.Website, n int) {
	t.Helper()
	now := time.Now().UTC()
	rows := make([]events.IngestedEvent, n)
	for i := range rows {
		at := now.Add(-time.Duration(n-i) * time.Hour / time.Duration(n))
		path := fmt.Sprintf("/page-%d", i%50)
		rows[i] = events.IngestedEvent{
			WebsiteID: website.ID, UserSignature: fmt.Sprintf("visitor-%d", i/5), Hostname: website.Domain,
			Pathname: path, RawURL: "https://" + website.Domain + path, ReferrerHostname: "google.com",
			EventType: events.EventTypePageView, Timestamp: at, CreatedAt: at,
			UserAgent: browsers[i%len(browsers)], Country: "US",
		}
	}
	require.NoError(t, dbManager.GetConnection().CreateInBatches(rows, 1000).Error)
}

func queued(dbManager cartridge.DBManager) int64 {
	var n int64
	dbManager.GetConnection().Model(&events.IngestedEvent{}).Where("processed = 0").Count(&n)
	return n
}

// ingestAtRate sends perSecond events through the real ingestion function,
// for the given time, or until stop closes. It returns the time of each
// call and the number of calls that the database refused as busy.
func ingestAtRate(dbManager cartridge.DBManager, logger *slog.Logger, website websites.Website,
	perSecond int, duration time.Duration, stop <-chan struct{}) ([]time.Duration, int) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var latencies []time.Duration
	busy := 0
	ticker := time.NewTicker(time.Second / time.Duration(perSecond))
	defer ticker.Stop()
	var timeout <-chan time.Time
	if duration > 0 {
		timeout = time.After(duration)
	}
	for i := 0; ; i++ {
		select {
		case <-stop:
			wg.Wait()
			return latencies, busy
		case <-timeout:
			wg.Wait()
			return latencies, busy
		case <-ticker.C:
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			input := &events.CollectEventInput{
				IPAddress: fmt.Sprintf("10.0.%d.%d", i/250%250, i%250), UserAgent: browsers[i%len(browsers)],
				ReferrerURL: "https://google.com/", EventType: events.EventTypePageView,
				RawUrl: fmt.Sprintf("https://%s/page-%d", website.Domain, i%50),
			}
			start := time.Now()
			err := events.CollectEvent(dbManager, logger, input)
			took := time.Since(start)
			mu.Lock()
			defer mu.Unlock()
			latencies = append(latencies, took)
			if errors.Is(err, events.ErrStorageBusy) {
				busy++
			}
		}()
	}
}

// timedWrites records how long each write holds the write connection,
// from the start of the transaction to the end of the commit.
type timedWrites struct {
	*database.DBManager
	mu    sync.Mutex
	turns []time.Duration
}

func (w *timedWrites) Write(ctx context.Context, fn func(tx *gorm.DB) error) error {
	var start time.Time
	err := w.DBManager.Write(ctx, func(tx *gorm.DB) error {
		start = time.Now()
		return fn(tx)
	})
	if !start.IsZero() {
		w.mu.Lock()
		w.turns = append(w.turns, time.Since(start))
		w.mu.Unlock()
	}
	return err
}

func summarize(durations []time.Duration) string {
	if len(durations) == 0 {
		return "no samples"
	}
	sorted := slices.Clone(durations)
	slices.Sort(sorted)
	at := func(p float64) time.Duration {
		return sorted[int(p*float64(len(sorted)-1))].Round(10 * time.Microsecond)
	}
	return fmt.Sprintf("%d samples, p50 %s, p99 %s, max %s", len(sorted), at(0.50), at(0.99), sorted[len(sorted)-1].Round(10*time.Microsecond))
}
