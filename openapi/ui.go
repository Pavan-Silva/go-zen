package openapi

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen/internal/log"
	"github.com/Pavan-Silva/go-zen/internal/system"
)

const fallbackUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>OpenAPI Documentation</title>
  <style>body{font-family:system-ui,sans-serif;max-width:720px;margin:2rem auto;padding:0 1rem;line-height:1.5}</style>
</head>
<body>
  <h1>OpenAPI Documentation</h1>
  <p>The embedded documentation UI assets are unavailable, so the
  documentation page is running in a minimal fallback mode.</p>
  <p>OpenAPI spec: <a href="%s">%s</a></p>
  <p>go-zen version %s</p>
</body>
</html>`

//go:embed ui/dist/*
var uiDist embed.FS

var uiAssets http.FileSystem
var uiTemplate = fallbackUIHTML

func init() {
	uiTemplate = fallbackUIHTML

	sub, err := fs.Sub(uiDist, "ui/dist")
	if err != nil {
		log.Debug("openapi: embedded UI assets unavailable, using fallback UI: %v", err)
		return
	}
	uiAssets = http.FS(sub)

	data, err := uiDist.ReadFile("ui/dist/index.html")
	if err != nil {
		log.Debug("openapi: embedded index.html unavailable, using fallback UI: %v", err)
		return
	}
	uiTemplate = string(data)
}

func uiHTML(o *OpenAPI) string {
	if uiTemplate == "" {
		uiTemplate = fallbackUIHTML
	}

	if uiTemplate == fallbackUIHTML {
		return fmt.Sprintf(fallbackUIHTML, o.cfg.SpecPath, o.cfg.SpecPath, system.Version)
	}

	// The embedded UI HTML may contain literal '%' characters (CSS, JS), so
	// positional format verbs must be substituted with strings.Replace instead
	// of fmt.Sprintf, which would corrupt them.
	html := strings.Replace(uiTemplate, "%[1]s", o.cfg.SpecPath, 1)
	html = strings.Replace(html, "%[2]s", system.Version, 1)

	if extraOpts := swaggerUIOptionsString(o.cfg.SwaggerUIOptions); extraOpts != "" {
		html = strings.Replace(html, "// ui-extra-options", extraOpts, 1)
	} else {
		html = strings.Replace(html, "// ui-extra-options", "", 1)
	}

	return html
}

// swaggerUIOptionsString serializes a map of UI init options (forwarded to
// Scalar.createApiReference) into indented JavaScript object-literal lines. Each value is marshaled as a
// JSON literal so that strings, booleans, numbers, arrays, and nested objects
// are all represented correctly. Returns empty string when the map is empty.
func swaggerUIOptionsString(opts map[string]any) string {
	if len(opts) == 0 {
		return ""
	}
	var b strings.Builder
	first := true
	for k, v := range opts {
		j, err := json.Marshal(v)
		if err != nil {
			log.Debug("openapi: skipping UI option %q: %v", k, err)
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
