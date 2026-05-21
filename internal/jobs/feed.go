package jobs

import (
	"log/slog"
	"time"

	"fusionaly/internal/database"
	"fusionaly/internal/feed"
)

const (
	// FeedRetention is how long to keep feed items before cleanup
	FeedRetention = 90 * 24 * time.Hour // 90 days

	// BaselineRetention is how long to keep unused baselines
	BaselineRetention = 90 * 24 * time.Hour // 90 days
)

// FeedJob runs the activity feed detector on a schedule.
type FeedJob struct {
	dbManager *database.DBManager
	logger    *slog.Logger
}

func NewFeedJob(dbManager *database.DBManager, logger *slog.Logger) *FeedJob {
	return &FeedJob{
		dbManager: dbManager,
		logger:    logger,
	}
}

// Run runs a single feed detection cycle: detect events, learn baselines from
// yesterday's data, then clean up stale items and baselines.
func (j *FeedJob) Run() error {
	db := j.dbManager.GetConnection()

	j.logger.Info("Running feed detection...")

	detector := feed.NewDetector(db, j.logger)
	if err := detector.DetectAll(); err != nil {
		j.logger.Error("Feed detection failed", slog.Any("error", err))
	}

	// Learn baselines from yesterday's data
	if err := feed.LearnFromYesterday(db); err != nil {
		j.logger.Error("Baseline learning failed", slog.Any("error", err))
	}

	// Cleanup old data
	if err := feed.CleanupOldItems(db, FeedRetention); err != nil {
		j.logger.Error("Feed cleanup failed", slog.Any("error", err))
	}
	if err := feed.CleanupStaleBaselines(db, BaselineRetention); err != nil {
		j.logger.Error("Baseline cleanup failed", slog.Any("error", err))
	}

	return nil
}
