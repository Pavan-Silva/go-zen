# Changelog

## v1.4.0

### Features

- `BindPathParams`, `BindQueryParams`, `BindHeader` auto-validation — all 8 bind methods now validate when auto-validation is enabled
- `c.Validate()` on `*Ctx` — explicit per-request validation, nil-safe when no validator is set
- `e.Validate()` on `*Engine` — standalone validation for use outside request handlers
- `ClientIP()` on `*Ctx` — checks `X-Forwarded-For` → `X-Real-IP` → `RemoteAddr`
- `Params()` on `*Ctx` — returns all matched path params as `map[string]string`
- `QueryParams()` on `*Ctx` — returns all query params as `map[string][]string`
- `DefaultQuery(key, default)` on `*Ctx` — fallback when query param is absent or empty
- `DefaultParam(key, default)` on `*Ctx` — fallback when path param is absent or empty
- `HeaderXRequestID` constant on `zen` package — canonical request ID header name
- Request ID middleware — injects unique ID per request via configurable header
- Request timeout middleware — `middleware.Timeout()` with configurable duration and skipper
- Template rendering — `LoadTemplates()` and `c.Render()` / `RenderWriter()` for HTML templates
- `JSONPretty()` on `*Ctx` — indented JSON response with two-space indentation
- `IsAjax()` on `*Ctx` — checks `X-Requested-With: XMLHttpRequest` header
- `ContentType()` on `*Ctx` — returns parsed MIME content type without parameters
- Testing utilities — `NewTestRequest()` and `NewTestRequestWithBody()`

### Breaking

- Removed package-level `zen.Validate()`, `zen.SetValidator()`, `zen.EnableAutoValidation()`, `zen.DefaultValidator()` — use per-instance equivalents on `*Engine` instead
- Validator moved from `global/atomic.Pointer` to plain `Validator` field on `Engine` — match Echo's pattern, do not mutate after server start
- All validation types and methods consolidated into `validate.go`

### Fixes

- Removed validator mutex — `Validate` now uses atomic `sync.Uint32` for race-free config reinit
- Request ID middleware uses `zen.HeaderXRequestID` constant

### Documentation

- Updated routing docs with `Params`, `QueryParams`, `DefaultQuery`, `DefaultParam`, `ClientIP`
- Updated request-id docs to show response header retrieval pattern (`c.Response.Header().Get(zen.HeaderXRequestID)`)
- Updated context docs to reference `zen.HeaderXRequestID`
- Added request-id, timeout, and middleware docs for all new features
- Updated responses.mdx with Template and JSONPretty
- Updated routing.mdx with IsAjax and ContentType
- Updated middleware.mdx with full example stack

## v1.3.0

### Features

- Custom radix tree router — replaces the old router with per-method radix tree for zero-allocation path params, backtracking, trailing-slash redirect, and 405 detection
- PBAC (Policy-Based Access Control) support in auth package
- OpenAPI auto-generation with embedded Swagger UI for offline access
- Custom validator registration support
- Swagger UI extra options and update workflow
- RequireClaim helper for JWT auth

### Fixes

- radix tree panic under specific route patterns
- Graceful shutdown signal leak
- SSE event terminator — browser EventSource no longer buffers indefinitely
- Data race in context.Get()/Keys() — nil/len checks moved inside RLock
- Data race on defaultLogger — migrated to sync/atomic.Pointer
- url.QueryUnescape → url.PathUnescape for path param decoding (fixes `+` decoded as space)
- HEAD fallback params contamination — saved/restored psLen/skipLen around HEAD tree lookup
- Content type header allocation improvements
- Optimized auto-validation path
- Simplified validation logic
- Graceful handling of missing Swagger UI assets
- Header binding merged with existing binding logic

### Performance

- Zero-allocation routing path
- Logger delegates to slog.Log() — removes one allocation per call and fragile skip-depth
- Console handler replaces slog.NewTextHandler — level-colored output, sync.Pool buffer reuse, zero additional allocations
- Optimized CORS, SSE, and rate-limit middleware
- Reduced response header allocations

### Logger

- Colored output with custom consoleHandler (FATAL=bold red, ERROR=red, WARN=yellow, INFO=green, DEBUG/TRACE=gray)
- Timestamp added to middleware logs (`[ZEN] 2026/07/10 - 10:04:04`)
- Column-aligned output (rightPad/leftPad) for status, duration, IP, method
- Terminal detection via ModeCharDevice + NO_COLOR env var support
- slog levels: FATAL, ERROR, WARN, INFO, DEBUG, TRACE

### Documentation

- Comprehensive doc comments across all packages (system, env, openapi, auth, logger, middleware)
- Fixed factual errors in router.go comments
- Updated README with features, installation, and quick start

### Maintenance

- Upgraded dependencies
- Updated naming conventions across codebase
- Refactored RBAC and auth implementations
