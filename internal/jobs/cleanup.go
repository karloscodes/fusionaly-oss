package jobs

import (
	"log/slog"
	"time"

	"fusionaly/internal/config"
	"fusionaly/internal/database"
	"fusionaly/internal/events"
)

// CleanupJob handles cleanup of old ingested events
type CleanupJob struct {
	dbManager *database.DBManager
	logger    *slog.Logger
	cfg       *config.Config
}

func NewCleanupJob(dbManager *database.DBManager, logger *slog.Logger, cfg *config.Config) *CleanupJob {
	return &CleanupJob{
		dbManager: dbManager,
		logger:    logger,
		cfg:       cfg,
	}
}

// Run removes processed ingested events older than the retention period.
// This helps with GDPR data minimization and reduces storage usage.
func (j *CleanupJob) Run() error {
	retentionDays := j.cfg.IngestedEventsRetentionDays
	db := j.dbManager.GetConnection()
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	j.logger.Info("Starting cleanup of old ingested events",
		slog.Int("retention_days", retentionDays),
		slog.Time("cutoff_date", cutoffDate))

	// Count events to be deleted first
	var countToDelete int64
	if err := db.Model(&events.IngestedEvent{}).
		Where("processed = 1 AND created_at < ?", cutoffDate).
		Count(&countToDelete).Error; err != nil {
		j.logger.Error("Failed to count old ingested events", slog.Any("error", err))
		return err
	}

	if countToDelete == 0 {
		j.logger.Debug("No old ingested events to clean up")
		return nil
	}

	// Delete in batches to avoid locking the database for too long
	batchSize := 1000
	totalDeleted := int64(0)

	for {
		result := db.Where("processed = 1 AND created_at < ?", cutoffDate).
			Limit(batchSize).
			Delete(&events.IngestedEvent{})

		if result.Error != nil {
			j.logger.Error("Failed to delete old ingested events",
				slog.Any("error", result.Error),
				slog.Int64("deleted_so_far", totalDeleted))
			return result.Error
		}

		totalDeleted += result.RowsAffected

		if result.RowsAffected < int64(batchSize) {
			break
		}

		// Small delay between batches to prevent database lock contention
		time.Sleep(100 * time.Millisecond)
	}

	j.logger.Info("Cleaned up old ingested events",
		slog.Int64("deleted_count", totalDeleted),
		slog.Int("retention_days", retentionDays))

	return nil
}
