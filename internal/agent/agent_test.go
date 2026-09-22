package agent_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/agent"
	"fusionaly/internal/testsupport"
)

// tables mirrors a real install: analytics tables plus the secret ones.
var tables = []string{"site_stats", "page_stats", "users", "settings", "onboarding_sessions", "ingested_events"}

func TestValidateReadOnlyQuery(t *testing.T) {
	t.Run("allows valid SELECT queries", func(t *testing.T) {
		valid := []string{
			"SELECT * FROM site_stats",
			"select * from site_stats",
			"SELECT COUNT(*) FROM site_stats WHERE website_id = 1",
			"SELECT pathname, SUM(visitors_count) FROM page_stats GROUP BY pathname",
			"SELECT * FROM site_stats WHERE hour >= datetime('now', '-7 days')",
			"SELECT * FROM page_stats WHERE pathname = '/delete-account'",
			"SELECT * FROM page_stats WHERE pathname LIKE '%update%'",
			"WITH daily AS (SELECT DATE(hour) as day FROM site_stats) SELECT * FROM daily",
			"with cte as (select 1) select * from cte",
			"SELECT * FROM site_stats;",
			"SELECT 'it''s' AS quote, \"hour\" FROM site_stats",
			"SELECT * FROM page_stats WHERE pathname = '/users/settings'",
		}

		for _, q := range valid {
			if err := agent.ValidateReadOnlyQuery(q, tables); err != nil {
				t.Errorf("expected valid query %q to pass, got error: %v", q, err)
			}
		}
	})

	t.Run("blocks non-SELECT queries", func(t *testing.T) {
		invalid := []string{
			"INSERT INTO site_stats VALUES (1, 2, 3)",
			"UPDATE site_stats SET visitors = 0",
			"DELETE FROM site_stats",
			"DROP TABLE site_stats",
			"CREATE TABLE evil (id INT)",
			"ALTER TABLE site_stats ADD COLUMN evil TEXT",
			"TRUNCATE site_stats",
		}

		for _, q := range invalid {
			if err := agent.ValidateReadOnlyQuery(q, tables); err == nil {
				t.Errorf("expected invalid query %q to fail", q)
			}
		}
	})

	t.Run("blocks queries with comments", func(t *testing.T) {
		invalid := []string{
			"SELECT * FROM site_stats /* comment */",
			"SELECT * FROM site_stats -- comment",
			"SELECT * FROM site_stats; DEL/**/ETE FROM users",
		}

		for _, q := range invalid {
			if err := agent.ValidateReadOnlyQuery(q, tables); err == nil {
				t.Errorf("expected query with comments %q to fail", q)
			}
		}
	})

	t.Run("blocks multiple statements", func(t *testing.T) {
		invalid := []string{
			"SELECT 1; SELECT 2;",
			"SELECT * FROM site_stats; SELECT * FROM users;",
			"SELECT * FROM site_stats; DELETE FROM site_stats;",
		}

		for _, q := range invalid {
			if err := agent.ValidateReadOnlyQuery(q, tables); err == nil {
				t.Errorf("expected multiple statement query %q to fail", q)
			}
		}
	})

	t.Run("blocks dangerous keywords even with whitespace tricks", func(t *testing.T) {
		invalid := []string{
			"SELECT * FROM site_stats;\nDELETE FROM users",
			"SELECT * FROM site_stats;\tDROP TABLE users",
			"SELECT * FROM site_stats;  DELETE   FROM users",
		}

		for _, q := range invalid {
			if err := agent.ValidateReadOnlyQuery(q, tables); err == nil {
				t.Errorf("expected whitespace-obfuscated query %q to fail", q)
			}
		}
	})

	t.Run("blocks SQLite dangerous functions", func(t *testing.T) {
		invalid := []string{
			"SELECT load_extension('evil.so')",
			"SELECT writefile('/tmp/evil', 'data')",
			"SELECT readfile('/etc/passwd')",
			"PRAGMA table_info(site_stats)",
			"ATTACH DATABASE '/tmp/evil.db' AS evil",
		}

		for _, q := range invalid {
			if err := agent.ValidateReadOnlyQuery(q, tables); err == nil {
				t.Errorf("expected SQLite-specific dangerous query %q to fail", q)
			}
		}
	})

	t.Run("blocks a second statement hidden behind a comment marker in a string", func(t *testing.T) {
		q := "SELECT '/*' AS a; DELETE FROM site_stats WHERE '*/' = '*/'"

		err := agent.ValidateReadOnlyQuery(q, tables)

		assert.Error(t, err)
	})

	t.Run("blocks transaction control that would hold a write lock", func(t *testing.T) {
		for _, q := range []string{"SELECT 1; BEGIN IMMEDIATE", "SELECT 1; SAVEPOINT x"} {
			assert.Error(t, agent.ValidateReadOnlyQuery(q, tables), q)
		}
	})

	t.Run("blocks tables that hold secrets, however they are quoted", func(t *testing.T) {
		invalid := []string{
			"SELECT key, value FROM settings",
			"SELECT * FROM users",
			"SELECT * FROM \"users\"",
			"SELECT * FROM [users]",
			"SELECT * FROM `onboarding_sessions`",
			"SELECT * FROM site_stats WHERE website_id IN (SELECT id FROM users)",
			"SELECT * FROM ingested_events",
			"SELECT * FROM sqlite_master",
			"SELECT * FROM pragma_table_info('users')",
		}

		for _, q := range invalid {
			assert.Error(t, agent.ValidateReadOnlyQuery(q, tables), q)
		}
	})
}

