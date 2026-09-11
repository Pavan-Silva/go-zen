package scalar

import (
	"strings"
	"testing"

	"github.com/Pavan-Silva/go-zen/internal/system"
)

func TestUIHTML(t *testing.T) {
	body := UIHTML("/openapi.json", nil)
	if !strings.Contains(body, "createApiReference") {
		t.Fatal("expected Scalar createApiReference bootstrap in HTML")
	}
	if !strings.Contains(body, `url: "/openapi.json"`) {
		t.Fatal("expected default spec URL in HTML")
	}
}

// Literal '%' characters in the embedded UI HTML must survive substitution
// untouched (fmt.Sprintf would corrupt them).
func TestUIHTMLLiteralPercent(t *testing.T) {
	prevTemplate := uiTemplate
	uiTemplate = `<html><div style="width:100%">url="%[1]s" v="%[2]s"</div></html>`
	t.Cleanup(func() { uiTemplate = prevTemplate })

	html := UIHTML("/spec.json", nil)

	if strings.Contains(html, "%!") {
		t.Fatalf("HTML corrupted by format-verb interpretation: %s", html)
	}
	if !strings.Contains(html, `url="/spec.json"`) || !strings.Contains(html, `v="`+system.Version+`"`) {
		t.Fatalf("placeholders not substituted: %s", html)
	}
	if !strings.Contains(html, "width:100%") {
		t.Fatalf("literal %% lost: %s", html)
	}
}

func TestAssets(t *testing.T) {
	if Assets() == nil {
		t.Fatal("expected embedded UI assets")
	}
	if _, err := Assets().Open("index.html"); err != nil {
		t.Fatalf("expected index.html in assets, got %v", err)
	}
}
