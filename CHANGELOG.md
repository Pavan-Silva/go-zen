# Changelog

All notable changes to this project are documented in this file.

## Unreleased

### Breaking

- Removed `zen.FromContext` — use `zen.FromRequest(r)`, which is equivalent
- Removed `auth.Middleware` — use `auth.RequireAuth` or `auth.MiddlewareWithSkipper`
- Removed `auth.WithAuth` — use `auth.WithAuthFunc`, which additionally provides the authenticated user
- Removed `auth.Authorities` — declare authority slices directly (`[]string{...}`)

### Fixed

- `Group("/")` no longer panics with an index-out-of-range error; empty and "/" prefixes now inherit the parent group prefix unchanged instead of producing double-slash route paths

### Maintenance

- auth: RBAC, ABAC, and PBAC middlewares now share a single `authorize` helper instead of repeating the user lookup / 403 boilerplate
- router: radix tree internals refactored — traversal no longer reassigns method receivers (explicit local cursors), the triplicated backtracking loop extracted into `backtrack`, and the duplicated param-append block into `appendParamValue`; no behavior change

## v1.4.0

Consolidates all work since the v1.3.0 release, including the framework-wide audit.

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
- **OIDC fail-closed**: removed the swallowed `errOIDCJWTVerificationSkipped` path — when `SkipTokenVerification` is false, `Authenticate` now requires `KeyFunc` and returns `ErrOIDCKeyFuncNotConfigured` instead of silently accepting unverified tokens
- **Compress middleware**: teardown moved into a `defer` so a downstream panic (e.g. with `Recover`) yields a real 500 instead of a silent empty 200; `WriteHeader` is now first-wins; `Vary` uses `Add` instead of `Set`; `Flush()` switches to direct streaming so SSE events deliver immediately instead of being held behind the 1KB compression buffer; pooled writer is fully reset between requests; status-only responses (e.g. 204/304) are no longer rewritten to 200 by the deferred flush
- **CORS middleware**: `Vary: Origin` is appended with `Add` instead of `Set` so pre-existing `Vary` values (e.g. from the compress middleware or the handler) are preserved
- **Timeout middleware**: rewritten around a background timer — once the deadline passes the response is marked timed out, the next write emits a 504 Gateway Timeout, and the handler's own write path (or teardown) produces it, so no goroutine-per-request or concurrent writer access
- **Rate limiter**: `lastSeen` migrated to `atomic.Int64` so per-request hits no longer take the shard write lock; negative `Limit` now falls back to 100 instead of panicking in `x/time/rate`
- **Router**: 404/405/trailing-slash-redirect responses now run through the root middleware chain; `ServeHTTP` guards against a bad pooled type; missing-method route registration panics with a clear message
- **Bind**: recursion depth guard prevents stack overflow on self-referencing structs; nil-`Ctx` guards; path param map preallocated to param count; empty values skipped during unmarshal; struct fields that are unexported are skipped instead of panicking during binding (`reflect.Value.Interface` on unexported fields)
- **JSON/XML deserialize**: nil `Request.Body` no longer errors
- **Multipart**: nil `*FileHeader` entries skipped instead of panicking
- `Ctx.Copy()` — preserves the engine and copies matched path params so copies stay request-independent
- `Ctx.Keys()` — data race fixed (RLock covers nil/len checks)
- `Attachment()` — `Content-Disposition` filename URL-escaped to prevent header injection
- **OpenAPI UI**: literal `%` characters in the embedded Swagger UI HTML are no longer corrupted by `fmt.Sprintf` (replaced with `strings.Replace`)
- **OpenAPI paths**: catch-all route params (e.g. `{path...}`) now emit the param name without the trailing `...`
- **InMemorySessionStore**: background cleanup goroutine can be stopped via `StopCleanup()` so stores created with a TTL no longer leak a goroutine

### Performance

- Path param binding preallocates the params map to the matched param count

### Documentation

- Updated routing docs with `Params`, `QueryParams`, `DefaultQuery`, `DefaultParam`, `ClientIP`
- Updated request-id docs to show response header retrieval pattern (`c.Response.Header().Get(zen.HeaderXRequestID)`)
- Updated context docs to reference `zen.HeaderXRequestID`
- Added request-id, timeout, and middleware docs for all new features
- Updated responses docs with Template and JSONPretty
- Updated routing docs with IsAjax and ContentType
- Updated middleware docs with full example stack

### Testing

- Added regression tests for `time.Time` query binding via the `format` tag and for middleware registered after route registration

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
