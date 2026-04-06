# Zen

A small Go web framework built on the standard library `net/http`.
Zen provides routing, request binding, middleware, and simple helpers with a minimal API.

## Why Zen?

- Uses Go 1.22+ `http.ServeMux` with pattern matching
- Minimal surface area, no external router dependency
- Built-in JSON, form, and file binding
- Simple middleware chaining and graceful shutdown
- Optional SSE/WebSocket support via separate `sse` and `ws` packages
- Optional `util` package for env loading and DB helpers
- Optional `auth` package for authentication across HTTP, WebSocket, and SSE

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
- Optional `util` package for env loading and DB helpers

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
- Response methods: `c.JSON(status, data)`, `c.SendError(err)`
- Key-value storage: `c.Set(key, val)`, `c.Get(key)`

Contexts are pooled and reused across requests for zero allocation overhead.

## Utilities

Zen includes an optional `config` package for common app setup helpers:

- `config.LoadEnvFile(path, override)` to load `.env` files
- `config.GetEnv`, `config.MustGetEnv`, `config.GetEnvInt`, `config.GetEnvBool`, `config.GetEnvDuration`
- `config.OpenDB`, `config.WithTransaction`, `config.CloseDB`

### Environment helpers

```go
import (
    "github.com/Pavan-Silva/go-zen/config"
)

if err := config.LoadEnvFile(".env", false); err != nil {
    panic(err)
}

port := config.GetEnv("PORT", "8080")
maxActive, _ := config.GetEnvInt("DB_MAX_ACTIVE", 25)
```

### Database helpers

```go
import (
    "context"
    "database/sql"

    "github.com/Pavan-Silva/go-zen/config"
)

cfg := config.DefaultDBConfig()
ctx := context.Background()
db, err := config.OpenDB(ctx, "postgres", "postgres://user:pass@localhost/db?sslmode=disable", cfg)
if err != nil {
    panic(err)
}
defer config.CloseDB(db)

err = config.WithTransaction(ctx, db, func(tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx, "INSERT INTO users(name) VALUES($1)", "alice")
    return err
})
```

## Optional Streaming Packages

SSE and WebSocket support are available as optional packages rather than being included in the core `zen` package.

### Server-Sent Events (SSE)

```go
import (
    "time"

    "github.com/Pavan-Silva/go-zen"
    "github.com/Pavan-Silva/go-zen/sse"
)

r := zen.New(":8080")

sse.Handle(r, "GET /events", func(c *zen.Context) error {
    for i := 1; i <= 5; i++ {
        if err := sse.Send(c, "message", map[string]any{
            "count": i,
            "time":  time.Now().Format(time.RFC3339),
        }); err != nil {
            return err
        }
        time.Sleep(1 * time.Second)
    }
    return nil
})
```

### WebSockets

```go
import (
    "github.com/Pavan-Silva/go-zen"
    "github.com/Pavan-Silva/go-zen/ws"
)

r := zen.New(":8080")

ws.Handle(r, "GET /ws", func(c *zen.Context, conn *ws.Conn) {
    defer conn.Close()

    for {
        var msg map[string]any
        if err := conn.ReadJSON(&msg); err != nil {
            return
        }
        _ = conn.WriteJSON(map[string]any{"echo": msg})
    }
})
```

## Authentication

Zen provides an optional `auth` package for consistent authentication across HTTP, WebSocket, and SSE routes.

### Authenticator Interface

Implement the `Authenticator` interface to define custom authentication logic:

```go
type Authenticator interface {
    Authenticate(r *http.Request) (User, error)
}
```

### Built-in Authenticators

- `BasicAuth` - HTTP Basic Authentication
- `JWTAuth` - JWT token authentication
- `APIKeyAuth` - API key via header or query param
- `SessionAuth` - Session-based authentication

### HTTP Authentication

```go
import (
    "github.com/Pavan-Silva/go-zen"
    "github.com/Pavan-Silva/go-zen/auth"
)

authenticator := &auth.BasicAuth{
    Realm: "My App",
    Validate: func(username, password string) (auth.User, error) {
        // Your validation logic
        return auth.User{ID: "1", Username: username}, nil
    },
}

r := zen.New(":8080")
// Skip auth for public health and metrics endpoints
r.Use(auth.RequireAuthWithSkipper(authenticator, auth.SkipPaths("/health", "/metrics")))

r.Handle("GET /protected", func(c *zen.Context) {
    user := auth.GetUser(c)
    c.JSON(200, map[string]string{"user": user.Username})
})
```

### WebSocket Authentication

```go
ws.HandleWithAuth(r, "GET /ws", authenticator, func(c *zen.Context, conn *ws.Conn) {
    user := auth.GetUser(c)
    // Authenticated WebSocket handler
})
```

### SSE Authentication

```go
sse.HandleWithAuth(r, "GET /events", authenticator, func(c *zen.Context) error {
    user := auth.GetUser(c)
    // Authenticated SSE handler
    return sse.Send(c, "message", map[string]any{"user": user.Username})
})
```

### Skipping Auth for Selected Routes

```go
r := zen.New(":8080")
r.Use(auth.RequireAuth(authenticator, auth.SkipPaths("/health", "/metrics")))

// /health and /metrics are public.
// All other routes require auth.
```

### Role-Based Access

```go
r.Use(auth.RequireRole("admin", nil))

r.Handle("GET /admin", func(c *zen.Context) {
    // Only accessible to users with "admin" role
})
```

## Route Groups

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

## Optional Streaming Packages

SSE and WebSocket support are available as optional packages rather than bundled into the core `zen` package.

### Server-Sent Events (SSE)

```go
import (
    "time"

    "github.com/Pavan-Silva/go-zen"
    "github.com/Pavan-Silva/go-zen/sse"
)

r := zen.New(":8080")

sse.Handle(r, "GET /events", func(c *zen.Context) error {
    for i := 1; i <= 5; i++ {
        if err := sse.Send(c, "message", map[string]any{
            "count": i,
            "time":  time.Now().Format(time.RFC3339),
        }); err != nil {
            return err
        }
        time.Sleep(1 * time.Second)
    }
    return nil
})
```

### WebSockets

```go
import (
    "github.com/Pavan-Silva/go-zen"
    "github.com/Pavan-Silva/go-zen/ws"
)

r := zen.New(":8080")

ws.Handle(r, "GET /ws", func(c *zen.Context, conn *ws.Conn) {
    defer conn.Close()

    for {
        var msg map[string]any
        if err := conn.ReadJSON(&msg); err != nil {
            return
        }

        _ = conn.WriteJSON(map[string]any{"echo": msg})
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
├── auth/               # Optional authentication helpers
│   └── auth.go          # Authenticators for HTTP, WS, SSE
├── config/             # Optional app configuration helpers
│   ├── db.go           # Database connection helpers and transactions
│   └── env.go          # Environment loading and typed env helpers
├── middleware/         # Built-in middleware
│   ├── logger.go       # Request logging with metrics
│   ├── recover.go      # Panic recovery
│   └── cors.go         # CORS handling
├── sse/                # Optional SSE helpers
│   └── sse.go
├── ws/                 # Optional WebSocket helpers
│   └── ws.go
├── system/
│   └── banner.go       # Startup banner
├── cmd/                # Example servers
│   └── zenserver/main.go
└── examples/           # Example applications
    ├── simple/main.go
    └── sse_ws/main.go
```

## License

MIT
