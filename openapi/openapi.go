package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/Pavan-Silva/go-zen"
)

// Config configures the OpenAPI spec generation and UI.
type Config struct {
	Title           string                    // API title (required).
	Version         string                    // API version (required).
	Description     string                    // Optional API description.
	SpecPath        string                    // Path to serve spec JSON (default "/openapi.json").
	DocPath         string                    // Path to serve docs UI (default "/docs").
	DisableUI       bool                      // When true, no documentation UI is served.
	SecuritySchemes map[string]SecurityScheme // Security schemes for components/securitySchemes.
	DefaultSecurity []map[string][]string     // Default security requirements applied to all routes.
}

// RouteInfo holds OpenAPI metadata for a route.
type RouteInfo struct {
	Summary     string
	Description string
	Tags        []string
	RequestBody any
	Responses   map[int]any
	Deprecated  bool
	Security    []map[string][]string // nil inherits DefaultSecurity; empty means no auth
}

// RouteInfoBuilder builds a RouteInfo via method chaining.
type RouteInfoBuilder struct {
	info RouteInfo
}

// RI starts a new RouteInfo builder chain.
func RI() *RouteInfoBuilder { return &RouteInfoBuilder{} }

// Summary sets the route summary.
func (b *RouteInfoBuilder) Summary(s string) *RouteInfoBuilder { b.info.Summary = s; return b }

// Desc sets the route description.
func (b *RouteInfoBuilder) Desc(s string) *RouteInfoBuilder { b.info.Description = s; return b }

// Tags sets the route tags.
func (b *RouteInfoBuilder) Tags(tags ...string) *RouteInfoBuilder { b.info.Tags = tags; return b }

// Resp adds a response with the given status code and model type.
// Pass nil for model to indicate no body (e.g., 204 No Content).
func (b *RouteInfoBuilder) Resp(status int, model any) *RouteInfoBuilder {
	if b.info.Responses == nil {
		b.info.Responses = make(map[int]any)
	}

	b.info.Responses[status] = model
	return b
}

// Body sets the request body type.
func (b *RouteInfoBuilder) Body(model any) *RouteInfoBuilder { b.info.RequestBody = model; return b }

// Deprecated marks the route as deprecated.
func (b *RouteInfoBuilder) Deprecated() *RouteInfoBuilder { b.info.Deprecated = true; return b }

// Security sets a security requirement for this route.
// Scopes are for OAuth2 schemes; omit for apiKey/http schemes.
// Pass an empty name to mark the route as having no security (overrides default).
func (b *RouteInfoBuilder) Security(name string, scopes ...string) *RouteInfoBuilder {
	if name == "" {
		b.info.Security = []map[string][]string{}
		return b
	}
	b.info.Security = []map[string][]string{{name: scopes}}
	return b
}

// OpenAPI manages OpenAPI 3.0.3 spec generation for a zen engine.
type OpenAPI struct {
	cfg    Config
	mu     sync.RWMutex
	routes map[string]map[string]RouteInfo
	spec   []byte
}

// New creates an OpenAPI instance with the given configuration.
func New(cfg Config) *OpenAPI {
	if cfg.SpecPath == "" {
		cfg.SpecPath = "/openapi.json"
	}

	if cfg.DocPath == "" {
		cfg.DocPath = "/docs"
	}

	return &OpenAPI{
		cfg:    cfg,
		routes: make(map[string]map[string]RouteInfo),
	}
}

// Register enriches a route with OpenAPI metadata.
func (o *OpenAPI) Register(method, path string, b *RouteInfoBuilder) {
	info := b.info
	o.mu.Lock()
	if o.routes[method] == nil {
		o.routes[method] = make(map[string]RouteInfo)
	}

	existing := o.routes[method][path]
	if info.Summary != "" {
		existing.Summary = info.Summary
	}

	if info.Description != "" {
		existing.Description = info.Description
	}

	if info.Tags != nil {
		existing.Tags = info.Tags
	}

	if info.RequestBody != nil {
		existing.RequestBody = info.RequestBody
	}

	if info.Responses != nil {
		existing.Responses = info.Responses
	}

	existing.Deprecated = info.Deprecated
	if info.Security != nil {
		existing.Security = info.Security
	}
	o.routes[method][path] = existing
	o.spec = nil
	o.mu.Unlock()
}

// SpecJSON returns the generated OpenAPI spec as JSON bytes.
func (o *OpenAPI) SpecJSON() []byte {
	o.mu.RLock()
	spec := o.spec
	o.mu.RUnlock()
	if spec != nil {
		return spec
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if o.spec != nil {
		return o.spec
	}

	data := o.generate()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		b = fmt.Appendf(nil, `{"error":"%s"}`, err.Error())
	}
	o.spec = b
	return b
}

// SpecHandler returns a HandlerFunc that serves /openapi.json.
func (o *OpenAPI) SpecHandler() zen.HandlerFunc {
	return func(c *zen.Ctx) {
		c.Response.Header()["Content-Type"] = []string{"application/json"}
		c.Response.WriteHeader(http.StatusOK)
		c.Response.Write(o.SpecJSON())
	}
}

// DocHandler returns a HandlerFunc that serves the API documentation UI.
// Returns a 404 handler when DisableUI is true.
func (o *OpenAPI) DocHandler() zen.HandlerFunc {
	if o.cfg.DisableUI {
		return func(c *zen.Ctx) {
			c.Response.WriteHeader(http.StatusNotFound)
		}
	}
	html := uiHTML(o)
	return func(c *zen.Ctx) {
		c.Response.Header()["Content-Type"] = []string{"text/html; charset=utf-8"}
		c.Response.WriteHeader(http.StatusOK)
		c.Response.Write([]byte(html))
	}
}

