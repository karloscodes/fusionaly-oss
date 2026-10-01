package events

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"fusionaly/internal/config"
)

// SessionKeys returns one opaque key per event, in the same order. Events of
// one visit get the same key. The key does not show the user signature.
//
// An event with a session start belongs to the visit (signature, session
// start). An event from before session starts existed has none. For those
// events, a gap longer than the session timeout starts a new visit.
func SessionKeys(list []Event) []string {
	keys := make([]string, len(list))
	var legacy []int

	for i, event := range list {
		if event.SessionStart == nil {
			legacy = append(legacy, i)
			continue
		}
		keys[i] = sessionKey("visit", event.WebsiteID, event.UserSignature, *event.SessionStart)
	}

	timeout := time.Duration(config.GetConfig().SessionTimeoutSeconds) * time.Second
	sort.SliceStable(legacy, func(a, b int) bool {
		ea, eb := list[legacy[a]], list[legacy[b]]
		if ea.WebsiteID != eb.WebsiteID {
			return ea.WebsiteID < eb.WebsiteID
		}
		if ea.UserSignature != eb.UserSignature {
			return ea.UserSignature < eb.UserSignature
		}
		return ea.Timestamp.Before(eb.Timestamp)
	})

	var start, previous Event
	for n, i := range legacy {
		event := list[i]
		sameVisitor := n > 0 && event.WebsiteID == previous.WebsiteID && event.UserSignature == previous.UserSignature
		if !sameVisitor || event.Timestamp.Sub(previous.Timestamp) > timeout {
			start = event
		}
		keys[i] = sessionKey("gap", event.WebsiteID, event.UserSignature, start.Timestamp)
		previous = event
	}

	return keys
}

func sessionKey(kind string, websiteID uint, signature string, start time.Time) string {
	input := fmt.Sprintf("%s|%d|%s|%s", kind, websiteID, signature, start.UTC().Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:8])
}
