package jobs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("download cut off") }

func TestWriteAtomically(t *testing.T) {
	t.Run("replaces the file without touching an open reader", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "GeoLite2-Country.mmdb")
		require.NoError(t, os.WriteFile(dest, []byte("old database"), 0o644))
		reader, err := os.Open(dest) // like the memory-mapped reader in use
		require.NoError(t, err)
		defer reader.Close()

		err = writeAtomically(dest, strings.NewReader("new database"))

		require.NoError(t, err)
		current, _ := os.ReadFile(dest)
		assert.Equal(t, "new database", string(current))
		old, _ := io.ReadAll(reader)
		assert.Equal(t, "old database", string(old), "the open reader keeps the old file")
		entries, _ := os.ReadDir(dir)
		assert.Len(t, entries, 1, "no temp file is left behind")
	})

	t.Run("keeps the old database when the download fails", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "GeoLite2-Country.mmdb")
		require.NoError(t, os.WriteFile(dest, []byte("old database"), 0o644))

		err := writeAtomically(dest, failingReader{})

		assert.Error(t, err)
		current, _ := os.ReadFile(dest)
		assert.Equal(t, "old database", string(current))
		entries, _ := os.ReadDir(dir)
		assert.Len(t, entries, 1, "no temp file is left behind")
	})
}
