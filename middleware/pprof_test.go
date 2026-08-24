package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pavan-Silva/go-zen"
)

// Regression: with a custom prefix, profile endpoints used to be registered
// with pprof.Index, which resolves profile names by trimming a hardcoded
// "/debug/pprof/" prefix - custom prefixes silently returned the HTML index
// page instead of the requested profile.
func TestPprof_CustomPrefixServesProfiles(t *testing.T) {
	r := zen.New(":0")
	RegisterPprofWithConfig(r, PprofConfig{Prefix: "/prof"})

	for _, path := range []string{"/prof/heap", "/prof/goroutine", "/prof/allocs"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		ct := w.Header().Get("Content-Type")
		if w.Code != http.StatusOK || strings.Contains(ct, "text/html") {
			t.Errorf("%s: expected profile data, got status=%d content-type=%q", path, w.Code, ct)
		}
	}
}

func TestPprof_DefaultPrefixStillWorks(t *testing.T) {
	r := zen.New(":0")
	RegisterPprof(r)

	for _, path := range []string{"/debug/pprof/", "/debug/pprof/heap", "/debug/pprof/cmdline"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: unexpected status %d", path, w.Code)
		}
	}
}
