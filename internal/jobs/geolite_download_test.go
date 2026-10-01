package jobs

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"fusionaly/internal/config"
)

type unreachableNetwork struct{}

func (unreachableNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestGeoLiteDownloadError(t *testing.T) {
	t.Run("with a network error, the error does not contain the license key", func(t *testing.T) {
		original := geoliteClient
		geoliteClient = &http.Client{Transport: unreachableNetwork{}}
		job := &GeoLiteUpdaterJob{cfg: &config.Config{GeoDBPath: filepath.Join(t.TempDir(), "GeoLite2-City.mmdb")}}

		err := job.downloadAndUpdate("secret-license-key")

		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "secret-license-key")
		assert.NotContains(t, err.Error(), "license_key")
		assert.Contains(t, err.Error(), "connection refused")

		geoliteClient = original
	})
}
