package agent

import (
	"testing"
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
		}

		for _, q := range valid {
			if err := ValidateReadOnlyQuery(q); err != nil {
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
			if err := ValidateReadOnlyQuery(q); err == nil {
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
			if err := ValidateReadOnlyQuery(q); err == nil {
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
			if err := ValidateReadOnlyQuery(q); err == nil {
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
			if err := ValidateReadOnlyQuery(q); err == nil {
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
			if err := ValidateReadOnlyQuery(q); err == nil {
				t.Errorf("expected SQLite-specific dangerous query %q to fail", q)
			}
		}
	})
}
