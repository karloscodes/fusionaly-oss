package ai_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/ai"
	"fusionaly/internal/testsupport"
)

// TestValidateReadOnlyQuery exercises the real ai.ValidateReadOnlyQuery validator.
func TestValidateReadOnlyQuery(t *testing.T) {
	t.Run("accepts plain SELECT", func(t *testing.T) {
		err := ai.ValidateReadOnlyQuery("SELECT visitors FROM site_stats WHERE website_id = 1")

		assert.NoError(t, err)
	})

	t.Run("accepts SELECT with a CTE (WITH)", func(t *testing.T) {
		query := `WITH days AS (SELECT 1 AS d UNION SELECT 2) SELECT d FROM days`

		err := ai.ValidateReadOnlyQuery(query)

		assert.NoError(t, err)
	})

	t.Run("rejects write and DDL statements", func(t *testing.T) {
		cases := map[string]string{
			"INSERT": "INSERT INTO site_stats (visitors) VALUES (1)",
			"UPDATE": "UPDATE site_stats SET visitors = 0",
			"DELETE": "DELETE FROM site_stats WHERE website_id = 1",
			"DROP":   "DROP TABLE site_stats",
			"ALTER":  "ALTER TABLE site_stats ADD COLUMN foo TEXT",
			"ATTACH": "ATTACH DATABASE 'evil.db' AS evil",
			"PRAGMA": "PRAGMA writable_schema = 1",
		}

		for name, query := range cases {
			t.Run(name, func(t *testing.T) {
				err := ai.ValidateReadOnlyQuery(query)

				assert.Error(t, err, "%s should be rejected", name)
			})
		}
	})

	t.Run("rejects multi-statement injection that hides a write", func(t *testing.T) {
		query := "SELECT * FROM site_stats; DROP TABLE site_stats"

		err := ai.ValidateReadOnlyQuery(query)

		assert.Error(t, err)
	})

	t.Run("rejects a write hidden behind a SQL comment", func(t *testing.T) {
		query := "SELECT 1\n-- harmless\nDELETE FROM site_stats"

		err := ai.ValidateReadOnlyQuery(query)

		assert.Error(t, err)
	})

	t.Run("rejects a tab-separated write keyword (whitespace bypass)", func(t *testing.T) {
		err := ai.ValidateReadOnlyQuery("SELECT 1\nDELETE\tFROM site_stats")

		assert.Error(t, err)
	})

	t.Run("rejects a multi-statement write that tabs around the keyword", func(t *testing.T) {
		// The confirmed exploit: the SQLite driver runs both statements.
		err := ai.ValidateReadOnlyQuery("SELECT id FROM site_stats;\nDELETE\tFROM site_stats")

		assert.Error(t, err)
	})

	t.Run("rejects load_extension", func(t *testing.T) {
		err := ai.ValidateReadOnlyQuery("SELECT load_extension('evil.so')")

		assert.Error(t, err)
	})

	t.Run("rejects a statement that does not start with SELECT or WITH", func(t *testing.T) {
		err := ai.ValidateReadOnlyQuery("VACUUM")

		assert.Error(t, err)
	})

	t.Run("allows a write keyword inside a string literal (no false positive)", func(t *testing.T) {
		err := ai.ValidateReadOnlyQuery("SELECT count(*) FROM page_stats WHERE pathname LIKE '%/delete%'")

		assert.NoError(t, err)
	})
}

