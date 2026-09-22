package events

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"log/slog"

	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	ua "fusionaly/internal/pkg/user_agent"
)

// EventProcessingResult holds the results of batch event processing
type EventProcessingResult struct {
	ProcessedEvents []*Event
	ProcessingData  []*EventProcessingData
	Fetched         int // ingested events this run looked at, bots included; 0 means drained
}

// MaxEventsPerRun bounds one ProcessUnprocessedEvents call, so a backlog
// (after downtime, or a traffic flood) is drained over several runs instead
// of loaded into memory at once.
const MaxEventsPerRun = 5000

// Values of IngestedEvent.Processed.
const (
	statusUnprocessed = 0
	statusProcessed   = 1
	statusFailed      = 2 // could not be processed; kept for inspection, then cleaned up
)

// ProcessUnprocessedEvents processes up to MaxEventsPerRun unprocessed
// IngestedEvents, oldest first, in batches of batchSize. Call it again until
// it returns no events to drain a larger backlog.
func ProcessUnprocessedEvents(dbManager cartridge.DBManager, logger *slog.Logger, batchSize int) (*EventProcessingResult, error) {
	db := dbManager.GetConnection()
	result := &EventProcessingResult{
		ProcessedEvents: make([]*Event, 0),
		ProcessingData:  make([]*EventProcessingData, 0),
	}

	var tempEvents []IngestedEvent
	err := db.Where("processed = ?", statusUnprocessed).
		Order("created_at asc, id asc").
		Limit(MaxEventsPerRun).
		Find(&tempEvents).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unprocessed events: %w", err)
	}

	if len(tempEvents) == 0 {
		logger.Info("No unprocessed events found")
		return result, nil
	}

	result.Fetched = len(tempEvents)
	logger.Info("Processing unprocessed events", slog.Int("total", len(tempEvents)))

	for i := 0; i < len(tempEvents); i += batchSize {
		end := min(i+batchSize, len(tempEvents))
		batch := tempEvents[i:end]

		if err := processBatchInWrite(db, logger, batch, result); err != nil {
			// One bad event must not block its whole batch forever. Retry the
			// events one by one and set aside the ones that still fail.
			logger.Error("Failed to process batch; retrying its events one by one",
				slog.Int("start", i), slog.Int("end", end), slog.Any("error", err))
			for _, event := range batch {
				if err := processBatchInWrite(db, logger, []IngestedEvent{event}, result); err != nil {
					markFailed(db, logger, event.ID, err)
				}
			}
		}
	}

	logger.Info("Processed events",
		slog.Int("processed", len(result.ProcessedEvents)),
		slog.Int("total", len(tempEvents)))
	return result, nil
}

// processBatchInWrite processes one batch in a serialized write transaction
// and adds its events to result.
func processBatchInWrite(db *gorm.DB, logger *slog.Logger, batch []IngestedEvent, result *EventProcessingResult) error {
	return sqlite.PerformWrite(logger, db, func(tx *gorm.DB) error {
		events, processingData, err := processEventBatch(tx, logger, batch)
		if err != nil {
			return err
		}
		result.ProcessedEvents = append(result.ProcessedEvents, events...)
		result.ProcessingData = append(result.ProcessingData, processingData...)
		return nil
	})
}

// markFailed sets an event aside so the next run does not retry it forever.
func markFailed(db *gorm.DB, logger *slog.Logger, id uint, cause error) {
	logger.Error("Setting aside an event that cannot be processed",
		slog.Uint64("ingested_event_id", uint64(id)), slog.Any("error", cause))
	err := sqlite.PerformWrite(logger, db, func(tx *gorm.DB) error {
		return tx.Model(&IngestedEvent{}).Where("id = ?", id).Update("processed", statusFailed).Error
	})
	if err != nil {
		logger.Error("Failed to set aside event", slog.Uint64("ingested_event_id", uint64(id)), slog.Any("error", err))
	}
}

