package jobs

import (
	"log/slog"

	"fusionaly/internal/database"
)

// Jobs is an alias for Scheduler to maintain backwards compatibility
// Deprecated: Use Scheduler directly for new code
type Jobs = Scheduler

// NewJobs creates a new job scheduler (backwards compatibility wrapper)
// Deprecated: Use NewScheduler directly for new code
func NewJobs(dbManager *database.DBManager, logger *slog.Logger) (*Jobs, error) {
	return NewScheduler(dbManager, logger)
}
