# Zen

A lightweight, high-performance microframework built on Go's standard library `net/http`. Zen prioritizes simplicity and performance, providing essential helpers while maintaining compatible throughput with Gin and Echo.

## Why Zen?

- **Stdlib-only** — No router dependency; uses Go 1.22+ `http.ServeMux` with pattern matching
- **Performance-optimized** — Context pooling, buffer pooling, reflection caching, zero-allocation middleware chains
- **Production-ready** — Security hardened defaults, graceful shutdown, comprehensive error handling
- **Well-documented** — Every public API includes examples and performance notes
- **Lightweight overhead** — Typically 1–2ms slower per request than Gin/Echo, but used with stdlib benefits (no runtime, security updates in Go releases)

## Features

### Core

- ✅ Request/response binding (JSON, forms, files)
- ✅ Struct validation with custom validators
- ✅ Route groups with middleware inheritance
- ✅ Built-in middleware (Logger, Recover, CORS)
- ✅ Graceful shutdown with signal handling
- ✅ Structured error responses
- ✅ Context pooling for zero-GC per-request
- ✅ Security hardened defaults (timeouts, headers)

### Performance Optimizations

- ✅ Pre-built middleware chains (no per-request construction)
- ✅ Form field reflection cached per struct type
- ✅ Lazy validator initialization
- ✅ JSON buffer pooling
- ✅ Inline context storage (8 key-value pairs without allocation)

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
- Response methods: `c.JSON(status, data)`, `c.SendError(err)`
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
        c.SendError(zen.CommonErrors.BadRequest(err.Error()))
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
        c.SendError(zen.CommonErrors.BadRequest(err.Error()))
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
        c.SendError(zen.CommonErrors.BadRequest(err.Error()))
        return
    }
    // file.Filename, file.Size, len(content)
    c.JSON(http.StatusOK, map[string]string{"size": fmt.Sprintf("%d bytes", len(content))})
})

r.Handle("POST /attachments", func(c *zen.Context) {
    // Multiple files
    files, err := c.BindFiles("attachments")
    if err != nil {
        c.SendError(zen.CommonErrors.BadRequest(err.Error()))
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
        c.SendError(zen.CommonErrors.Unauthorized())
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
c.SendError(zen.CommonErrors.BadRequest("email invalid"))
c.SendError(zen.CommonErrors.Unauthorized())
c.SendError(zen.CommonErrors.NotFound("user"))
c.SendError(zen.CommonErrors.InternalError("database error"))
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
    c.SendError(zen.CommonErrors.BadRequest(err.Error()))
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

Zen is designed for high throughput and low latency:

- **Context pooling** - reuses Context objects across requests
- **No allocations** in the hot path (inline storage for common operations)
- **Form field caching** - reflection metadata is computed once per struct type
- **Query parameter caching** - parsed once per request
- **Direct middleware chain** - bypasses context creation when no middleware registered

Benchmarks (100 concurrent connections, 10 second duration):

| Framework | RPS     | Mean Latency |
| --------- | ------- | ------------ |
| zen       | ~70,000 | ~1.3ms       |
| gin       | ~70,000 | ~1.4ms       |
| echo      | ~72,000 | ~1.3ms       |

Run your own benchmarks:

```bash
cd loadtest
./run_loadtest.sh
```

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
    └── simple/main.go
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

| Feature             | Zen                  | Gin        | Echo       | Notes                                     |
| ------------------- | -------------------- | ---------- | ---------- | ----------------------------------------- |
| **Dependencies**    | `validator/v10` only | High       | Medium     | Zen uses only stdlib + optional validator |
| **Routing**         | Go 1.22+ ServeMux    | Custom     | Custom     | Zen leverages standard library            |
| **Performance**     | ~70k req/s           | ~70k req/s | ~72k req/s | Real TCP, not httptest                    |
| **Context pooling** | Yes                  | Yes        | Yes        | All modern frameworks do this             |
| **Learning curve**  | Minimal              | Low        | Low        | Stdlib knowledge is main requirement      |
| **Maintenance**     | Lightweight          | Heavy      | Heavy      | Zen updates with Go releases              |

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