// processEventBatch processes a batch of IngestedEvents within a transaction
func processEventBatch(tx *gorm.DB, logger *slog.Logger, batch []IngestedEvent) ([]*Event, []*EventProcessingData, error) {
	var events []*Event
	var processingData []*EventProcessingData

	for i, tempEvent := range batch {
		// Parse User Agent early to check for bots
		parsedUA := ua.ParseUserAgent(tempEvent.UserAgent)
		if parsedUA.Bot {
			logger.Debug("Skipping bot event", slog.Uint64("ingested_event_id", uint64(uint64(tempEvent.ID))), slog.String("user_agent", tempEvent.UserAgent))
			continue // Skip processing for bots
		}

		// Add debug logging for first few events in batch
		if i < 3 {
			logger.Debug("Processing event timestamp",
				slog.Time("raw_timestamp", tempEvent.Timestamp),
				slog.String("formatted_timestamp", tempEvent.Timestamp.Format(time.RFC3339)),
				slog.String("timestamp_utc", tempEvent.Timestamp.UTC().Format(time.RFC3339)))
		}

		event := &Event{
			WebsiteID:        tempEvent.WebsiteID,
			UserSignature:    tempEvent.UserSignature,
			Hostname:         tempEvent.Hostname,
			Pathname:         tempEvent.Pathname,
			ReferrerHostname: tempEvent.ReferrerHostname,
			ReferrerPathname: tempEvent.ReferrerPathname,
			EventType:        tempEvent.EventType,
			CustomEventName:  tempEvent.CustomEventName,
			CustomEventMeta:  tempEvent.CustomEventMeta,
			Timestamp:        tempEvent.Timestamp,
			CreatedAt:        tempEvent.CreatedAt,
		}

		if err := tx.Create(event).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to create event: %w", err)
		}

		// Pass the already parsed UA struct
		data, err := prepareEventProcessingData(tx, &tempEvent, event.ID, parsedUA)
		if err != nil {
			logger.Error("Failed to prepare processing data", slog.Uint64("id", uint64(uint64(tempEvent.ID))), slog.Any("error", err))
			return nil, nil, fmt.Errorf("failed to prepare processing data: %w", err)
		}

		events = append(events, event)
		processingData = append(processingData, data)
	}

	// Update aggregates for the batch using the provided function
	if len(processingData) > 0 { // Only update aggregates if there are non-bot events
		if err := UpdateAllAggregatesBatch(tx, logger, processingData); err != nil {
			return nil, nil, fmt.Errorf("failed to update aggregates: %w", err)
		}
	}

	// Mark all events in the batch (including skipped bots) as processed using their IDs
	var eventIDs []uint
	for _, tempEvent := range batch {
		eventIDs = append(eventIDs, tempEvent.ID)
	}
	if len(eventIDs) > 0 {
		if err := tx.Model(&IngestedEvent{}).Where("id IN ?", eventIDs).Update("processed", statusProcessed).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to mark events as processed: %w", err)
		}
	}

	return events, processingData, nil
}

// prepareEventProcessingData enriches event data for aggregation
// Accepts the pre-parsed useragent.UserAgent struct
func prepareEventProcessingData(db *gorm.DB, tempEvent *IngestedEvent, eventID uint, parsedUA ua.UserAgent) (*EventProcessingData, error) {
	// Unified check for first-ever event and new session (used for page views and most aggregates)
	isNewVisitor, isNewSession, err := checkVisitorAndSessionStatus(db, tempEvent.WebsiteID, tempEvent.UserSignature, tempEvent.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to check visitor and session status: %w", err)
	}

	// For custom events, override isNewVisitor to check if this is the first time the visitor triggered this specific event
	if tempEvent.EventType == EventTypeCustomEvent {
		isNewVisitor, err = checkIsNewEventVisitor(db, tempEvent.WebsiteID, tempEvent.UserSignature, tempEvent.CustomEventName, tempEvent.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to check event-specific visitor status: %w", err)
		}
	}

	isExit, err := checkIsExitEvent(db, tempEvent.WebsiteID, tempEvent.UserSignature, tempEvent.EventType, tempEvent.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to check if exit event: %w", err)
	}

	isEntrance := isNewSession && tempEvent.EventType == EventTypePageView

	utmSource, utmMedium, utmCampaign, utmTerm, utmContent := EmptyUTMAttr, EmptyUTMAttr, EmptyUTMAttr, EmptyUTMAttr, EmptyUTMAttr
	queryParams := make(map[string]string)

	if tempEvent.RawURL != "" {
		parsedURL, err := url.Parse(tempEvent.RawURL)
		if err == nil {
			utmSource = getUTMParam(parsedURL, "utm_source")
			utmMedium = getUTMParam(parsedURL, "utm_medium")
			utmCampaign = getUTMParam(parsedURL, "utm_campaign")
			utmTerm = getUTMParam(parsedURL, "utm_term")
			utmContent = getUTMParam(parsedURL, "utm_content")

			// Extract ALL query parameters
			for key, values := range parsedURL.Query() {
				if len(values) > 0 && values[0] != "" {
					queryParams[key] = values[0] // Take first value if multiple
				}
			}
		}
	}

	customEventKey := ""
	if tempEvent.EventType == EventTypeCustomEvent {
		customEventKey = tempEvent.CustomEventName
	}

	hasUTM := utmSource != EmptyUTMAttr || utmMedium != EmptyUTMAttr || utmCampaign != EmptyUTMAttr

	return &EventProcessingData{
		EventID:          eventID,
		WebsiteID:        tempEvent.WebsiteID,
		UserSignature:    tempEvent.UserSignature,
		Hostname:         tempEvent.Hostname,
		Pathname:         tempEvent.Pathname,
		ReferrerHostname: tempEvent.ReferrerHostname,
		ReferrerPathname: tempEvent.ReferrerPathname,
		DeviceType:       getDeviceTypeFromParsedUA(parsedUA),
		Browser:          getBrowserFromParsedUA(parsedUA, tempEvent.SecChUa),
		OperatingSystem:  getOSFromParsedUA(parsedUA),
		Country:          tempEvent.Country,
		UTMSource:        utmSource,
		UTMMedium:        utmMedium,
		UTMCampaign:      utmCampaign,
		UTMTerm:          utmTerm,
		UTMContent:       utmContent,
		QueryParams:      queryParams,
		CustomEventName:  tempEvent.CustomEventName,
		CustomEventKey:   customEventKey,
		EventType:        EventType(tempEvent.EventType),
		IsNewVisitor:     isNewVisitor,
		IsNewSession:     isNewSession,
		Timestamp:        tempEvent.Timestamp,
		IsEntrance:       isEntrance,
		IsExit:           isExit,
		IsBounce:         false,
		HasUTM:           hasUTM,
	}, nil
}

