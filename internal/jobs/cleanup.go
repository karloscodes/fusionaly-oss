package jobs

import (
	"log/slog"
	"time"

	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/database"
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

	// Delete in batches inside the serialized writer, so ingestion is never
	// blocked for long. GORM drops .Limit() on a SQLite DELETE, so the batch
	// is a subquery. Failed events (processed = 2) age out the same way.
	const batchSize = 1000
	totalDeleted := int64(0)

	for {
		var deleted int64
		err := sqlite.PerformWrite(j.logger, db, func(tx *gorm.DB) error {
			result := tx.Exec(`DELETE FROM ingested_events WHERE id IN (
				SELECT id FROM ingested_events WHERE processed IN (1, 2) AND created_at < ? LIMIT ?)`,
				cutoffDate, batchSize)
			deleted = result.RowsAffected
			return result.Error
		})
		if err != nil {
			j.logger.Error("Failed to delete old ingested events",
				slog.Any("error", err),
				slog.Int64("deleted_so_far", totalDeleted))
			return err
		}

		totalDeleted += deleted
		if deleted < batchSize {
			break
		}

		// Small delay between batches to prevent database lock contention
		time.Sleep(100 * time.Millisecond)
	}

	if totalDeleted == 0 {
		j.logger.Debug("No old ingested events to clean up")
		return nil
	}

	j.logger.Info("Cleaned up old ingested events",
		slog.Int64("deleted_count", totalDeleted),
		slog.Int("retention_days", retentionDays))

	return nil
}
