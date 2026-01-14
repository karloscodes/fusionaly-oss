package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/manager/config"
	"fusionaly/internal/manager/logging"
)

func TestNewInstaller(t *testing.T) {
	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	installer := NewInstaller(logger)

	assert.NotNil(t, installer)
	assert.NotNil(t, installer.logger)
	assert.NotNil(t, installer.config)
	assert.NotNil(t, installer.docker)
	assert.NotNil(t, installer.database)
	assert.Equal(t, DefaultBinaryPath, installer.binaryPath)
}

func TestGetConfig(t *testing.T) {
	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	installer := NewInstaller(logger)

	config := installer.GetConfig()
	assert.NotNil(t, config)
}

func TestGetMainDBPath(t *testing.T) {
	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	installer := NewInstaller(logger)

	// Test with default config
	dbPath := installer.GetMainDBPath()
	expectedPath := filepath.Join(DefaultInstallDir, "storage", "fusionaly-production.db")
	assert.Equal(t, expectedPath, dbPath)

	// Test with custom install dir
	cfg := config.NewConfig(logger)
	cfg.SetInstallDir("/custom/install/dir")
	installer.config = cfg

	dbPath = installer.GetMainDBPath()
	expectedPath = filepath.Join("/custom/install/dir", "storage", "fusionaly-production.db")
	assert.Equal(t, expectedPath, dbPath)
}

func TestGetBackupDir(t *testing.T) {
	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	installer := NewInstaller(logger)

	// Test with default config
	backupDir := installer.GetBackupDir()
	expectedDir := filepath.Join(DefaultInstallDir, "storage", "backups")
	assert.Equal(t, expectedDir, backupDir)

	// Test with custom install dir
	cfg := config.NewConfig(logger)
	cfg.SetInstallDir("/custom/install/dir")
	installer.config = cfg

	backupDir = installer.GetBackupDir()
	expectedDir = filepath.Join("/custom/install/dir", "storage", "backups")
	assert.Equal(t, expectedDir, backupDir)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "/opt/fusionaly", DefaultInstallDir)
	assert.Equal(t, "/usr/local/bin/fusionaly", DefaultBinaryPath)
	assert.Equal(t, "/etc/cron.d/fusionaly-update", DefaultCronFile)
	assert.Equal(t, "0 3 * * *", DefaultCronSchedule)
}

func TestRestoreDBFlow(t *testing.T) {
	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	installer := NewInstaller(logger)
	tempDir := t.TempDir()
	
	// Configure installer with temp directory
	cfg := config.NewConfig(logger)
	cfg.SetInstallDir(tempDir)
	installer.config = cfg

	t.Run("ListBackupsFromEmptyDirectory", func(t *testing.T) {
		backupDir := installer.GetBackupDir()
		err := os.MkdirAll(backupDir, 0755)
		require.NoError(t, err)
		
		backups, err := installer.ListBackups()
		
		assert.NoError(t, err, "Listing backups should not error when directory is empty")
		assert.Empty(t, backups, "Should return empty backup list")
	})

	t.Run("ListBackupsSortedByNewestFirst", func(t *testing.T) {
		backupDir := installer.GetBackupDir()
		err := os.MkdirAll(backupDir, 0755)
		require.NoError(t, err)
		
		// Create test backup files (older to newer)
		testBackups := []string{
			"backup_20240101_120000.db",
			"backup_20240102_120000.db", 
			"backup_20240103_120000.db",
		}
		
		for _, backup := range testBackups {
			err := os.WriteFile(filepath.Join(backupDir, backup), []byte("test db content"), 0644)
			require.NoError(t, err)
		}
		
		backups, err := installer.ListBackups()
		
		assert.NoError(t, err, "Listing backups should not error")
		assert.Len(t, backups, 3, "Should return all 3 backup files")
		assert.Equal(t, "backup_20240103_120000.db", backups[0].Name, "Newest backup should be first")
		assert.Equal(t, "backup_20240101_120000.db", backups[2].Name, "Oldest backup should be last")
	})
}

func TestBackupValidation(t *testing.T) {
	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	installer := NewInstaller(logger)
	tempDir := t.TempDir()
	
	t.Run("ValidateNonexistentFileReturnsError", func(t *testing.T) {
		nonexistentPath := filepath.Join(tempDir, "nonexistent.db")
		
		err := installer.ValidateBackup(nonexistentPath)
		
		assert.Error(t, err, "Should error when backup file doesn't exist")
		assert.Contains(t, err.Error(), "cannot access backup", "Error should indicate file access issue")
	})

	t.Run("ValidateEmptyFileReturnsError", func(t *testing.T) {
		emptyBackup := filepath.Join(tempDir, "empty.db")
		err := os.WriteFile(emptyBackup, []byte{}, 0644)
		require.NoError(t, err)
		
		err = installer.ValidateBackup(emptyBackup)
		
		assert.Error(t, err, "Should error when backup file is empty")
		assert.Contains(t, err.Error(), "backup file is empty", "Error should indicate empty file")
	})
}
