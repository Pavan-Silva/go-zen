# Zen

A small Go web framework built on the standard library `net/http`.
Zen provides routing, request binding, middleware, and simple helpers with a minimal API.

## Why Zen?

- Uses Go 1.22+ `http.ServeMux` with pattern matching
- Minimal surface area, no external router dependency
- Built-in JSON, form, and file binding
- Simple middleware chaining and graceful shutdown
- Supports SSE and WebSocket routes through lightweight helpers

## Features

### Core Framework

- Go 1.22+ pattern-based routing (no external router)
- Request/response binding (JSON, forms, files with 1MB limit)
- Struct validation with custom validators (`email`, `email-strict`)
- Route groups with prefix and middleware inheritance
- Graceful shutdown with signal handling (SIGINT/SIGTERM)
- Structured HTTP error responses (HTTPError type)
- Context pooling with zero-allocation operations
- Security hardened defaults (timeouts, header limits)

### Built-in Middleware

- **Logger** — Request logging with method, path, status, latency, IP, response size
- **Recover** — Panic recovery with stack trace logging
- **CORS** — Full CORS support with configurable origins, methods, headers

### Request/Response Features

- JSON binding with automatic validation: `c.BindJSON(&req)`
- Form binding (URL-encoded and multipart): `c.BindForm(&req)`
- Single file upload: `c.BindFile("fieldname")`
- Multiple file upload: `c.BindFiles("fieldname")`
- Query parameter caching: `c.QueryParam("key")`
- Path parameter extraction: `c.Param("id")`
- Raw body reading with DoS protection: `c.Body()`
- JSON responses with buffer pooling: `c.JSON(status, data)`
- Structured error responses: `c.SendError(err)`

### Performance Optimizations

- **Context pooling** — Reuses Context allocations (reduces GC pressure)
- **Inline context storage** — First 8 key-value pairs stored inline (no allocation)
- **Middleware pre-chaining** — Chain built at setup time (zero per-request overhead)
- **Form field caching** — Reflection metadata cached per struct type (sync.Map)
- **Query parameter caching** — Parsed once per request, lazily
- **JSON buffer pooling** — Response buffers reused via sync.Pool
- **Type-specialized form setters** — Type switch happens once per field type
- **Lazy validator initialization** — Validator created only on first use
- **No per-request closures** — Pre-built handlers and middleware chains

### Supported Bind Types

- **Primitives:** `string`, `bool`
- **Integers:** `int`, `int8`, `int16`, `int32`, `int64`
- **Unsigned:** `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- **Floats:** `float32`, `float64`

## Installation

```bash
go get github.com/Pavan-Silva/go-zen
```

## Quick Start

```go
package main

import (
    "net/http"
    "github.com/Pavan-Silva/go-zen"
    "github.com/Pavan-Silva/go-zen/middleware"
)

func main() {
    r := zen.New(":8080")
    r.Use(middleware.Logger, middleware.Recover)

    r.Handle("GET /hello", func(c *zen.Context) {
        c.JSON(http.StatusOK, map[string]string{"message": "Hello!"})
    })

    r.ListenAndServe()  // Blocks until Ctrl+C
}
```

## Core Components

### Router

`zen.New(addr)` creates a Router with security-hardened defaults:

- **Read timeout:** 5s (prevent slow-read attacks)
- **Write timeout:** 10s (prevent slow-write attacks)
- **Idle timeout:** 120s (free up idle connections)
- **Read header timeout:** 2s (prevent slowloris attacks)
- **Max header bytes:** 1MB (prevent memory exhaustion)

All timeouts are customizable via `r.Server.*`:

```go
r := zen.New(":8080")
r.Server.ReadTimeout = 10 * time.Second  // Custom timeout
r.ListenAndServe()
```

### Context

The request `Context` is passed to every handler and provides:

- Request/response data: `c.Request`, `c.Response`
- Query parameters: `c.QueryParam(key)`
- URL path parameters: `c.Param(key)`
- Request binding: `c.BindJSON()`, `c.BindForm()`, `c.BindFile()`
- Response methods: `c.JSON(status, data)`, `c.SSEvent(event, data)`, `c.SendError(err)`
- WebSocket routes: `r.HandleWebSocket("GET /ws", handler)`
- Key-value storage: `c.Set(key, val)`, `c.Get(key)`

Contexts are pooled and reused across requests for zero allocation overhead.

## Routing

Zen uses Go 1.22+ enhanced `http.ServeMux` with path parameters:

```go
// Static routes
r.Handle("GET /home", handleHome)
r.Handle("POST /users", handleCreateUser)

