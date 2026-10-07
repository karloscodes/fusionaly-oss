package events

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
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
// IngestedEvents, earliest event time first, in batches of batchSize. Call it again until
// it returns no events to drain a larger backlog.
func ProcessUnprocessedEvents(dbManager cartridge.DBManager, logger *slog.Logger, batchSize int) (*EventProcessingResult, error) {
	db := dbManager.GetConnection()
	result := &EventProcessingResult{
		ProcessedEvents: make([]*Event, 0),
		ProcessingData:  make([]*EventProcessingData, 0),
	}

	var tempEvents []IngestedEvent
	// Event time order, so a visit's events are counted in the order they
	// happened, also when the requests arrived in another order.
	err := db.Where("processed = ?", statusUnprocessed).
		Order("timestamp asc, id asc").
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

batches:
	for i := 0; i < len(tempEvents); i += batchSize {
		end := min(i+batchSize, len(tempEvents))
		batch := tempEvents[i:end]

		err := processBatchInWrite(dbManager, logger, batch, result)
		if errors.Is(err, sqlite.ErrBusy) {
			// The database is busy, not the events. They stay unprocessed
			// for the next run.
			logger.Warn("Database busy; leaving the rest for the next run", slog.Int("start", i))
			break
		}
		if err != nil {
			// One bad event must not block its whole batch forever. Retry the
			// events one by one and set aside the ones that still fail.
			logger.Error("Failed to process batch; retrying its events one by one",
				slog.Int("start", i), slog.Int("end", end), slog.Any("error", err))
			for _, event := range batch {
				err := processBatchInWrite(dbManager, logger, []IngestedEvent{event}, result)
				if errors.Is(err, sqlite.ErrBusy) {
					logger.Warn("Database busy; leaving the rest for the next run", slog.Uint64("ingested_event_id", uint64(event.ID)))
					break batches
				}
				if err != nil {
					markFailed(dbManager, logger, event.ID, err)
				}
			}
		}
	}

	logger.Info("Processed events",
		slog.Int("processed", len(result.ProcessedEvents)),
		slog.Int("total", len(tempEvents)))
	return result, nil
}

// processBatchInWrite processes one batch in a queued write transaction and,
// after the commit, adds its events to result.
func processBatchInWrite(dbManager cartridge.DBManager, logger *slog.Logger, batch []IngestedEvent, result *EventProcessingResult) error {
	var events []*Event
	var processingData []*EventProcessingData
	err := cartridge.Write(context.Background(), dbManager, func(tx *gorm.DB) error {
		var err error
		events, processingData, err = processEventBatch(tx, logger, batch)
		return err
	})
	if err != nil {
		return err
	}
	result.ProcessedEvents = append(result.ProcessedEvents, events...)
	result.ProcessingData = append(result.ProcessingData, processingData...)
	return nil
}

