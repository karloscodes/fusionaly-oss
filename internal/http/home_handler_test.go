package http

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/testsupport"
	"github.com/stretchr/testify/assert"
)

func TestDemoIndexAction(t *testing.T) {
	t.Run("serves the demo page from any working directory", func(t *testing.T) {
		srv := testsupport.NewTestServer(t, testsupport.TestServerOptions{
			RouteMountFunc: func(s *cartridge.Server) { s.Get("/_demo", DemoIndexAction) },
		})
		t.Chdir(t.TempDir()) // the Docker image has no demo.html on disk

		resp, _ := srv.Server.Test(httptest.NewRequest("GET", "/_demo", nil))

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Contains(t, string(body), `<script defer src="/y/api/v1/sdk.js"></script>`)
	})
}
