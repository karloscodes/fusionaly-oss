package mcp_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each AI client reads its own manifest. They must agree on name and version,
// or a release updates one client and leaves the others behind.
func TestPluginManifestsAgree(t *testing.T) {
	type manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Plugins []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"plugins"`
	}
	read := func(path string) manifest {
		b, err := os.ReadFile("../../" + path)
		require.NoError(t, err, path)
		var m manifest
		require.NoError(t, json.Unmarshal(b, &m), path)
		return m
	}

	claude := read("plugin/.claude-plugin/plugin.json")
	for _, path := range []string{"plugin/.codex-plugin/plugin.json", "plugin/.cursor-plugin/plugin.json", "gemini-extension.json"} {
		m := read(path)
		assert.Equal(t, claude.Name, m.Name, path)
		assert.Equal(t, claude.Version, m.Version, path)
	}
	for _, path := range []string{".claude-plugin/marketplace.json", ".cursor-plugin/marketplace.json"} {
		m := read(path)
		require.Len(t, m.Plugins, 1, path)
		assert.Equal(t, claude.Name, m.Plugins[0].Name, path)
		assert.Equal(t, claude.Version, m.Plugins[0].Version, path)
	}
}
