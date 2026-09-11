// Package scalar embeds the Scalar documentation UI assets and renders the
// HTML bootstrap for zen's OpenAPI serving (see zen.OpenAPI). The embed stays
// here so the UI files live next to the //go:embed directive; zen's
// EnableAPIDocs wires everything together.
package scalar

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen/internal/log"
	"github.com/Pavan-Silva/go-zen/internal/system"
)

//go:embed assets/dist/*
var uiDist embed.FS

var (
	uiAssets   http.FileSystem
	uiTemplate string
)

func init() {
	sub, err := fs.Sub(uiDist, "assets/dist")
	if err != nil {
		// Embedded at compile time; unreachable unless the build is broken.
		log.Debug("scalar: embedded UI assets unavailable: %v", err)
		return
	}
	uiAssets = http.FS(sub)

	data, err := uiDist.ReadFile("assets/dist/index.html")
	if err != nil {
		log.Debug("scalar: embedded index.html unavailable: %v", err)
		return
	}
	uiTemplate = string(data)
}

// UIHTML renders the documentation bootstrap page for the given spec path and
// UI init options.
func UIHTML(specPath string, opts map[string]any) string {
	// The embedded UI HTML may contain literal '%' characters (CSS, JS), so
	// positional format verbs must be substituted with strings.Replace instead
	// of fmt.Sprintf, which would corrupt them.
	html := strings.Replace(uiTemplate, "%[1]s", specPath, 1)
	html = strings.Replace(html, "%[2]s", system.Version, 1)

	if extraOpts := swaggerUIOptionsString(opts); extraOpts != "" {
		html = strings.Replace(html, "// ui-extra-options", extraOpts, 1)
	} else {
		html = strings.Replace(html, "// ui-extra-options", "", 1)
	}

	return html
}

// Assets returns the embedded documentation UI files.
func Assets() http.FileSystem {
	return uiAssets
}

// swaggerUIOptionsString serializes a map of UI init options (forwarded to
// Scalar.createApiReference) into indented JavaScript object-literal lines.
// Each value is marshaled as a JSON literal so that strings, booleans,
// numbers, arrays, and nested objects are all represented correctly. Returns
// an empty string when the map is empty.
func swaggerUIOptionsString(opts map[string]any) string {
	if len(opts) == 0 {
		return ""
	}
	var b strings.Builder
	first := true
	for k, v := range opts {
		j, err := json.Marshal(v)
		if err != nil {
			log.Debug("scalar: skipping UI option %q: %v", k, err)
			continue
		}
		if !first {
			b.WriteString(",\n        ")
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.Write(j)
		first = false
	}
	return b.String()
}
