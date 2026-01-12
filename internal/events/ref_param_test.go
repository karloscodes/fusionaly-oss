package events_test

import (
	"testing"
	"time"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefQueryParameterSupport(t *testing.T) {
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	testCases := []struct {
		name             string
		url              string
		httpReferrer     string
		expectedReferrer string
		expectedRefParam string
	}{
		{
			name:             "HTTP referrer is preserved and ref parameter is tracked separately",
			url:              "https://example.com/page?ref=producthunt",
			httpReferrer:     "https://google.com/search",
			expectedReferrer: "google.com",
			expectedRefParam: "producthunt",
		},
		{
			name:             "Ref parameter without HTTP referrer",
			url:              "https://example.com/page?ref=producthunt",
			httpReferrer:     "",
			expectedReferrer: events.DirectOrUnknownReferrer,
			expectedRefParam: "producthunt",
		},
		{
			name:             "No ref parameter - should be empty",
			url:              "https://example.com/page",
			httpReferrer:     "https://google.com",
			expectedReferrer: "google.com",
			expectedRefParam: events.EmptyUTMAttr,
		},
		{
			name:             "Empty ref parameter",
			url:              "https://example.com/page?ref=",
			httpReferrer:     "",
			expectedReferrer: events.DirectOrUnknownReferrer,
			expectedRefParam: events.EmptyUTMAttr,
		},
		{
			name:             "Ref parameter with special characters",
			url:              "https://example.com/page?ref=product-hunt",
			httpReferrer:     "",
			expectedReferrer: events.DirectOrUnknownReferrer,
			expectedRefParam: "product-hunt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testsupport.CleanAllTables(db)
			website := testsupport.CreateTestWebsite(db, "example.com")

			input := &events.CollectEventInput{
				IPAddress:       "203.0.113.1",
				UserAgent:       "Mozilla/5.0 (test)",
				ReferrerURL:     tc.httpReferrer,
				EventType:       events.EventTypePageView,
				CustomEventName: "",
				CustomEventMeta: "",
				Timestamp:       time.Now().UTC(),
				RawUrl:          tc.url,
			}

			// Collect the event
			err := events.CollectEvent(dbManager, logger, input)
			require.NoError(t, err)

			// Verify ingested event
			var ingestedEvent events.IngestedEvent
			err = db.Where("website_id = ?", website.ID).First(&ingestedEvent).Error
			require.NoError(t, err)
			assert.Equal(t, tc.expectedReferrer, ingestedEvent.ReferrerHostname,
				"Referrer hostname should match expected value")

			// Process the event
			_, err = events.ProcessUnprocessedEvents(dbManager, logger, 10)
			require.NoError(t, err)

			// Verify processed event data has ref_param extracted
			var processedEvent events.Event
			err = db.Where("website_id = ?", website.ID).First(&processedEvent).Error
			require.NoError(t, err)

			// Note: We can't directly check RefParam on the Event model since it's only in EventProcessingData
			// But we can verify it's tracked in query_param_stats if there's a ref parameter
			if tc.expectedRefParam != events.EmptyUTMAttr {
				// Check that query_param_stats was created with the ref parameter
				var count int64
				err = db.Table("query_param_stats").
					Where("website_id = ? AND param_name = ? AND param_value = ?", website.ID, "ref", tc.expectedRefParam).
					Count(&count).Error
				require.NoError(t, err)
				assert.Greater(t, count, int64(0), "query_param_stats should contain ref parameter entry")
			}
		})
	}
}

func TestAllQueryParametersTracked(t *testing.T) {
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	testsupport.CleanAllTables(db)
	website := testsupport.CreateTestWebsite(db, "example.com")

	// URL with multiple query parameters including UTM and custom ones
	testURL := "https://example.com/page?utm_source=google&utm_medium=cpc&ref=producthunt&via=twitter&gclid=abc123&custom_param=value"

	input := &events.CollectEventInput{
		IPAddress:       "203.0.113.1",
		UserAgent:       "Mozilla/5.0 (test)",
		ReferrerURL:     "",
		EventType:       events.EventTypePageView,
		CustomEventName: "",
		CustomEventMeta: "",
		Timestamp:       time.Now().UTC(),
		RawUrl:          testURL,
	}

	// Collect and process the event
	err := events.CollectEvent(dbManager, logger, input)
	require.NoError(t, err)

	_, err = events.ProcessUnprocessedEvents(dbManager, logger, 10)
	require.NoError(t, err)

	// Verify ALL query parameters are tracked
	expectedParams := map[string]string{
		"utm_source":   "google",
		"utm_medium":   "cpc",
		"ref":          "producthunt",
		"via":          "twitter",
		"gclid":        "abc123",
		"custom_param": "value",
	}

	for paramName, expectedValue := range expectedParams {
		var stats []struct {
			ParamName  string
			ParamValue string
			Count      int64
		}

		err = db.Table("query_param_stats").
			Select("param_name, param_value, visitors_count as count").
			Where("website_id = ? AND param_name = ?", website.ID, paramName).
			Scan(&stats).Error
		require.NoError(t, err)

		assert.NotEmpty(t, stats, "Should have entry for parameter: %s", paramName)
		if len(stats) > 0 {
			assert.Equal(t, expectedValue, stats[0].ParamValue,
				"Parameter %s should have correct value", paramName)
			assert.Greater(t, stats[0].Count, int64(0),
				"Parameter %s should have visitor count", paramName)
		}
	}

	// Verify total number of distinct parameters tracked
	var distinctParamCount int64
	err = db.Table("query_param_stats").
		Where("website_id = ?", website.ID).
		Distinct("param_name").
		Count(&distinctParamCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(len(expectedParams)), distinctParamCount,
		"Should track all %d query parameters", len(expectedParams))
}
