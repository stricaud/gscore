package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stricaud/gscore/internal/format"
	_ "github.com/stricaud/gscore/internal/format/musescore"
)

func TestZZDbg(t *testing.T) {
	path := os.Getenv("PROBE")
	if path == "" {
		t.Skip("no PROBE")
	}
	imp, _ := format.ImporterFor(path)
	f, _ := os.Open(path)
	defer f.Close()
	s, _ := imp.Import(f)
	a := New(s, "")
	mux := http.NewServeMux()
	a.routes(mux)
	start := time.Now()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/map", nil))
	t.Logf("took %v, %d bytes", time.Since(start), rec.Body.Len())
	t.Logf("%s", rec.Body.String())
}