func TestGetSchema(t *testing.T) {
	t.Run("returns schema with tables and concepts", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		schema, err := agent.GetSchema(db)
		require.NoError(t, err)

		assert.NotEmpty(t, schema.Schema, "Schema should not be empty")
		assert.Contains(t, schema.Schema, "CREATE TABLE", "Schema should contain CREATE TABLE statements")
		assert.NotEmpty(t, schema.Concepts, "Concepts should not be empty")
		assert.Contains(t, schema.Concepts, "website_scoping", "Concepts should include website_scoping")
	})
}

func TestGetDatabaseSchema(t *testing.T) {
	t.Run("returns raw schema SQL", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		schema, err := agent.GetDatabaseSchema(db)
		require.NoError(t, err)

		assert.NotEmpty(t, schema)
		assert.Contains(t, schema, "CREATE TABLE")
	})
}

func TestQuery(t *testing.T) {
	t.Run("executes valid SELECT query", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		ctx := context.Background()
		result, err := agent.Query(ctx, db, "SELECT 1 as test", 5*time.Second)
		require.NoError(t, err)

		assert.Equal(t, []string{"test"}, result.Columns)
		assert.Len(t, result.Rows, 1)
		assert.Equal(t, 1, result.RowCount)
	})

	t.Run("rejects invalid query", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		ctx := context.Background()
		_, err := agent.Query(ctx, db, "DELETE FROM users", 5*time.Second)
		assert.Error(t, err)
	})

	t.Run("respects timeout", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		ctx := context.Background()
		// Very short timeout - SQLite doesn't really support cancellation well
		// but we at least test the timeout parameter is used
		result, err := agent.Query(ctx, db, "SELECT 1", 1*time.Second)
		require.NoError(t, err)
		assert.Equal(t, 1, result.RowCount)
	})

	t.Run("executes WITH (CTE) query", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		ctx := context.Background()
		result, err := agent.Query(ctx, db, "WITH cte AS (SELECT 42 as val) SELECT * FROM cte", 5*time.Second)
		require.NoError(t, err)

		assert.Equal(t, []string{"val"}, result.Columns)
		assert.Len(t, result.Rows, 1)
	})

	t.Run("caps the result at MaxRows", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		q := "WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM n WHERE x < 1500) SELECT x FROM n"

		result, err := agent.Query(context.Background(), db, q, 5*time.Second)

		require.NoError(t, err)
		assert.Equal(t, agent.MaxRows, result.RowCount)
		assert.True(t, result.Truncated)
	})

	t.Run("leaves the pool writable after a read-only query", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		for i := 0; i < 5; i++ {
			_, err := agent.Query(context.Background(), db, "SELECT 1", 5*time.Second)
			require.NoError(t, err)
		}

		err := db.Exec("CREATE TABLE after_query (x INTEGER)").Error

		assert.NoError(t, err, "no read-only connection may return to the pool")
	})

	t.Run("keeps existing data after a query", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		require.NoError(t, db.Exec("CREATE TABLE keep_me (x INTEGER)").Error)
		require.NoError(t, db.Exec("INSERT INTO keep_me VALUES (1)").Error)

		_, err := agent.Query(context.Background(), db, "SELECT 1", 5*time.Second)
		require.NoError(t, err)

		var count int64
		db.Raw("SELECT COUNT(*) FROM keep_me").Scan(&count)
		assert.Equal(t, int64(1), count, "an in-memory DB is lost if its last connection closes")
	})
}