// TestQueryCache exercises the real cache read/write/expiry functions.
func TestQueryCache(t *testing.T) {
	t.Run("cache key is deterministic for same question+website+model", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		err := ai.SetCachedQuery(db, "Top pages?", 1, "gpt-5.2", "SELECT 1", ai.TypeTable, "", nil)
		require.NoError(t, err)

		// Same inputs (with different whitespace/case) must resolve to the same cached row.
		got, found := ai.GetCachedQuery(db, "  top   PAGES?  ", 1, "gpt-5.2")

		require.True(t, found)
		assert.Equal(t, "SELECT 1", got.SQL)

		// A write with the normalized-equivalent question must upsert, not duplicate.
		err = ai.SetCachedQuery(db, "TOP pages?", 1, "gpt-5.2", "SELECT 2", ai.TypeTable, "", nil)
		require.NoError(t, err)

		var count int64
		require.NoError(t, db.Model(&ai.AIQueryCache{}).Count(&count).Error)
		assert.EqualValues(t, 1, count, "normalized-equivalent questions share one cache row")

		got, found = ai.GetCachedQuery(db, "Top pages?", 1, "gpt-5.2")
		require.True(t, found)
		assert.Equal(t, "SELECT 2", got.SQL)
	})

	t.Run("different website or model yields a different cache key", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		require.NoError(t, ai.SetCachedQuery(db, "Top pages?", 1, "gpt-5.2", "SELECT 1", ai.TypeTable, "", nil))

		_, foundOtherSite := ai.GetCachedQuery(db, "Top pages?", 2, "gpt-5.2")
		_, foundOtherModel := ai.GetCachedQuery(db, "Top pages?", 1, "gpt-4.1")

		assert.False(t, foundOtherSite, "different website_id must be a cache miss")
		assert.False(t, foundOtherModel, "different model must be a cache miss")
	})

	t.Run("expired entries are treated as misses", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		require.NoError(t, ai.SetCachedQuery(db, "Stale?", 1, "gpt-5.2", "SELECT 1", ai.TypeTable, "", nil))

		// Force the stored row to be expired.
		require.NoError(t, db.Model(&ai.AIQueryCache{}).
			Where("1 = 1").
			Update("expires_at", time.Now().Add(-1*time.Hour)).Error)

		_, found := ai.GetCachedQuery(db, "Stale?", 1, "gpt-5.2")

		assert.False(t, found, "an expired entry must be a cache miss")
	})

	t.Run("CleanExpiredCache removes only expired rows", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		require.NoError(t, ai.SetCachedQuery(db, "Fresh?", 1, "gpt-5.2", "SELECT 1", ai.TypeTable, "", nil))
		require.NoError(t, ai.SetCachedQuery(db, "Old?", 1, "gpt-5.2", "SELECT 2", ai.TypeTable, "", nil))
		require.NoError(t, db.Model(&ai.AIQueryCache{}).
			Where("question = ?", "Old?").
			Update("expires_at", time.Now().Add(-1*time.Hour)).Error)

		require.NoError(t, ai.CleanExpiredCache(db))

		var remaining int64
		require.NoError(t, db.Model(&ai.AIQueryCache{}).Count(&remaining).Error)
		assert.EqualValues(t, 1, remaining)

		_, found := ai.GetCachedQuery(db, "Fresh?", 1, "gpt-5.2")
		assert.True(t, found, "the non-expired row must survive cleanup")
	})
}

// TestSavedQueryCRUD exercises the real saved-query DB round trips.
func TestSavedQueryCRUD(t *testing.T) {
	t.Run("create, list-by-website, get and delete", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		websiteID := uint(7)

		created, err := ai.CreateSavedQueryWithVega(db, "Top pages", "SELECT 1", "", &websiteID, ai.TypeTable, "")
		require.NoError(t, err)
		require.NotZero(t, created.ID)
		assert.Equal(t, ai.DefaultModel, created.Model, "empty model falls back to the default")

		// list-by-website returns the saved query
		list, err := ai.GetSavedQueriesByWebsiteID(db, websiteID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "Top pages", list[0].Title)

		// list-by-website is scoped: a different website sees nothing
		other, err := ai.GetSavedQueriesByWebsiteID(db, websiteID+1)
		require.NoError(t, err)
		assert.Empty(t, other)

		// get by id
		fetched, err := ai.GetSavedQuery(db, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, fetched.ID)

		// delete
		require.NoError(t, ai.DeleteSavedQuery(db, created.ID))
		afterDelete, err := ai.GetSavedQueriesByWebsiteID(db, websiteID)
		require.NoError(t, err)
		assert.Empty(t, afterDelete)
	})

	t.Run("newest saved query is ordered first", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		websiteID := uint(7)

		_, err := ai.CreateSavedQueryWithVega(db, "First", "SELECT 1", "", &websiteID, ai.TypeTable, "gpt-5.2")
		require.NoError(t, err)
		_, err = ai.CreateSavedQueryWithVega(db, "Second", "SELECT 2", "", &websiteID, ai.TypeTable, "gpt-5.2")
		require.NoError(t, err)

		list, err := ai.GetSavedQueriesByWebsiteID(db, websiteID)
		require.NoError(t, err)
		require.Len(t, list, 2)

		// CreateSavedQueryWithVega bumps every existing row's order, so the most
		// recently created query sorts to the top (lowest order).
		assert.Equal(t, "Second", list[0].Title)
		assert.Equal(t, "First", list[1].Title)
		assert.Less(t, list[0].Order, list[1].Order)
	})

	t.Run("clone copies the source query with a (Copy) title", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		websiteID := uint(7)

		original, err := ai.CreateSavedQueryWithVega(db, "Source", "SELECT 1", `{"mark":"bar"}`, &websiteID, ai.TypeDistribution, "gpt-5.2")
		require.NoError(t, err)

		clone, err := ai.CloneSavedQuery(db, original.ID)
		require.NoError(t, err)

		assert.Equal(t, "Source (Copy)", clone.Title)
		assert.Equal(t, original.GeneratedSQL, clone.GeneratedSQL)
		assert.Equal(t, original.VegaSpec, clone.VegaSpec)
		assert.Equal(t, original.QueryType, clone.QueryType)

		list, err := ai.GetSavedQueriesByWebsiteID(db, websiteID)
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})
}
