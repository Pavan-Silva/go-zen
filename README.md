# Zen

A lightweight Go web framework built on the standard library `net/http`.
Zen gives you modern request routing, binding, middleware, and reusable helpers with a small API surface and no external router dependency.

## Why Zen?

- Minimal, stdlib-first framework for Go 1.22+
- Pattern-based routing via `http.ServeMux` and Go's route syntax
- Fast request lifecycle with pooled contexts and zero-allocation middleware chaining
- Secure defaults for graceful shutdown
- Built-in request binding for JSON, XML, forms, multipart files, and headers
- Optional helper packages for auth, env, SSE, and observability

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
    app := zen.New(":8080")
    app.Use(middleware.Logger, middleware.Recover)
    app.GET("/", func(c *zen.Ctx) {
        c.String(http.StatusOK, "Hello World")
    })
    app.Run()
}
```

## Configuration

Customize the server by passing a config to `zen.New()`:

```go
config := zen.Config{
    ReadTimeout:       8 * time.Second,
    WriteTimeout:      12 * time.Second,
    IdleTimeout:       90 * time.Second,
    ReadHeaderTimeout: 2 * time.Second,
    MaxHeaderBytes:    1 << 20,
    ShutdownTimeout:   10 * time.Second,
}

app := zen.New(":8080", config)
```

All timeout and header fields default to `0` (stdlib defaults), matching raw `net/http` performance.

## Routing

Handlers receive a `*zen.Ctx` and use request/response helpers directly.

```go
app.GET("/users/{id}", func(c *zen.Ctx) {
    page := c.QueryParam("page")
    userID := c.Param("id")
    c.JSON(http.StatusOK, map[string]string{"id": userID, "page": page})
})

app.POST("/users", func(c *zen.Ctx) {
    var payload struct {
        Name  string `json:"name" validate:"required"`
        Email string `json:"email" validate:"required,email"`
    }
    if err := c.BindJSON(&payload); err != nil {
        c.Error(http.StatusBadRequest, err.Error())
        return
    }
    c.JSON(http.StatusCreated, payload)
})

app.PUT("/users/{id}", func(c *zen.Ctx) { /* ... */ })
app.DELETE("/users/{id}", func(c *zen.Ctx) { c.NoContent(http.StatusNoContent) })
app.PATCH("/users/{id}", func(c *zen.Ctx) { /* ... */ })
app.HEAD("/health", func(c *zen.Ctx) { c.NoContent(http.StatusOK) })
app.OPTIONS("/api", func(c *zen.Ctx) { c.NoContent(http.StatusNoContent) })
```

### Route Groups

```go
api := app.Group("/api", authMiddleware)
api.GET("/users", listUsers)
api.POST("/users", createUser)

admin := api.Group("/admin", adminMiddleware)
admin.GET("/stats", adminStats)
```

Group middleware is inherited by subgroups and pre-chained at startup.

### Raw Handlers

Register a raw `http.Handler` directly on the underlying `ServeMux`:

```go
app.HandleRaw("GET /health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    _, _ = w.Write([]byte("ok"))
}))
```

## Middleware

Global middleware is registered with `r.Use(...)`.

```go
app.Use(middleware.Recover, middleware.Logger, middleware.CORS(corsConfig))
app.Use(middleware.BodyLimit("2M"))
```

Middleware is a `func(*zen.Ctx)`. Call `c.Next()` to pass control downstream. Return without calling it to short-circuit.

```go
app.Use(func(c *zen.Ctx) {
    if c.Request.Header.Get("Authorization") == "" {
        c.Error(http.StatusUnauthorized, "missing token")
        return
    }
    c.Next()
})
```

### Per-Route Middleware

Pass middleware before the final handler:

```go
app.GET("/admin", authMiddleware, auditLog, func(c *zen.Ctx) {
    c.String(http.StatusOK, "admin")
})
```

### Built-in Middleware

- `middleware.Logger` — request logging with method, path, status, latency, client IP, and response size
- `middleware.Recover` — recovers from panics and returns a 500 response
- `middleware.CORS` — flexible CORS support for origins, methods, headers, and credentials
- `middleware.BodyLimit` — limits request body size with support for skippers
- `middleware.Compress()` — gzip compression for clients that accept it
- `middleware.CompressWithLevel(level)` — gzip with custom compression level
- `middleware.RateLimiter()` — in-memory rate limiting with token bucket
- `middleware.RateLimiterWithConfig(config)` — custom rate limit config
- `middleware.RegisterPprof(r)` — mounts Go pprof endpoints on the router

```go
middleware.RegisterPprof(app)
```

## Static Files

```go
app.File("/", "public/index.html")
app.Static("/images", "assets/images")
app.StaticFS("/images", embeddedFS)
```

## Request Binding

Zen supports binding for JSON, XML, form data, multipart, headers, and path/query params.

```go
app.POST("/submit", func(c *zen.Ctx) {
    var form struct {
        Name  string `form:"name" validate:"required"`
        Email string `form:"email" validate:"required,email"`
    }
    if err := c.BindForm(&form); err != nil {
        c.Error(http.StatusBadRequest, err.Error())
        return
    }
    c.JSON(http.StatusOK, form)
})
```

### Header Binding

```go
type Headers struct {
    UserID string `header:"X-User-Id"`
    Rate   int    `header:"X-Rate-Limit"`
}

app.GET("/api", func(c *zen.Ctx) {
    var h Headers
    if err := c.BindHeader(&h); err != nil {
        c.Error(http.StatusBadRequest, err.Error())
        return
    }
    c.JSON(http.StatusOK, h)
})
```

## Responses

```go
c.JSON(http.StatusOK, data)
c.XML(http.StatusOK, data)
c.HTML(http.StatusOK, "<h1>Hello</h1>")
c.String(http.StatusOK, "plain text")
c.NoContent(http.StatusNoContent)
c.Redirect(http.StatusMovedPermanently, "/new-path")
c.Blob(http.StatusOK, "text/csv", data)
c.Stream(http.StatusOK, "image/png", reader)
c.Error(http.StatusBadRequest, "invalid input")

// File responses
c.File("/tmp/report.pdf")
c.Attachment("/tmp/report.pdf", "monthly-report.pdf")
c.Inline("/tmp/image.png")
```

### SSE

```go
app.GET("/events", func(c *zen.Ctx) {
    if err := c.SSEvent("message", map[string]string{"hello": "world"}); err != nil {
        c.Error(http.StatusInternalServerError, err.Error())
    }
})
```

## Authentication

The optional `auth` package supports multiple authenticator types: basic auth, JWT, OAuth2, OIDC, and sessions.

```go
jwtAuth := &auth.JWTAuth{Secret: []byte("your-secret-key")}
app.Use(auth.RequireAuth(jwtAuth))

app.GET("/profile", func(c *zen.Ctx) {
    user := auth.GetUser(c)
    c.JSON(http.StatusOK, user)
})
```

## License

Zen is released under the MIT License.
