package jobs

import (
	"log/slog"

	"fusionaly/internal/analytics"
	"fusionaly/internal/database"
	"fusionaly/internal/events"
	"fusionaly/internal/pkg/geoip"
)

// EventProcessorJob handles processing of ingested events
type EventProcessorJob struct {
	dbManager *database.DBManager
	logger    *slog.Logger
}

func NewEventProcessorJob(dbManager *database.DBManager, logger *slog.Logger) *EventProcessorJob {
	return &EventProcessorJob{
		dbManager: dbManager,
		logger:    logger,
	}
}

// Run processes unprocessed events from the ingest database
func (j *EventProcessorJob) Run() error {
	j.logger.Info("Starting event processing")

	// Check if GeoLite database is available - required for event processing
	if geoip.GetGeoDB() == nil {
		j.logger.Warn("GeoLite database not configured - events will remain queued. " +
			"Configure GeoLite in Administration > System or set FUSIONALY_GEO_DB_PATH")
		return nil
	}

	db := j.dbManager.GetConnection()

	// Count unprocessed events
	var unprocessedCount int64
	if err := db.Model(&events.IngestedEvent{}).Where("processed = 0").Count(&unprocessedCount).Error; err != nil {
		j.logger.Error("Failed to count unprocessed events", slog.Any("error", err))
		return err
	}

	j.logger.Info("Found unprocessed events", slog.Int64("count", unprocessedCount))

	if unprocessedCount == 0 {
		return nil
	}

	// Process the events
	result, err := events.ProcessUnprocessedEvents(j.dbManager, j.logger, 100)
	if err != nil {
		j.logger.Error("Failed to process events", slog.Any("error", err))
		return err
	}

	processedCount := 0
	if result != nil {
		processedCount = len(result.ProcessedEvents)
		// Log details about processed events (first 5 only)
		for i, event := range result.ProcessedEvents {
			if i < 5 {
				j.logger.Info("Processed event",
					slog.Uint64("id", uint64(uint64(event.ID))),
					slog.Uint64("websiteID", uint64(event.WebsiteID)),
					slog.String("hostname", event.Hostname),
					slog.String("path", event.Pathname),
					slog.Time("timestamp", event.Timestamp))
			}
		}
	}

	// Log event table stats
	var eventCount int64
	if err := db.Model(&events.Event{}).Count(&eventCount).Error; err != nil {
		j.logger.Error("Failed to count events", slog.Any("error", err))
	} else {
		j.logger.Info("Total events in Event table", slog.Int64("count", eventCount))
	}

	// Check for SiteStat entries
	var siteStatCount int64
	if err := db.Model(&analytics.SiteStat{}).Count(&siteStatCount).Error; err != nil {
		j.logger.Error("Failed to count site stats", slog.Any("error", err))
	} else {
		j.logger.Info("Total records in SiteStat table", slog.Int64("count", siteStatCount))
	}

	j.logger.Info("Events processed",
		slog.Int("count", processedCount),
		slog.Int64("remaining", unprocessedCount-int64(processedCount)))

	// Compute flow transitions for recent hours
	if err := events.ComputeFlowTransitionsForRecentHours(db, j.logger, 2, 5); err != nil {
		j.logger.Warn("Failed to compute flow transitions", slog.Any("error", err))
	}

	return nil
}
