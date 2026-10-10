package agent_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"

	"fusionaly/internal/agent"
)

// With the read pool, a query must not take the one write connection: a slow
// query would stop the ingestion of events.
func TestQueryWithReadPool(t *testing.T) {
	newDB := func(t *testing.T) (*sqlite.Manager, *gorm.DB) {
		t.Helper()
		m := sqlite.NewManager(sqlite.Config{Path: filepath.Join(t.TempDir(), "agent.db"), ReadPool: true})
		t.Cleanup(func() { _ = m.Close() })
		db, err := m.Connect()
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("CREATE TABLE events (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO events (name) VALUES ('pageview'), ('signup')").Error; err != nil {
			t.Fatal(err)
		}
		return m, db
	}

	t.Run("a query runs while a write holds the write connection", func(t *testing.T) {
		m, db := newDB(t)
		inWrite, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- m.Write(context.Background(), func(tx *gorm.DB) error {
				close(inWrite)
				<-release
				return nil
			})
		}()
		<-inWrite

		result, err := agent.Query(context.Background(), db, "SELECT COUNT(*) AS n FROM events", 2*time.Second)
		close(release)
		<-done

		if err != nil || result == nil || result.RowCount != 1 {
			t.Errorf("result = %+v, err = %v, want one row with no wait", result, err)
		}
	})

	t.Run("a write does not wait for a running query", func(t *testing.T) {
		m, db := newDB(t)
		// A query that takes about a second: a recursive count.
		slow := "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 3000000) SELECT COUNT(*) AS n FROM c, (SELECT 1 FROM events LIMIT 1)"
		queryDone := make(chan error, 1)
		go func() {
			_, err := agent.Query(context.Background(), db, slow, 10*time.Second)
			queryDone <- err
		}()
		time.Sleep(50 * time.Millisecond)

		start := time.Now()
		writeErr := m.Write(context.Background(), func(tx *gorm.DB) error {
			return tx.Exec("INSERT INTO events (name) VALUES ('during')").Error
		})
		waited := time.Since(start)

		select {
		case err := <-queryDone:
			t.Skipf("the query ended before the write (%v), so the test shows nothing on this machine", err)
		default:
		}
		if writeErr != nil || waited > 500*time.Millisecond {
			t.Errorf("the write took %v (err %v), want it not to wait for the query", waited, writeErr)
		}
		if err := <-queryDone; err != nil {
			t.Errorf("query: %v", err)
		}
	})

	t.Run("a query still cannot write", func(t *testing.T) {
		_, db := newDB(t)

		_, err := agent.Query(context.Background(), db, "DELETE FROM events", time.Second)

		var count int64
		db.Raw("SELECT COUNT(*) FROM events").Scan(&count)
		if err == nil || count != 2 {
			t.Errorf("err = %v, rows left = %d, want an error and 2 rows", err, count)
		}
	})
}