// RegisterRoutes registers the spec endpoint and optionally the docs UI on the engine.
// The docs UI is skipped when DisableUI is true.
func (o *OpenAPI) RegisterRoutes(r *zen.Engine) {
	r.HandleRaw("GET "+o.cfg.SpecPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(o.SpecJSON())
	}))

	if !o.cfg.DisableUI {
		docHTML := uiHTML(o)
		r.HandleRaw("GET "+o.cfg.DocPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(docHTML))
		}))

		if uiAssets != nil {
			r.HandleRaw("GET /openapi/swagger-ui/{path...}", http.StripPrefix("/openapi/swagger-ui/", http.FileServer(uiAssets)))
		}
	}
}

// GET enriches a GET route with OpenAPI metadata.
func (o *OpenAPI) GET(path string, b *RouteInfoBuilder) { o.Register("GET", path, b) }

// POST enriches a POST route with OpenAPI metadata.
func (o *OpenAPI) POST(path string, b *RouteInfoBuilder) { o.Register("POST", path, b) }

// PUT enriches a PUT route with OpenAPI metadata.
func (o *OpenAPI) PUT(path string, b *RouteInfoBuilder) { o.Register("PUT", path, b) }

// DELETE enriches a DELETE route with OpenAPI metadata.
func (o *OpenAPI) DELETE(path string, b *RouteInfoBuilder) { o.Register("DELETE", path, b) }

// PATCH enriches a PATCH route with OpenAPI metadata.
func (o *OpenAPI) PATCH(path string, b *RouteInfoBuilder) { o.Register("PATCH", path, b) }

// HEAD enriches a HEAD route with OpenAPI metadata.
func (o *OpenAPI) HEAD(path string, b *RouteInfoBuilder) { o.Register("HEAD", path, b) }

// OPTIONS enriches an OPTIONS route with OpenAPI metadata.
func (o *OpenAPI) OPTIONS(path string, b *RouteInfoBuilder) { o.Register("OPTIONS", path, b) }

// generate builds the full OpenAPI document map.
func (o *OpenAPI) generate() map[string]any {
	sb := newSchemaBuilder()
	info := map[string]any{
		"title":   o.cfg.Title,
		"version": o.cfg.Version,
	}

	if o.cfg.Description != "" {
		info["description"] = o.cfg.Description
	}

	doc := map[string]any{
		"openapi": "3.0.3",
		"info":    info,
		"paths":   o.buildPaths(sb),
	}

	components := make(map[string]any)
	if schemas := sb.build(); len(schemas) > 0 {
		components["schemas"] = schemas
	}
	if sec := o.buildSecuritySchemes(); len(sec) > 0 {
		components["securitySchemes"] = sec
	}
	if len(components) > 0 {
		doc["components"] = components
	}
	if len(o.cfg.DefaultSecurity) > 0 {
		doc["security"] = o.cfg.DefaultSecurity
	}

	return doc
}

// buildPaths constructs the paths map from registered routes.
func (o *OpenAPI) buildPaths(sb *schemaBuilder) map[string]any {
	paths := make(map[string]any)
	uniquePaths := make(map[string]bool)

	for method := range o.routes {
		for path := range o.routes[method] {
			uniquePaths[path] = true
		}
	}

	for path := range uniquePaths {
		oapiPath := path
		params := o.buildParams(path)
		pathItem := make(map[string]any)

		for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
			info, ok := o.routes[method][path]
			if !ok {
				continue
			}

			op := map[string]any{
				"responses": make(map[string]any),
			}
			if params != nil {
				op["parameters"] = params
			}

			if info.Summary != "" {
				op["summary"] = info.Summary
			}

			if info.Description != "" {
				op["description"] = info.Description
			}

			if len(info.Tags) > 0 {
				op["tags"] = info.Tags
			}

			if info.Deprecated {
				op["deprecated"] = true
			}

			if info.Security != nil {
				op["security"] = info.Security
			}

			if info.RequestBody != nil {
				op["requestBody"] = map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": sb.schemaOf(info.RequestBody),
						},
					},
				}
			}

			responses := make(map[string]any)
			if info.Responses != nil {
				for status, model := range info.Responses {
					resp := map[string]any{
						"description": http.StatusText(status),
					}
					if model != nil {
						resp["content"] = map[string]any{
							"application/json": map[string]any{
								"schema": sb.schemaOf(model),
							},
						}
					}
					responses[fmt.Sprintf("%d", status)] = resp
				}
			}

			if len(responses) == 0 {
				responses["200"] = map[string]any{"description": "OK"}
			}

			op["responses"] = responses
			pathItem[strings.ToLower(method)] = op
		}

		if len(pathItem) > 0 {
			paths[oapiPath] = pathItem
		}
	}
	return paths
}

// buildParams extracts path parameters from a route pattern.
func (o *OpenAPI) buildParams(path string) []map[string]any {
	n := strings.Count(path, "{")
	if n == 0 {
		return nil
	}
	params := make([]map[string]any, 0, n)
	for seg := range strings.SplitSeq(path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			name := seg[1 : len(seg)-1]
			params = append(params, map[string]any{
				"name":     name,
				"in":       "path",
				"required": true,
				"schema": map[string]any{
					"type": "string",
				},
			})
		}
	}
	return params
}
