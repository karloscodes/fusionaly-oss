package settings_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/settings"
	"fusionaly/internal/testsupport"
)

func TestIsIPExcluded(t *testing.T) {
	t.Run("excludes exact IP match", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		err := settings.UpdateSetting(db, "excluded_ips", "192.168.1.100")
		require.NoError(t, err)

		isExcluded, err := settings.IsIPExcluded("192.168.1.100")
		require.NoError(t, err)
		assert.True(t, isExcluded, "The exact IP in the exclusion list should be excluded")

		isExcluded, err = settings.IsIPExcluded("192.168.1.101")
		require.NoError(t, err)
		assert.False(t, isExcluded, "A different IP should not be excluded")
	})

	t.Run("handles IPs with whitespace", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		err := settings.UpdateSetting(db, "excluded_ips", " 192.168.1.100 , 10.0.0.1 ")
		require.NoError(t, err)

		isExcluded, err := settings.IsIPExcluded("192.168.1.100")
		require.NoError(t, err)
		assert.True(t, isExcluded, "IP should be excluded even with spaces in the setting")

		isExcluded, err = settings.IsIPExcluded("10.0.0.1")
		require.NoError(t, err)
		assert.True(t, isExcluded, "Second IP should be excluded even with spaces in the setting")
	})

	t.Run("handles empty exclusion value", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		err := settings.UpdateSetting(db, "excluded_ips", "")
		require.NoError(t, err)

		isExcluded, err := settings.IsIPExcluded("192.168.1.100")
		require.NoError(t, err)
		assert.False(t, isExcluded, "With empty exclusion value, no IPs should be excluded")
	})

	t.Run("reflects updates to exclusion list", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		err := settings.UpdateSetting(db, "excluded_ips", "192.168.1.100")
		require.NoError(t, err)

		isExcluded, err := settings.IsIPExcluded("192.168.1.100")
		require.NoError(t, err)
		assert.True(t, isExcluded, "Initial IP should be excluded")

		testIP := "10.0.0.5"
		isExcluded, err = settings.IsIPExcluded(testIP)
		require.NoError(t, err)
		assert.False(t, isExcluded, "Second IP should not be excluded initially")

		err = settings.UpdateSetting(db, "excluded_ips", "192.168.1.100,10.0.0.5")
		require.NoError(t, err)

		isExcluded, err = settings.IsIPExcluded(testIP)
		require.NoError(t, err)
		assert.True(t, isExcluded, "Second IP should be excluded after update")
	})
}

func TestGetSetting(t *testing.T) {
	t.Run("returns value for existing setting", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		err := settings.UpdateSetting(db, "test_setting", "test_value")
		require.NoError(t, err)

		value, err := settings.GetSetting(db, "test_setting")
		require.NoError(t, err)
		assert.Equal(t, "test_value", value, "GetSetting should return the correct value")
	})

	t.Run("returns error for non-existent setting", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		_, err := settings.GetSetting(db, "non_existent")
		assert.Error(t, err, "GetSetting should return an error for non-existent setting")
	})
}

func TestUpdateSetting(t *testing.T) {
	t.Run("updates existing setting", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		err := settings.UpdateSetting(db, "test_setting", "initial_value")
		require.NoError(t, err)

		value, err := settings.GetSetting(db, "test_setting")
		require.NoError(t, err)
		assert.Equal(t, "initial_value", value)

		err = settings.UpdateSetting(db, "test_setting", "updated_value")
		require.NoError(t, err)

		value, err = settings.GetSetting(db, "test_setting")
		require.NoError(t, err)
		assert.Equal(t, "updated_value", value, "UpdateSetting should update the value correctly")
	})

	t.Run("creates new setting if not exists", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		err := settings.UpdateSetting(db, "new_setting", "new_value")
		require.NoError(t, err)

		value, err := settings.GetSetting(db, "new_setting")
		require.NoError(t, err)
		assert.Equal(t, "new_value", value, "UpdateSetting should create a new setting if it doesn't exist")
	})
}

func TestCacheConsistency(t *testing.T) {
	t.Run("cache reflects setting changes", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		settings.SetupDefaultSettings(db)

		err := settings.UpdateSetting(db, "excluded_ips", "192.168.1.1")
		require.NoError(t, err)

		isExcluded, err := settings.IsIPExcluded("192.168.1.1")
		require.NoError(t, err)
		assert.True(t, isExcluded, "Initial IP should be excluded")

		err = settings.UpdateSetting(db, "excluded_ips", "192.168.1.1,192.168.1.2")
		require.NoError(t, err)

		isExcluded, err = settings.IsIPExcluded("192.168.1.2")
		require.NoError(t, err)
		assert.True(t, isExcluded, "New IP should be excluded after cache update")

		err = settings.UpdateSetting(db, "excluded_ips", "192.168.1.2")
		require.NoError(t, err)

		isExcluded, err = settings.IsIPExcluded("192.168.1.1")
		require.NoError(t, err)
		assert.False(t, isExcluded, "First IP should no longer be excluded after removal")
	})
}