// checkVisitorAndSessionStatus determines if this is the first-ever event for a visitor
// and if it starts a new session based on any previous event.
// Note: For custom events, use checkIsNewEventVisitor to check event-specific visitor status.
func checkVisitorAndSessionStatus(db *gorm.DB, websiteID uint, userSignature string, timestamp time.Time) (isNewVisitor bool, isNewSession bool, err error) {
	sessionTimeout := config.GetConfig().SessionTimeoutSeconds

	var previousEvent Event
	qErr := db.Where("website_id = ? AND user_signature = ? AND timestamp < ?",
		websiteID, userSignature, timestamp).
		Order("timestamp DESC").
		Limit(1).
		First(&previousEvent).Error

	if qErr != nil {
		if errors.Is(qErr, gorm.ErrRecordNotFound) {
			return true, true, nil
		}
		return false, false, fmt.Errorf("failed to query previous event: %w", qErr)
	}

	isNewVisitor = false

	// A new session starts if the time since the last event exceeds sessionTimeout
	timeSinceLastEvent := timestamp.Sub(previousEvent.Timestamp)
	isNewSession = timeSinceLastEvent > time.Duration(sessionTimeout)*time.Second

	return isNewVisitor, isNewSession, nil
}

// checkIsNewEventVisitor checks if this is the first time a visitor triggers a specific custom event
func checkIsNewEventVisitor(db *gorm.DB, websiteID uint, userSignature, eventName string, timestamp time.Time) (bool, error) {
	var count int64
	err := db.Model(&Event{}).
		Where("website_id = ? AND user_signature = ? AND event_type = ? AND custom_event_name = ? AND timestamp < ?",
			websiteID, userSignature, EventTypeCustomEvent, eventName, timestamp).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check previous custom event: %w", err)
	}
	return count == 0, nil
}

func checkIsExitEvent(db *gorm.DB, websiteID uint, userSignature string, eventType EventType, timestamp time.Time) (bool, error) {
	sessionTimeout := config.GetConfig().SessionTimeoutSeconds
	endTime := timestamp.Add(time.Duration(sessionTimeout) * time.Second)

	var nextEventCount int64
	err := db.Model(&Event{}).
		Where("website_id = ? AND user_signature = ? AND timestamp > ? AND timestamp <= ?",
			websiteID, userSignature, timestamp, endTime).
		Count(&nextEventCount).Error

	return nextEventCount == 0, err
}

func getUTMParam(parsedURL *url.URL, param string) string {
	if value := parsedURL.Query().Get(param); value != "" {
		return value
	}
	return EmptyUTMAttr
}