// markFailed sets an event aside so the next run does not retry it forever.
func markFailed(dbManager cartridge.DBManager, logger *slog.Logger, id uint, cause error) {
	logger.Error("Setting aside an event that cannot be processed",
		slog.Uint64("ingested_event_id", uint64(id)), slog.Any("error", cause))
	err := cartridge.Write(context.Background(), dbManager, func(tx *gorm.DB) error {
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
		// A headless browser can send a normal User-Agent, but its client
		// hints still name it.
		if parsedUA.Bot || strings.Contains(tempEvent.SecChUa, "HeadlessChrome") {
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

		visit, err := visitStatus(tx, tempEvent.WebsiteID, tempEvent.UserSignature, tempEvent.Timestamp)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to check visitor and session status: %w", err)
		}
		tempEvent.Timestamp = visit.at

		if tempEvent.EventType == EventTypePageHide {
			if err := storePageHide(tx, &tempEvent, visit); err != nil {
				return nil, nil, err
			}
			continue
		}

		event := &Event{
			SessionStart:     visit.sessionStart,
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
		data, err := prepareEventProcessingData(tx, &tempEvent, event.ID, parsedUA, visit)
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

// storePageHide stores a page hide in its visit. A page hide only extends a
// visit that is still open, so a hide after the session timeout, or without
// an earlier event, is dropped. It updates no counter.
func storePageHide(tx *gorm.DB, tempEvent *IngestedEvent, visit visit) error {
	if visit.isNewSession {
		return nil
	}
	event := &Event{
		SessionStart:  visit.sessionStart,
		WebsiteID:     tempEvent.WebsiteID,
		UserSignature: tempEvent.UserSignature,
		Hostname:      tempEvent.Hostname,
		Pathname:      tempEvent.Pathname,
		EventType:     EventTypePageHide,
		Timestamp:     tempEvent.Timestamp,
		CreatedAt:     tempEvent.CreatedAt,
	}
	if err := tx.Create(event).Error; err != nil {
		return fmt.Errorf("failed to create page hide: %w", err)
	}
	return nil
}

// prepareEventProcessingData enriches event data for aggregation
// Accepts the pre-parsed useragent.UserAgent struct
func prepareEventProcessingData(db *gorm.DB, tempEvent *IngestedEvent, eventID uint, parsedUA ua.UserAgent, visit visit) (*EventProcessingData, error) {
	isNewVisitor, isNewSession := visit.isNewVisitor, visit.isNewSession
	var err error

	// For custom events, override isNewVisitor to check if this is the first time the visitor triggered this specific event
	if tempEvent.EventType == EventTypeCustomEvent {
		isNewVisitor, err = checkIsNewEventVisitor(db, tempEvent.WebsiteID, tempEvent.UserSignature, tempEvent.CustomEventName, tempEvent.Timestamp, eventID)
		if err != nil {
			return nil, fmt.Errorf("failed to check event-specific visitor status: %w", err)
		}
	}

	isPageView := tempEvent.EventType == EventTypePageView
	// A page view is its visit's exit until a later page view in the visit
	// arrives; that later page view then corrects it (see visitCorrections).
	isExit := isPageView

	var isNewPageVisitor, isBounce bool
	var previousPageView *PageViewRef
	var unbounceAt *time.Time
	if isPageView {
		// Page-view counters count a visitor at their first page view and a
		// visit at its first page view. A custom event before it counts in
		// neither, so a visit that opens with a custom event still counts.
		isNewVisitor, err = checkIsFirstPageView(db, tempEvent.WebsiteID, tempEvent.UserSignature, tempEvent.Timestamp, eventID)
		if err != nil {
			return nil, fmt.Errorf("failed to check visitor status: %w", err)
		}
		isNewPageVisitor, err = checkIsNewPageVisitor(db, tempEvent.WebsiteID, tempEvent.UserSignature, tempEvent.Hostname, tempEvent.Pathname, tempEvent.Timestamp, eventID)
		if err != nil {
			return nil, fmt.Errorf("failed to check page visitor status: %w", err)
		}
		if visit.sessionStart != nil {
			so, err := visitSoFar(db, tempEvent.WebsiteID, tempEvent.UserSignature, *visit.sessionStart, tempEvent.Timestamp, eventID)
			if err != nil {
				return nil, fmt.Errorf("failed to check the visit's earlier events: %w", err)
			}
			isNewSession = len(so.pageViews) == 0
			isBounce = isNewSession && !so.engaged
			if len(so.pageViews) > 0 {
				last := so.pageViews[0]
				previousPageView = &PageViewRef{Hostname: last.Hostname, Pathname: last.Pathname, Timestamp: last.Timestamp}
			}
			unbounceAt = so.bounceToTakeBack()
		}
	} else if isEngagement(tempEvent.CustomEventName) && visit.sessionStart != nil {
		so, err := visitSoFar(db, tempEvent.WebsiteID, tempEvent.UserSignature, *visit.sessionStart, tempEvent.Timestamp, eventID)
		if err != nil {
			return nil, fmt.Errorf("failed to check the visit's earlier events: %w", err)
		}
		unbounceAt = so.bounceToTakeBack()
	}
	isEntrance := isNewSession && isPageView

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

			// Keep only the parameters that name a traffic source. Others can
			// hold personal data (?email=, ?token=) or a unique ID per click
			// (?fbclid=), which must not be stored.
			for _, key := range sourceQueryParams {
				if value := parsedURL.Query().Get(key); value != "" {
					queryParams[key] = value
				}
			}
		}
	}

	customEventKey := ""
	if tempEvent.EventType == EventTypeCustomEvent {
		customEventKey = tempEvent.CustomEventName
	}

	hasUTM := utmSource != EmptyUTMAttr || utmMedium != EmptyUTMAttr || utmCampaign != EmptyUTMAttr

	// The source of a visit: utm_source, then ref, then the referrer. Whoever
	// tagged the link named the source; a tagged link without a referrer
	// (an email, an app) is not direct traffic.
	sourceHostname, sourcePathname := tempEvent.ReferrerHostname, tempEvent.ReferrerPathname
	if utmSource != EmptyUTMAttr {
		sourceHostname, sourcePathname = utmSource, ""
	} else if ref := queryParams["ref"]; ref != "" {
		sourceHostname, sourcePathname = ref, ""
	}

	return &EventProcessingData{
		EventID:          eventID,
		WebsiteID:        tempEvent.WebsiteID,
		UserSignature:    tempEvent.UserSignature,
		Hostname:         tempEvent.Hostname,
		Pathname:         tempEvent.Pathname,
		ReferrerHostname: sourceHostname,
		ReferrerPathname: sourcePathname,
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
		IsNewPageVisitor: isNewPageVisitor,
		HasUTM:           hasUTM,
		PreviousPageView: previousPageView,
		UnbounceAt:       unbounceAt,
		IsBounce:         isBounce,
	}, nil
}

// visit describes where an event falls in its visitor's day. The rules:
//   - a visit is a run of events, of any type, with no gap longer than the
//     session timeout
//   - an event never falls before the visitor's latest processed event: an
//     event that arrives late moves to that event's time, so it extends the
//     visit and is counted once
type visit struct {
	at           time.Time  // the event's time, after the late-arrival rule
	isNewVisitor bool       // the visitor's first event
	isNewSession bool       // the visit's first event
	sessionStart *time.Time // nil when the visit began before visits were recorded
}

// visitStatus decides the visit of an event from the visitor's latest
// processed event. A visitor is new on their first event (the signature
// rotates daily, so in practice their first event of the UTC day).
func visitStatus(db *gorm.DB, websiteID uint, userSignature string, timestamp time.Time) (visit, error) {
	sessionTimeout := time.Duration(config.GetConfig().SessionTimeoutSeconds) * time.Second
	at := timestamp.UTC()

	var latest Event
	err := db.Where("website_id = ? AND user_signature = ?", websiteID, userSignature).
		Order("timestamp DESC, id DESC").
		Limit(1).
		First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		start := at
		return visit{at: at, isNewVisitor: true, isNewSession: true, sessionStart: &start}, nil
	}
	if err != nil {
		return visit{}, fmt.Errorf("failed to query previous event: %w", err)
	}

	if latest.Timestamp.After(at) {
		at = latest.Timestamp.UTC()
	}
	if at.Sub(latest.Timestamp) > sessionTimeout {
		start := at
		return visit{at: at, isNewSession: true, sessionStart: &start}, nil
	}
	if latest.SessionStart == nil {
		return visit{at: at}, nil // the visit began before visits were recorded
	}
	sessionStart := latest.SessionStart.UTC()
	return visit{at: at, sessionStart: &sessionStart}, nil
}

// The checks below run after the event is stored, so they skip it by ID.
// Events at the same time count as earlier: the late-arrival rule can give
// two events the same time.

// checkIsFirstPageView reports whether this is the visitor's first page view.
func checkIsFirstPageView(db *gorm.DB, websiteID uint, userSignature string, timestamp time.Time, eventID uint) (bool, error) {
	var count int64
	err := db.Model(&Event{}).
		Where("website_id = ? AND user_signature = ? AND event_type = ? AND timestamp <= ? AND id != ?",
			websiteID, userSignature, EventTypePageView, timestamp, eventID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check previous page views: %w", err)
	}
	return count == 0, nil
}

// checkIsNewPageVisitor reports whether this is the visitor's first view of
// the page, so a page's visitors count each visitor once.
func checkIsNewPageVisitor(db *gorm.DB, websiteID uint, userSignature, hostname, pathname string, timestamp time.Time, eventID uint) (bool, error) {
	var count int64
	err := db.Model(&Event{}).
		Where("website_id = ? AND user_signature = ? AND event_type = ? AND hostname = ? AND pathname = ? AND timestamp <= ? AND id != ?",
			websiteID, userSignature, EventTypePageView, hostname, pathname, timestamp, eventID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check previous page views: %w", err)
	}
	return count == 0, nil
}

// isEngagement reports whether a custom event shows the visitor interacted:
// any custom event except the SDK's automatic scroll events.
// The prefix check is case-sensitive, like the SQL check in visitSoFar
// (substr, not LIKE, which ignores case): the SDK sends lowercase names.
func isEngagement(customEventName string) bool {
	return !strings.HasPrefix(customEventName, "scroll:")
}

// visitProgress is what a visit holds before an event. The visit's counts
// are made as its events arrive, so the processor cannot know yet whether
// more will follow:
//   - the visit's latest page view was counted as its exit; a later page
//     view takes that back
//   - a visit is counted as a bounce at its first page view, unless it had
//     engagement before; a second page view or the first engagement takes
//     the bounce back
type visitProgress struct {
	pageViews []Event // the latest two, latest first
	engaged   bool
}

// bounceToTakeBack returns the bucket time of the visit's bounce when this
// event is the one that takes it back, or nil.
func (v visitProgress) bounceToTakeBack() *time.Time {
	if len(v.pageViews) != 1 || v.engaged {
		return nil
	}
	at := v.pageViews[0].Timestamp
	return &at
}

func visitSoFar(db *gorm.DB, websiteID uint, userSignature string, sessionStart, timestamp time.Time, eventID uint) (visitProgress, error) {
	var progress visitProgress
	inVisit := db.Where("website_id = ? AND user_signature = ? AND session_start = ? AND timestamp <= ? AND id != ?",
		websiteID, userSignature, sessionStart.UTC(), timestamp, eventID)

	err := db.Model(&Event{}).Select("hostname", "pathname", "timestamp").
		Where(inVisit).Where("event_type = ?", EventTypePageView).
		Order("timestamp DESC, id DESC").
		Limit(2).
		Find(&progress.pageViews).Error
	if err != nil {
		return progress, err
	}

	var engagements int64
	err = db.Model(&Event{}).
		Where(inVisit).Where("event_type = ? AND substr(custom_event_name, 1, 7) != 'scroll:'", EventTypeCustomEvent).
		Count(&engagements).Error
	progress.engaged = engagements > 0
	return progress, err
}

// checkIsNewEventVisitor checks if this is the first time a visitor triggers a specific custom event
func checkIsNewEventVisitor(db *gorm.DB, websiteID uint, userSignature, eventName string, timestamp time.Time, eventID uint) (bool, error) {
	var count int64
	err := db.Model(&Event{}).
		Where("website_id = ? AND user_signature = ? AND event_type = ? AND custom_event_name = ? AND timestamp <= ? AND id != ?",
			websiteID, userSignature, EventTypeCustomEvent, eventName, timestamp, eventID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check previous custom event: %w", err)
	}
	return count == 0, nil
}

// sourceQueryParams are the query parameters stored in query_param_stats.
// UTM parameters have their own table.
var sourceQueryParams = []string{"ref", "source", "via"}

func getUTMParam(parsedURL *url.URL, param string) string {
	if value := parsedURL.Query().Get(param); value != "" {
		return value
	}
	return EmptyUTMAttr
}
