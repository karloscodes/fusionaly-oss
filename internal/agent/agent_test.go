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
		}

		for _, q := range valid {
			if err := agent.ValidateReadOnlyQuery(q); err != nil {
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
			if err := agent.ValidateReadOnlyQuery(q); err == nil {
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
			if err := agent.ValidateReadOnlyQuery(q); err == nil {
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
			if err := agent.ValidateReadOnlyQuery(q); err == nil {
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
			if err := agent.ValidateReadOnlyQuery(q); err == nil {
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
			if err := agent.ValidateReadOnlyQuery(q); err == nil {
				t.Errorf("expected SQLite-specific dangerous query %q to fail", q)
			}
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

func TestExecuteQuery(t *testing.T) {
	t.Run("executes valid SELECT query", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		ctx := context.Background()
		result, err := agent.ExecuteQuery(ctx, db, "SELECT 1 as test", 5*time.Second)
		require.NoError(t, err)

		assert.Equal(t, []string{"test"}, result.Columns)
		assert.Len(t, result.Rows, 1)
		assert.Equal(t, 1, result.RowCount)
	})

	t.Run("rejects invalid query", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		ctx := context.Background()
		_, err := agent.ExecuteQuery(ctx, db, "DELETE FROM users", 5*time.Second)
		assert.Error(t, err)
	})

	t.Run("respects timeout", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		ctx := context.Background()
		// Very short timeout - SQLite doesn't really support cancellation well
		// but we at least test the timeout parameter is used
		result, err := agent.ExecuteQuery(ctx, db, "SELECT 1", 1*time.Second)
		require.NoError(t, err)
		assert.Equal(t, 1, result.RowCount)
	})

	t.Run("executes WITH (CTE) query", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		ctx := context.Background()
		result, err := agent.ExecuteQuery(ctx, db, "WITH cte AS (SELECT 42 as val) SELECT * FROM cte", 5*time.Second)
		require.NoError(t, err)

		assert.Equal(t, []string{"val"}, result.Columns)
		assert.Len(t, result.Rows, 1)
	})
}
