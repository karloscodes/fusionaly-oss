package database

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/manager/logging"
)

// ---- Test clock ----

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func TestNewDatabase(t *testing.T) {
	db := NewDatabase(nil)
	if db == nil {
		t.Fatal("NewDatabase returned nil")
	}
}

func TestListBackups_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	db := NewDatabase(nil)
	backups, err := db.ListBackups(dir)
	if err != nil {
		t.Fatalf("ListBackups error: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("Expected 0 backups, got %d", len(backups))
	}
}

func TestListBackups_Sorted(t *testing.T) {
	dir := t.TempDir()
	files := []string{"backup_20240101_120000.db", "backup_20240102_120000.db", "backup_20231231_120000.db"}
	for _, f := range files {
		_ = os.WriteFile(filepath.Join(dir, f), []byte("db"), 0o644)
	}
	db := NewDatabase(nil)
	backups, err := db.ListBackups(dir)
	if err != nil {
		t.Fatalf("ListBackups error: %v", err)
	}
	if len(backups) != 3 {
		t.Errorf("Expected 3 backups, got %d", len(backups))
	}
	if backups[0].Name != "backup_20240102_120000.db" {
		t.Errorf("Expected first backup to be latest, got %s", backups[0].Name)
	}
}

func TestValidateBackup_NonexistentFile(t *testing.T) {
	db := NewDatabase(nil)
	err := db.ValidateBackup("/does/not/exist.db")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestValidateBackup_ZeroSize(t *testing.T) {
	db := NewDatabase(nil)
	file := filepath.Join(t.TempDir(), "empty.db")
	_ = os.WriteFile(file, []byte{}, 0o644)
	err := db.ValidateBackup(file)
	if err == nil || err.Error() != "backup file is empty" {
		t.Errorf("Expected 'backup file is empty', got %v", err)
	}
}

func setupTestDB(t *testing.T) (*Database, string, string) {
	// Create a temporary directory for test database and backups
	tmpDir := t.TempDir()

	// Create paths
	dbPath := filepath.Join(tmpDir, "test.db")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create a simple SQLite database using the sqlite3 command
	cmd := exec.Command("sqlite3", dbPath, "PRAGMA page_size=4096; PRAGMA user_version=1; CREATE TABLE test(id INTEGER PRIMARY KEY);")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to create test database: %s", string(output))

	// Verify the database was created and is valid
	validateCmd := exec.Command("sqlite3", dbPath, "PRAGMA integrity_check;")
	output, err = validateCmd.CombinedOutput()
	require.NoError(t, err, "Database validation failed: %s", string(output))
	require.Equal(t, "ok\n", string(output), "Database integrity check failed")

	log.Printf("Created valid test database at %s", dbPath)

	// Verify database was created
	info, err := os.Stat(dbPath)
	require.NoError(t, err, "Database file was not created")
	require.Greater(t, info.Size(), int64(0), "Database file is empty")

	// Initialize test Database instance with logger
	logger := logging.NewLogger(logging.Config{
		Level:   "info",
		Verbose: false,
		Quiet:   true,
	})

	return NewDatabase(logger), dbPath, backupDir
}

func TestBackupCreationAndRetention(t *testing.T) {
	db, dbPath, backupDir := setupTestDB(t)

	// Freeze "now" to make retention logic deterministic (matches comments below)
	db.clock = fixedClock{t: time.Date(2025, 8, 11, 12, 0, 0, 0, time.UTC)}

	// Set retention for deterministic testing
	db.SetRetentionConfig(RetentionConfig{
		DailyRetentionDays:   3,  // Keep daily backups for 3 days
		WeeklyRetentionDays:  10, // Keep weekly backups for 10 days
		MonthlyRetentionDays: 15, // Keep monthly backups for 15 days
	})

	// Define test backups with specific fixed dates to ensure correct type detection
	// - Daily: not Sunday, not 1st of month
	// - Weekly: Sunday
	// - Monthly: 1st of month
	testBackups := []struct {
		backupTime time.Time
		expected   bool // should it exist after cleanup?
	}{
		// Daily backups (not on Sunday, not on 1st of month)
		{time.Date(2025, 8, 9, 10, 0, 0, 0, time.UTC), true},  // Saturday, 2 days ago, within 3-day retention
		{time.Date(2025, 8, 6, 10, 0, 0, 0, time.UTC), false}, // Tuesday, 5 days ago, beyond 3-day retention

		// Weekly backups (on Sundays)
		{time.Date(2025, 8, 3, 15, 0, 0, 0, time.UTC), true},   // Sunday, 8 days ago, within 10-day retention
		{time.Date(2025, 7, 27, 12, 0, 0, 0, time.UTC), false}, // Sunday, 15 days ago, beyond 10-day retention

		// Monthly backups (on 1st of month)
		{time.Date(2025, 8, 1, 18, 0, 0, 0, time.UTC), true},  // 1st, 10 days ago, within 15-day retention
		{time.Date(2025, 7, 1, 14, 0, 0, 0, time.UTC), false}, // 1st, 41 days ago, beyond 15-day retention
	}

	for _, tb := range testBackups {
		backupPath := filepath.Join(backupDir, "backup_"+tb.backupTime.Format("20060102_150405")+".db")
		require.NoError(t, os.MkdirAll(filepath.Dir(backupPath), 0o755))

		file, err := os.Create(backupPath)
		require.NoError(t, err)
		_, err = file.Write([]byte("test data"))
		require.NoError(t, err)
		_ = file.Close()

		// Set file modification time
		require.NoError(t, os.Chtimes(backupPath, tb.backupTime, tb.backupTime))
	}

	// Log the test database file info
	log.Printf("Test database at %s exists: %v", dbPath, fileExists(dbPath))

	// Add a small delay to ensure the database file from setupTestDB is fully released/flushed
	time.Sleep(1000 * time.Millisecond)

	// Create a new backup (timestamp uses fixed clock via db.clock)
	backupPath, err := db.BackupDatabase(dbPath, backupDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, backupPath)

	// Check remaining backups
	backups, err := db.ListBackups(backupDir)
	assert.NoError(t, err)

	// Count remaining backups (should only have recent ones)
	expectedCount := 0
	for _, tb := range testBackups {
		if tb.expected {
			expectedCount++
		}
	}
	// +1 for the new backup we just created
	assert.Equal(t, expectedCount+1, len(backups), "Unexpected number of backups after cleanup")

	// Verify backup types
	for _, backup := range backups {
		assert.NotEmpty(t, backup.BackupType, "Backup type should not be empty for %s", backup.Name)
	}
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func TestRetentionConfigUpdate(t *testing.T) {
	db, _, _ := setupTestDB(t)

	// Test default config
	defaultConfig := db.GetRetentionConfig()
	assert.Equal(t, 7, defaultConfig.DailyRetentionDays)
	assert.Equal(t, 14, defaultConfig.WeeklyRetentionDays)
	assert.Equal(t, 90, defaultConfig.MonthlyRetentionDays)

	// Test config update
	newConfig := RetentionConfig{
		DailyRetentionDays:   3,
		WeeklyRetentionDays:  10,
		MonthlyRetentionDays: 60,
	}
	db.SetRetentionConfig(newConfig)

	updatedConfig := db.GetRetentionConfig()
	assert.Equal(t, newConfig, updatedConfig)
}

func TestDatabaseBackupCreation(t *testing.T) {
	t.Run("CreateTimestampedBackupFromValidDatabase", func(t *testing.T) {
		db, mainDBPath, backupDir := setupTestDB(t)

		backupPath, err := db.BackupDatabase(mainDBPath, backupDir)

		assert.NoError(t, err, "Backup creation should succeed")
		assert.NotEmpty(t, backupPath, "Backup path should not be empty")
		assert.True(t, fileExists(backupPath), "Backup file should exist")

		// Verify backup filename format
		filename := filepath.Base(backupPath)
		assert.True(t, strings.HasPrefix(filename, "backup_"), "Backup should have correct prefix")
		assert.True(t, strings.HasSuffix(filename, ".db"), "Backup should have .db extension")
	})

	t.Run("ReturnErrorForNonExistentSourceDB", func(t *testing.T) {
		db, _, backupDir := setupTestDB(t)
		nonExistentDB := "/path/to/nonexistent.db"

		backupPath, err := db.BackupDatabase(nonExistentDB, backupDir)

		assert.Error(t, err, "Should error when source database doesn't exist")
		assert.Empty(t, backupPath, "Should return empty backup path on error")
	})
}

func TestDatabaseBackupCleanup(t *testing.T) {
	t.Run("RemoveExpiredBackupsButKeepRecent", func(t *testing.T) {
		db, _, backupDir := setupTestDB(t)

		// Ensure backup directory exists
		require.NoError(t, os.MkdirAll(backupDir, 0o755))

		// Create old backup files (simulate old timestamps)
		oldBackups := []string{
			"backup_20230101_120000.db", // Very old - should be deleted
			"backup_20230601_120000.db", // Old - should be deleted
		}

		for _, backup := range oldBackups {
			backupPath := filepath.Join(backupDir, backup)
			err := os.WriteFile(backupPath, []byte("old backup content"), 0o644)
			require.NoError(t, err)
		}

		// Create recent backup (should be kept)
		recentBackup := fmt.Sprintf("backup_%s.db", time.Now().Format("20060102_150405"))
		recentPath := filepath.Join(backupDir, recentBackup)
		err := os.WriteFile(recentPath, []byte("recent backup content"), 0o644)
		require.NoError(t, err)

		strictConfig := RetentionConfig{
			DailyRetentionDays:   1, // Very short retention
			WeeklyRetentionDays:  7,
			MonthlyRetentionDays: 30,
		}
		db.SetRetentionConfig(strictConfig)

		err = db.cleanupOldBackups(backupDir)

		assert.NoError(t, err, "Cleanup should succeed")
		// Verify recent backup still exists
		assert.True(t, fileExists(recentPath), "Recent backup should be preserved")
	})
}

func TestBackupRestoreFlow(t *testing.T) {
	t.Run("RestoreValidBackupReplacesMainDatabase", func(t *testing.T) {
		db, mainDBPath, backupDir := setupTestDB(t)

		// Ensure backup directory exists
		require.NoError(t, os.MkdirAll(backupDir, 0o755))

		// Create a valid backup file by first creating another database
		backupDBPath := filepath.Join(backupDir, "temp_backup_source.db")
		cmd := exec.Command("sqlite3", backupDBPath, "PRAGMA page_size=4096; PRAGMA user_version=1; CREATE TABLE backup_test(id INTEGER PRIMARY KEY, data TEXT); INSERT INTO backup_test(data) VALUES ('backup_content');")
		require.NoError(t, cmd.Run())

		// Now create the backup file in the expected location
		backupPath := filepath.Join(backupDir, "backup_20240101_120000.db")
		cmd = exec.Command("sqlite3", backupDBPath, fmt.Sprintf(".backup '%s'", backupPath))
		require.NoError(t, cmd.Run())

		// Clean up temp file
		_ = os.Remove(backupDBPath)

		err := db.RestoreDatabase(mainDBPath, backupPath)

		assert.NoError(t, err, "Restore should succeed")

		// Verify the database was restored by checking if it's a valid SQLite database
		validateCmd := exec.Command("sqlite3", mainDBPath, "PRAGMA integrity_check;")
		output, err := validateCmd.CombinedOutput()
		require.NoError(t, err)
		assert.Equal(t, "ok\n", string(output), "Restored database should be valid")

		// Original backup file should be consumed (moved)
		assert.False(t, fileExists(backupPath), "Original backup file should be moved/consumed")
	})

	t.Run("RestoreCorruptedBackupReturnsValidationError", func(t *testing.T) {
		db, mainDBPath, backupDir := setupTestDB(t)

		// Ensure backup directory exists
		require.NoError(t, os.MkdirAll(backupDir, 0o755))

		// Create empty backup file (invalid)
		corruptBackupPath := filepath.Join(backupDir, "backup_corrupted.db")
		err := os.WriteFile(corruptBackupPath, []byte{}, 0o644)
		require.NoError(t, err)

		err = db.RestoreDatabase(mainDBPath, corruptBackupPath)

		assert.Error(t, err, "Should error when backup is corrupted")
		assert.Contains(t, err.Error(), "validation failed", "Error should indicate validation failure")
	})
}
