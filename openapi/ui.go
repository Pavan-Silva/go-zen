package openapi

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/Pavan-Silva/go-zen/system"
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
  <p>The embedded Swagger UI assets are unavailable, so the documentation page is running in a minimal fallback mode.</p>
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
		return
	}
	uiAssets = http.FS(sub)

	data, err := uiDist.ReadFile("ui/dist/index.html")
	if err != nil {
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

	return fmt.Sprintf(uiTemplate, o.cfg.SpecPath, system.Version)
}