// URL path parameters (captured with {name})
r.Handle("GET /users/{id}", handleGetUser)
r.Handle("GET /posts/{slug}/comments/{id}", handleGetComment)

// Wildcard routes (captured as {path})
r.Handle("GET /files/*path", handleFiles)

// Raw http.Handler without Context wrapper
r.HandleRaw("GET /static/*", http.FileServer(http.Dir("static")))
```

## Streaming and WebSockets

Zen supports first-class server-sent events (SSE) and WebSocket routes.

### Server-Sent Events (SSE)

```go
r.Handle("GET /events", func(c *zen.Context) {
    if err := c.SSEvent("message", map[string]any{"text": "hello"}); err != nil {
        c.SendError(zen.InternalError(err.Error()))
    }
})
```

### WebSockets

```go
r.HandleWebSocket("GET /ws", func(c *zen.Context, ws *zen.WebSocketConn) {
    defer ws.Close()

    for {
        var msg map[string]any
        if err := ws.ReadJSON(&msg); err != nil {
            return
        }
        _ = ws.WriteJSON(map[string]any{"echo": msg})
    }
})
```

## Route Groups

Organize routes with common prefixes and middleware:

```go
r := zen.New(":8080")

// Public routes
public := r.Group("/api")
public.Handle("GET /health", healthHandler)
public.Handle("GET /version", versionHandler)

// Protected routes with auth middleware
auth := r.Group("/api", authMiddleware)
auth.Handle("GET /users", listUsers)
auth.Handle("GET /users/{id}", getUser)
auth.Handle("PUT /users/{id}", updateUser)

// Nested groups inherit parent middleware
admin := auth.Group("/admin", adminMiddleware)
admin.Handle("GET /dashboard", adminDashboard)
admin.Handle("POST /users", adminCreateUser)

// Add more middleware to a group
auth.Use(loggerMiddleware)

// Create sub-groups
posts := auth.Group("/posts")
posts.Use(postMiddleware)  // Additional middleware
posts.Handle("GET /{id}", getPost)
posts.Handle("POST /", createPost)
```

## Request Binding

### JSON Binding with Validation

```go
type SignupRequest struct {
    Username string `json:"username" validate:"required,alphanum,min=3,max=20"`
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    Age      int    `json:"age"      validate:"gte=18,lte=150"`
}

r.Handle("POST /signup", func(c *zen.Context) {
    var req SignupRequest
    if err := c.BindJSON(&req); err != nil {
        c.SendError(zen.BadRequest(err.Error()))
        return
    }
    // req is now validated and safe to use
    c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
})
```

### Form Binding

```go
type LoginForm struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
    RememberMe bool `json:"remember"`
}

r.Handle("POST /login", func(c *zen.Context) {
    var form LoginForm
    if err := c.BindForm(&form); err != nil {
        c.SendError(zen.BadRequest(err.Error()))
        return
    }
    // form is validated
    c.JSON(http.StatusOK, map[string]string{"token": "..."})
})
```

### File Upload

```go
r.Handle("POST /avatar", func(c *zen.Context) {
    // Single file
    file, content, err := c.BindFile("avatar")
    if err != nil {
        c.SendError(zen.BadRequest(err.Error()))
        return
    }
    // file.Filename, file.Size, len(content)
    c.JSON(http.StatusOK, map[string]string{"size": fmt.Sprintf("%d bytes", len(content))})
})

