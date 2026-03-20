# Zen

A minimalist Go web framework built on top of the standard library's `net/http`. Zen provides a small set of helpers to reduce boilerplate while maintaining excellent performance.

## Features

- **Zero dependencies** beyond the standard library (plus optional validator)
- **High performance** - benchmarks competitive with popular frameworks
- **Familiar API** - if you've used Gin or Echo, you'll feel right at home
- **Middleware support** - easy to add custom middleware
- **Request binding** - JSON and form parsing with validation
- **Graceful shutdown** - handles SIGINT/SIGTERM automatically
- **Memory efficient** - context pooling and form field caching

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
    r.Use(middleware.Recover)

    r.Handle("GET /hello", func(c *zen.Context) {
        c.JSON(http.StatusOK, map[string]string{"message": "Hello, World!"})
    })

    r.ListenAndServe()
}
```

## Core Types

### Router

`zen.New(addr)` creates a new Router with security-hardened defaults:

- Read timeout: 5 seconds
- Write timeout: 10 seconds
- Idle timeout: 120 seconds
- Read header timeout: 2 seconds
- Max header bytes: 1MB

```go
r := zen.New(":8080")
```

### Context

The `Context` is passed to every handler and provides access to request data and response utilities.

## Routing

Zen uses Go 1.22+ enhanced `http.ServeMux` routing with path parameters:

```go
// Static routes
r.Handle("GET /home", handleHome)
r.Handle("POST /users", handleCreateUser)

// Routes with path parameters
r.Handle("GET /users/{id}", handleGetUser)
r.Handle("GET /posts/{slug}/comments/{comment_id}", handleGetComment)

// Pattern matching
r.Handle("GET /files/*path", handleFiles)
```

## Request Handling

### JSON Binding

Parse and validate JSON request bodies:

```go
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,alphanum"`
    Email    string `json:"email"    validate:"required,email"`
    Age      int    `json:"age"      validate:"gte=18"`
}

r.Handle("POST /users", func(c *zen.Context) {
    var req CreateUserRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    // req.Username, req.Email, req.Age are now populated and validated
    c.JSON(http.StatusCreated, map[string]string{"status": "created"})
})
```

Validation uses [go-playground/validator](https://github.com/go-playground/validator). Available tags include:

| Tag | Description |
|-----|-------------|
| `required` | Field must not be empty |
| `email` | Valid email format (uses regex) |
| `email-strict` | Valid email format (uses RFC 5322 parser) |
| `alphanum` | Only letters and digits |
| `min=X` | Minimum length/value |
| `max=X` | Maximum length/value |
| `gte=X` | Greater than or equal |
| `lte=X` | Less than or equal |

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
```

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

| Framework | RPS | Mean Latency |
|-----------|-----|--------------|
| zen | ~70,000 | ~1.3ms |
| gin | ~70,000 | ~1.4ms |
| echo | ~72,000 | ~1.3ms |

Run your own benchmarks:

```bash
cd loadtest
./run_loadtest.sh
```

## Project Structure

```
zen/
├── context.go      # Request context with storage
├── router.go       # Router and server
├── request.go      # JSON/form binding
├── response.go     # JSON response helper
├── validate.go     # Struct validation
├── errors.go       # Error types
├── form.go         # Form binding
├── middleware/
│   └── recover.go  # Panic recovery
└── system/
    └── banner.go   # Startup banner
```

## License

MIT
