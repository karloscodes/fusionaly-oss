package cron

import (
	"testing"
	"fusionaly/internal/manager/logging"
)

func testLogger(t *testing.T) *logging.Logger {
	dir := t.TempDir()
	return logging.NewLogger(logging.Config{LogDir: dir})
}

func TestNewManager_Defaults(t *testing.T) {
	mgr := NewManager(testLogger(t))
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.cronFile != DefaultCronFile {
		t.Errorf("cronFile = %q, want %q", mgr.cronFile, DefaultCronFile)
	}
	if mgr.installDir != DefaultInstallDir {
		t.Errorf("installDir = %q, want %q", mgr.installDir, DefaultInstallDir)
	}
	if mgr.binaryPath != DefaultBinaryPath {
		t.Errorf("binaryPath = %q, want %q", mgr.binaryPath, DefaultBinaryPath)
	}
	if mgr.schedule != DefaultCronSchedule {
		t.Errorf("schedule = %q, want %q", mgr.schedule, DefaultCronSchedule)
	}
}
