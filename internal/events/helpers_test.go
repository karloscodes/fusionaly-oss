package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBrowserFromClientHints(t *testing.T) {
	t.Run("Chrome", func(t *testing.T) {
		header := `"Chromium";v="146", "Google Chrome";v="146", "Not-A.Brand";v="24"`

		assert.Equal(t, "chrome", parseBrowserFromClientHints(header))
	})

	t.Run("Brave", func(t *testing.T) {
		header := `"Chromium";v="146", "Brave";v="146", "Not-A.Brand";v="24"`

		assert.Equal(t, "brave", parseBrowserFromClientHints(header))
	})

	t.Run("Edge", func(t *testing.T) {
		header := `"Chromium";v="146", "Microsoft Edge";v="146", "Not-A.Brand";v="24"`

		assert.Equal(t, "microsoft edge", parseBrowserFromClientHints(header))
	})

	t.Run("Opera", func(t *testing.T) {
		header := `"Chromium";v="146", "Opera";v="100", "Not-A.Brand";v="24"`

		assert.Equal(t, "opera", parseBrowserFromClientHints(header))
	})

	t.Run("Vivaldi", func(t *testing.T) {
		header := `"Chromium";v="146", "Vivaldi";v="7.0", "Not-A.Brand";v="24"`

		assert.Equal(t, "vivaldi", parseBrowserFromClientHints(header))
	})

	t.Run("Arc", func(t *testing.T) {
		header := `"Chromium";v="146", "Arc";v="1.0", "Not-A.Brand";v="24"`

		assert.Equal(t, "arc", parseBrowserFromClientHints(header))
	})

	t.Run("empty header", func(t *testing.T) {
		assert.Equal(t, "", parseBrowserFromClientHints(""))
	})

	t.Run("only Chromium and grease", func(t *testing.T) {
		header := `"Chromium";v="146", "Not-A.Brand";v="24"`

		assert.Equal(t, "", parseBrowserFromClientHints(header))
	})

	t.Run("unknown Chromium fork", func(t *testing.T) {
		header := `"Chromium";v="146", "SomeBrowser";v="1.0", "Not-A.Brand";v="24"`

		assert.Equal(t, "somebrowser", parseBrowserFromClientHints(header))
	})
}
