# Changelog

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