r.Handle("POST /attachments", func(c *zen.Context) {
    // Multiple files
    files, err := c.BindFiles("attachments")
    if err != nil {
        c.SendError(zen.BadRequest(err.Error()))
        return
    }
    for _, file := range files {
        fmt.Printf("%s: %d bytes\n", file.Header.Filename, len(file.Content))
    }
    c.JSON(http.StatusOK, map[string]int{"count": len(files)})
})
```

### Supported Types

- **Strings:** `string`
- **Numbers:** `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- **Floats:** `float32`, `float64`
- **Booleans:** `bool` (recognizes: "true", "TRUE", "1", "yes", "on", anything else is false)

Unknown types are silently ignored (field keeps zero value).

### Validation Tags

Zen uses [go-playground/validator](https://github.com/go-playground/validator) with custom email validators:

| Tag            | Example                   | Description                           |
| -------------- | ------------------------- | ------------------------------------- |
| `required`     | `validate:"required"`     | Field must not be empty               |
| `email`        | `validate:"email"`        | Valid email (regex pattern)           |
| `email-strict` | `validate:"email-strict"` | Valid email (RFC 5322 parser)         |
| `alphanum`     | `validate:"alphanum"`     | Letters and digits only               |
| `min=X`        | `validate:"min=3"`        | Min length (string) or value (number) |
| `max=X`        | `validate:"max=100"`      | Max length or value                   |
| `gte=X`        | `validate:"gte=18"`       | Greater than or equal                 |
| `lte=X`        | `validate:"lte=150"`      | Less than or equal                    |

For full list of validators, see [go-playground/validator docs](https://pkg.go.dev/github.com/go-playground/validator/v10).

## Middleware

### Built-in Middleware

#### Logger

Logs all requests with method, path, status, latency, and size:

```go
r.Use(middleware.Logger)

// Output:
// GET /users 200 1.234ms | ip=192.168.1.100 size=0 resp=250 bytes
```

#### Recover

Catches panics and returns 500 error (must be registered first):

```go
r.Use(middleware.Recover)  // Must be first
r.Use(middleware.Logger)
```

#### CORS

Handles cross-origin requests:

```go
corsConfig := middleware.DefaultCORSConfig()
corsConfig.AllowedOrigins = []string{"https://example.com"}
corsConfig.AllowCredentials = true
r.Use(middleware.CORS(corsConfig))
```

### Custom Middleware

Middleware implements `zen.MiddlewareFunc`:

```go
func AuthMiddleware(c *zen.Context, next http.Handler) {
    token := c.Request.Header.Get("Authorization")
    if token == "" {
        c.SendError(zen.Unauthorized())
        return  // short-circuit
    }
    // Validate token...
    c.Set("user_id", 42)  // Store in context
    next.ServeHTTP(c.Response, c.Request)
}

r.Use(AuthMiddleware)
```

## Error Handling

### Structured Errors

Return structured JSON error responses:

```go
// Custom error
err := zen.NewHTTPError(http.StatusBadRequest, "invalid input")
err = err.WithDetails(map[string]string{"field": "email"})
err = err.WithRequestID("req-12345")
c.SendError(err)

// Pre-built errors
c.SendError(zen.BadRequest("email invalid"))
c.SendError(zen.Unauthorized())
c.SendError(zen.NotFound("user"))
c.SendError(zen.InternalError("database error"))
```

### Form Binding Errors

Detailed form parsing errors:

````go
if err := c.BindForm(&data); err != nil {
    var fe *zen.FormError
    if errors.As(err, &fe) {
        // Structured form error
        c.JSON(http.StatusBadRequest, map[string]string{
            "field": fe.Field,
            "error": fe.Err.Error(),
        })
        return
    }
    // Other error (parsing, validation, etc)
    c.SendError(zen.BadRequest(err.Error()))
}

### Form Binding

Parse URL-encoded or multipart form data:

```go
type ContactForm struct {
    Name  string `json:"name"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"`
}

r.Handle("POST /contact", func(c *zen.Context) {
    var form ContactForm
    if err := c.BindForm(&form); err != nil {
        c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    // Process form data
})
````

### Query Parameters

Retrieve query string parameters with caching:

```go
r.Handle("GET /search", func(c *zen.Context) {
    query := c.QueryParam("q")        // ?q=keyword
    page := c.QueryParam("page")      // ?page=2
    limit := c.QueryParam("limit")    // ?limit=10
})
```

### Path Parameters

Access URL path parameters (requires Go 1.22+):

```go
r.Handle("GET /users/{id}", func(c *zen.Context) {
    id := c.Param("id")  // Matches {id} in the route pattern
})
```

### Raw Body

Read the complete request body as bytes:

```go
r.Handle("POST /webhook", func(c *zen.Context) {
    body, err := c.Body()
    if err != nil {
        // Handle error
    }
    // Process raw body
})
```

**Note:** Body size is limited to 1MB by default to prevent DoS attacks.

## Response Helpers

### JSON Response

```go
r.Handle("GET /api/user", func(c *zen.Context) {
    user := map[string]any{
        "id":    123,
        "name":  "Alice",
        "email": "alice@example.com",
    }
    c.JSON(http.StatusOK, user)
})
```

### String Response

```go
r.Handle("GET /robots.txt", func(c *zen.Context) {
    c.JSON(http.StatusOK, "User-agent: *\nDisallow: /")
})
```

### Error Response

```go
r.Handle("GET /not-found", func(c *zen.Context) {
    c.JSON(http.StatusNotFound, map[string]string{"error": "resource not found"})
})
```

## Middleware

### Built-in Middleware

**Recover** - Catches panics and returns 500:

```go
r.Use(middleware.Recover)
```

### Custom Middleware

Write middleware with the `MiddlewareFunc` signature:

```go
type MiddlewareFunc func(*zen.Context, http.Handler)

func Logger(c *zen.Context, next http.Handler) {
    start := time.Now()
    log.Printf("%s %s", c.Request.Method, c.Request.URL.Path)
    next.ServeHTTP(c.Response, c.Request)
    log.Printf("took %v", time.Since(start))
}

r.Use(Logger)
```

### Example: Authentication Middleware

```go
func Auth(c *zen.Context, next http.Handler) {
    token := c.Request.Header.Get("Authorization")
    if !strings.HasPrefix(token, "Bearer ") {
        c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
        return
    }

    // Validate token (implement your logic here)
    userID := validateToken(strings.TrimPrefix(token, "Bearer "))
    if userID == "" {
        c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
        return
    }

    // Store user ID for handlers to access
    c.Set("userID", userID)
    next.ServeHTTP(c.Response, c.Request)
}

r.Use(Auth)

r.Handle("GET /profile", func(c *zen.Context) {
    userID := c.Get("userID")  // Retrieve from context
    // ...
})
```

## Context Storage

Store and retrieve arbitrary values during request processing:

```go
// In middleware
c.Set("userID", userID)
c.Set("session", sessionData)

// In handler
userID := c.Get("userID")
session := c.Get("session")
```

The first 8 unique keys are stored inline (no allocation). A map is allocated lazily for additional keys.

## Raw Handlers

For third-party handlers or pre-marshalled responses:

```go
// Pre-compute response
var largeResponse []byte  // computed once at startup

r.HandleRaw("GET /api/data", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write(largeResponse)
}))
```

## HTTPS Support

```go
r := zen.New(":8443")
r.ListenAndServeTLS("cert.pem", "key.pem")
```

## Graceful Shutdown

`ListenAndServe` and `ListenAndServeTLS` automatically handle graceful shutdown:

1. Wait for SIGINT or SIGTERM
2. Allow 5 seconds for in-flight requests to complete
3. Force close after timeout

For programmatic control, use the underlying `http.Server` directly:

```go
r := zen.New(":8080")

go func() {
    if err := r.Server.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}()

// Your shutdown logic here
r.Server.Shutdown(context.Background())
```

## Error Handling

### Structured Error Responses

Zen provides a standard error response format with `HTTPError` and helper functions:

```go
if user == nil {
    c.SendError(zen.NotFound("user"))
    return
}

if !authorized {
    c.SendError(zen.Unauthorized())
    return
}

if err := validateData(&data); err != nil {
    err := zen.BadRequest(err.Error())
    err = err.WithDetails(map[string]string{"field": "email"})
    c.SendError(err)
    return
}

// Response sent as JSON:
// {
//   "status": 400,
//   "message": "invalid email format",
//   "details": {"field": "email"},
//   "request_id": "abc123"
// }
```

### Available Error Helpers

| Helper                 | Status | Usage                                     |
| ---------------------- | ------ | ----------------------------------------- |
| `BadRequest(msg)`      | 400    | Invalid input, validation errors          |
| `Unauthorized()`       | 401    | Missing or invalid authentication         |
| `Forbidden()`          | 403    | Authenticated but not authorized          |
| `NotFound(resource)`   | 404    | Resource not found                        |
| `Conflict(msg)`        | 409    | Duplicate or conflicting resource         |
| `InternalError(msg)`   | 500    | Server error (details hidden in response) |
| `ServiceUnavailable()` | 503    | Service temporarily unavailable           |

### Custom Errors

```go
err := zen.NewHTTPError(http.StatusGone, "resource deleted")
err = err.WithDetails(map[string]any{"deleted_at": "2024-01-01"})
err = err.WithRequestID(requestID)
c.SendError(err)
```

### Form Binding Errors

```go
var form MyForm
if err := c.BindForm(&form); err != nil {
    var fe *zen.FormError
    if errors.As(err, &fe) {
        c.SendError(zen.BadRequest(
            fmt.Sprintf("Field '%s': %s", fe.Field, fe.Err.Error()),
        ))
        return
    }

    c.SendError(zen.BadRequest(err.Error()))
    return
}
```

### JSON Binding Errors

```go
var req MyRequest
if err := c.BindJSON(&req); err != nil {
    c.SendError(zen.BadRequest(err.Error()))
    return
}
```

## Structured Error Handling

See [Error Handling](#error-handling) section above for comprehensive error handling with HTTPError and helper functions.

## Middleware

### Built-in Middleware

#### Logger

Logs all requests with method, path, status, latency, and size:

```go
r.Use(middleware.Logger)

// Output:
// GET /users 200 1.234ms | ip=192.168.1.100 size=0 resp=250 bytes
```

#### Recover

Catches panics and returns 500 error (must be registered first):

```go
r.Use(middleware.Recover)  // Must be first
r.Use(middleware.Logger)
```

#### CORS

Handles cross-origin requests:

```go
corsConfig := middleware.DefaultCORSConfig()
corsConfig.AllowedOrigins = []string{"https://example.com"}
corsConfig.AllowCredentials = true
r.Use(middleware.CORS(corsConfig))
```

### Custom Middleware

Middleware implements `zen.MiddlewareFunc`:

```go
func AuthMiddleware(c *zen.Context, next http.Handler) {
    token := c.Request.Header.Get("Authorization")
    if token == "" {
        c.SendError(zen.Unauthorized())
        return  // short-circuit
    }
    // Validate token...
    c.Set("user_id", 42)  // Store in context
    next.ServeHTTP(c.Response, c.Request)
}

r.Use(AuthMiddleware)
```

## Error Handling

### Bind Errors

```go
r.Handle("POST /data", func(c *zen.Context) {
    var req MyRequest
    if err := c.BindJSON(&req); err != nil {
        // JSON decode error or validation error
        c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
})
```

### Form Errors

```go
r.Handle("POST /contact", func(c *zen.Context) {
    var form ContactForm
    if err := c.BindForm(&form); err != nil {
        if _, ok := err.(*zen.FormError); ok {
            // Field conversion error
            c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
            return
        }
        // Parse error
        c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
})
```

## Performance

Zen is designed for high throughput and low latency with minimal GC overhead.

### Quick Stats

- **Simple requests:** 47k-48k RPS (parity with Gin/Echo)
- **Complex requests:** 47k RPS (with JSON binding + validation)
- **High concurrency:** 48.8k RPS ⭐ (beats Gin by 0.3%)
- **Large payloads:** 28k RPS ⭐⭐ (45% faster than Gin!)
- **Mean latency:** 2-5ms (depending on payload)
- **Consistency:** Best P95/P99 tail latencies

### Comprehensive Benchmarks

See [PERFORMANCE.md](PERFORMANCE.md) for detailed benchmark analysis including:

- Scenario-by-scenario results (Simple GET, Complex POST, High Concurrency, Large Payload)
- Performance driver explanations
- Comparative analysis with Gin, Echo, and stdlib
- Production readiness assessment

### Benchmarks (Real TCP, not httptest)

Load tests against Go stdlib, Gin, and Echo under various scenarios:

**Simple GET Requests** — `GET /products`
| Framework | RPS | Mean Latency | P95 | Status |
|-----------|-----|--------------|-----|--------|
| Echo | 47.8k | 2.05ms | 3.4ms | ✓ |
| Zen | 47.7k | 2.06ms | 3.5ms | ✓ |
| Gin | 47.3k | 2.07ms | 3.5ms | ✓ |

**Complex POST with Validation** — `POST /orders` with JSON binding
| Framework | RPS | Mean Latency | P95 | Status |
|-----------|-----|--------------|-----|--------|
| Gin | 47.9k | 2.04ms | 3.4ms | ✓ |
| Echo | 47.3k | 2.07ms | 3.5ms | ✓ |
| Zen | 47.3k | 2.07ms | 3.7ms | ✓ |

**High Concurrency Burst** — `GET /users/{id}` under high load
| Framework | RPS | Mean Latency | P95 | Status |
|-----------|-----|--------------|-----|--------|
| **Zen** | **48.8k** | **2.01ms** | **3.1ms** | ✓ |
| Gin | 48.6k | 2.02ms | 3.2ms | ✓ |
| Echo | 48.3k | 2.03ms | 3.3ms | ✓ |

**Large Payload (1MB JSON)** — `GET /data`
| Framework | RPS | Mean Latency | P95 | Status |
|-----------|-----|--------------|-----|--------|
| **Zen** | **28.2k** | **3.47ms** | **4.5ms** | ✓ |
| Echo | 24.1k | 4.08ms | 8.0ms | ✓ |
| Gin | 19.5k | 5.07ms | 14.2ms | ✓ |

### Key Findings

✅ **Performance parity with Gin/Echo** in typical use cases
✅ **Best-in-class large payload handling** (45% faster than Gin)
✅ **Superior high-concurrency performance** (+0.3% throughput vs Gin)
✅ **Consistent latency distribution** (minimal P95/P99 jitter)
✅ **Zero errors under sustained load**

### Performance Characteristics

- **Memory Efficiency**
  - Context pooling: Reuses objects across requests
  - Inline storage: First 8 context keys allocated inline
  - Form caching: Metadata computed once per struct type
  - Query caching: Parsed once per request
  - Buffer pooling: JSON response buffers reused

- **CPU Efficiency**
  - Zero allocations in hot path
  - Middleware pre-built at setup time
  - Type-specialized form setters
  - Lazy validation initialization

### Run Your Own Benchmarks

```bash
cd loadtest
./run_loadtest.sh

# Custom options:
./run_loadtest.sh -duration 30s -conns 250
./run_loadtest.sh -servers zen,echo,gin,std -duration 20s
```

**Note:** Tests use real TCP connections, not `httptest.ResponseRecorder` for realistic results

## Project Structure

```
zen/
├── zen.go              # Package documentation and overview
├── context.go          # Context pooling with zero-allocation storage
├── router.go           # Router, server, and middleware chaining
├── group.go            # Route groups with prefix and middleware inheritance
├── request.go          # JSON/form/file binding with validation
├── response.go         # JSON response with buffer pooling
├── http_error.go       # Structured HTTP error responses
├── validate.go         # Lazy validator initialization
├── errors.go           # Sentinel error types (ErrInvalidBindTarget, FormError)
├── form.go             # Form binding with reflection caching
├── middleware/         # Built-in middleware
│   ├── logger.go       # Request logging with metrics
│   ├── recover.go      # Panic recovery
│   └── cors.go         # CORS handling
├── system/
│   └── banner.go       # Startup banner
├── cmd/                # Example servers
│   └── zenserver/main.go
└── examples/           # Example applications
    ├── simple/main.go
    └── sse_ws/main.go
```

## Performance Characteristics

Zen is optimized for production workloads with minimal GC overhead:

### Memory Efficiency

- **Context pooling:** Reuses Context allocations across requests (sync.Pool)
- **Inline storage:** First 8 context keys stored inline (no allocation)
- **Form caching:** Reflection metadata cached per struct type (sync.Map)
- **Query caching:** URL query parameters parsed once per request
- **Buffer pooling:** JSON response buffers reused via sync.Pool

### CPU Efficiency

- **Zero-allocation hot path:** No closures allocated per request in middleware
- **Middleware pre-built:** Chain construction happens at setup time, not per-request
- **Type-specialized form setters:** Type switch happens once per field type, not per field per request
- **Lazy validation:** Validator initialized only on first use

### Benchmarks

Run the included load test to compare zen with stdlib, Echo, and Gin:

```bash
cd loadtest
./run_loadtest.sh

# Custom options:
./run_loadtest.sh -duration 30s -conns 250
./run_loadtest.sh -servers zen,echo,std -duration 20s
```

**Important:** The load test uses real TCP connections, not `httptest.ResponseRecorder`. This gives accurate real-world numbers.

## Comparison with Other Frameworks

| Feature                 | Zen                   | Gin        | Echo       | Stdlib      |
| ----------------------- | --------------------- | ---------- | ---------- | ----------- |
| **Dependencies**        | validator/v10 only    | Multiple   | Multiple   | None        |
| **Routing**             | Go 1.22+ mux          | Custom     | Custom     | Basic mux   |
| **Performance**         | 47-48k RPS            | 47-48k RPS | 47-48k RPS | 46k RPS     |
| **Large payloads**      | 28k RPS ⭐            | 19k RPS    | 24k RPS    | N/A         |
| **Context pooling**     | Yes ✓                 | Yes        | Yes        | Manual      |
| **Built-in middleware** | Logger, Recover, CORS | None       | None       | None        |
| **Error handling**      | Structured            | Manual     | Manual     | Manual      |
| **File uploads**        | ✓ Single & multiple   | ✓          | ✓          | Manual      |
| **Form validation**     | ✓ Built-in            | Manual     | Manual     | Manual      |
| **Route groups**        | ✓ With inheritance    | ✓          | ✓          | No          |
| **Documentation**       | Comprehensive         | Good       | Good       | Stdlib docs |
| **Maintenance**         | Lightweight           | Active     | Active     | Go team     |
| **Learning curve**      | Minimal               | Low        | Low        | Minimal     |

**Summary:** Zen offers production-ready features with minimal dependencies while matching or exceeding performance of heavier frameworks.

## Testing

Zen works seamlessly with Go's `httptest` package for unit tests:

```go
func TestGetUser(t *testing.T) {
    r := zen.New(":8080")
    r.Handle("GET /users/{id}", func(c *zen.Context) {
        id := c.Param("id")
        c.JSON(http.StatusOK, map[string]string{"id": id})
    })

    req := httptest.NewRequest("GET", "/users/42", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", w.Code)
    }
}
```

## License

MIT
