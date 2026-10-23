package jobs

import (
	"log/slog"
	"time"

	"fusionaly/internal/config"
	"fusionaly/internal/database"
	"fusionaly/internal/events"
	"fusionaly/internal/pkg/batch"
	"fusionaly/internal/websites"
)

// CleanupJob deletes old queue rows, old raw events, and the data of deleted
// websites
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

// Run deletes old queue rows, old raw events, and the data of deleted
// websites.
func (j *CleanupJob) Run() error {
	if err := j.deleteOldQueueRows(); err != nil {
		return err
	}
	if err := j.deleteOldEvents(); err != nil {
		return err
	}
	return j.purgeDeletedWebsites()
}

// deleteOldQueueRows removes processed ingested events older than the
// retention period. This helps with GDPR data minimization and reduces
// storage usage.
func (j *CleanupJob) deleteOldQueueRows() error {
	retentionDays := j.cfg.IngestedEventsRetentionDays
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	j.logger.Info("Starting cleanup of old ingested events",
		slog.Int("retention_days", retentionDays),
		slog.Time("cutoff_date", cutoffDate))

	// Failed events (processed = 2) age out the same way.
	totalDeleted, err := batch.Delete(j.dbManager, `DELETE FROM ingested_events WHERE id IN (
		SELECT id FROM ingested_events WHERE processed IN (1, 2) AND created_at < ? LIMIT ?)`, cutoffDate)
	if err != nil {
		j.logger.Error("Failed to delete old ingested events",
			slog.Any("error", err),
			slog.Int64("deleted_so_far", totalDeleted))
		return err
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

// deleteOldEvents deletes the raw events older than
// FUSIONALY_EVENTS_RETENTION_DAYS, except the events with a price.
func (j *CleanupJob) deleteOldEvents() error {
	deleted, err := events.DeleteOldEvents(j.dbManager, time.Now())
	if err != nil {
		j.logger.Error("Failed to delete old events",
			slog.Any("error", err),
			slog.Int64("deleted_so_far", deleted))
		return err
	}
	if deleted > 0 {
		j.logger.Info("Deleted old events",
			slog.Int64("deleted_count", deleted),
			slog.Int("retention_days", j.cfg.EventsRetentionDays))
	}
	return nil
}

// purgeDeletedWebsites deletes the events, queue rows, and summaries of
// deleted websites. Deleting a website removes only the website itself.
func (j *CleanupJob) purgeDeletedWebsites() error {
	deleted, err := websites.PurgeDeletedWebsites(j.dbManager)
	if err != nil {
		j.logger.Error("Failed to delete the data of deleted websites",
			slog.Any("error", err),
			slog.Int64("deleted_so_far", deleted))
		return err
	}
	if deleted > 0 {
		j.logger.Info("Deleted the data of deleted websites", slog.Int64("deleted_count", deleted))
	}
	return nil
}
