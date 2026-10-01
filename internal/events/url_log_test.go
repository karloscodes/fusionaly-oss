package events_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

func TestInvalidPageURLLogging(t *testing.T) {
	sentAt := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	collect := func(t *testing.T, rawURL string) (string, error) {
		t.Helper()
		dbManager, _ := testsupport.SetupTestDBManager(t)
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		input := &events.CollectEventInput{
			IPAddress:  "203.0.113.1",
			UserAgent:  "Mozilla/5.0 (test)",
			EventType:  events.EventTypePageView,
			Timestamp:  sentAt,
			ReceivedAt: sentAt,
			RawUrl:     rawURL,
		}

		err := events.CollectEvent(dbManager, logger, input)

		return logs.String(), err
	}

	t.Run("with an unparsable URL, the log and the error have no query", func(t *testing.T) {
		logs, err := collect(t, "https://example.com/account/%zz?token=abc123")

		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "token=abc123")
		assert.NotContains(t, logs, "token=abc123")
	})

	t.Run("with a URL without host, the log has a short value without query", func(t *testing.T) {
		logs, err := collect(t, "/"+strings.Repeat("a", 100)+"?token=abc123")

		assert.Error(t, err)
		assert.NotContains(t, logs, "token=abc123")
		assert.NotContains(t, logs, strings.Repeat("a", 65))
		assert.Contains(t, logs, strings.Repeat("a", 60))
	})
}
