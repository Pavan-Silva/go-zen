package openapi

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/Pavan-Silva/go-zen/system"
)

//go:embed ui/dist/*
var uiDist embed.FS

var uiAssets http.FileSystem
var uiTemplate string

func init() {
	sub, err := fs.Sub(uiDist, "ui/dist")
	if err != nil {
		panic(err)
	}
	uiAssets = http.FS(sub)

	data, err := uiDist.ReadFile("ui/dist/index.html")
	if err != nil {
		panic(err)
	}
	uiTemplate = string(data)
}

func uiHTML(o *OpenAPI) string {
	return fmt.Sprintf(uiTemplate, o.cfg.SpecPath, system.Version)
}
