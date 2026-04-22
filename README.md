# Zen

A lightweight Go web framework built on the standard library `net/http`.
Zen gives you modern request routing, binding, middleware, and reusable helpers with a small API surface and no external router dependency.

## Why Zen?

- Minimal, stdlib-first framework for Go 1.22+
- Pattern-based routing via `http.ServeMux` and Go's route syntax
- Fast request lifecycle with pooled contexts and zero-allocation middleware chaining
- Secure defaults for timeouts, header limits, and graceful shutdown
- Built-in request binding for JSON, forms, multipart files, and raw bodies
- Optional helper packages for auth, Env, SSE, and WebSocket

## Installation

```bash
go get github.com/Pavan-Silva/go-zen
```

## Quick Start

````go
package main

import (
    "net/http"
    "time"

    "github.com/Pavan-Silva/go-zen"
    "github.com/Pavan-Silva/go-zen/middleware"
)

func main() {

## Configuration

Customize server timeouts and limits by passing a config to `zen.New()`:

```go
config := zen.DefaultConfig()
config.ReadTimeout = 10 * time.Second
config.WriteTimeout = 15 * time.Second
config.IdleTimeout = 90 * time.Second

r := zen.New(":8080", config)
````

Or create a config from scratch:

```go
config := zen.Config{
    ReadTimeout:       8 * time.Second,
    WriteTimeout:      12 * time.Second,
    IdleTimeout:       90 * time.Second,
    ReadHeaderTimeout: 2 * time.Second,
    MaxHeaderBytes:    2 << 20,
}

r := zen.New(":8080", config)
```

You can also replace the server entirely:

```go
config := zen.Config{
    Server: &http.Server{Addr: ":8080"},
}
r := zen.New(":8080", config)
```

## Context and Handlers

Handlers receive a `*zen.Context` and use request/response helpers directly.

```go
r.Handle("POST /users", func(c *zen.Context) {
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
```

You can also return XML responses:

```go
import "encoding/xml"

r.Handle("GET /users/{id}/xml", func(c *zen.Context) {
    userID := c.Param("id")
    user := struct {
        XMLName xml.Name `xml:"user"`
        ID      string   `xml:"id"`
        Name    string   `xml:"name"`
    }{
        ID:   userID,
        Name: "John Doe",
    }
    c.XML(http.StatusOK, user)
})
```

HTML responses:

```go
r.Handle("GET /page", func(c *zen.Context) {
    html := `
        <!DOCTYPE html>
        <html>
        <head><title>My App</title></head>
        <body><h1>Hello World</h1></body>
        </html>
    `
    c.HTML(http.StatusOK, html)
})
```

Plain text responses:

```go
r.Handle("GET /health", func(c *zen.Context) {
    c.String(http.StatusOK, "OK")
})

r.Handle("GET /token", func(c *zen.Context) {
    c.String(http.StatusOK, "your-secret-token")
})
```

No-content responses:

```go
r.Handle("DELETE /items/{id}", func(c *zen.Context) {
    c.NoContent(http.StatusNoContent)
})
```

Redirects:

```go
r.Handle("GET /old-path", func(c *zen.Context) {
    c.Redirect(http.StatusMovedPermanently, "/new-path")
})
```

Serve files:

```go
r.Handle("GET /download/report", func(c *zen.Context) {
    c.File("/tmp/report.pdf")
})

r.Handle("GET /download/attachment", func(c *zen.Context) {
    c.Attachment("/tmp/report.pdf", "monthly-report.pdf")
})

r.Handle("GET /image", func(c *zen.Context) {
    c.Inline("/tmp/image.png")
})
```

Blob responses:

```go
r.Handle("GET /csv", func(c *zen.Context) {
    data := []byte(`0306703,0035866,NO_ACTION,06/19/2006
0086003,"0005866",UPDATED,06/19/2006`)
    c.Blob(http.StatusOK, "text/csv", data)
})
```

Or Stream responses:

```go
r.Handle("GET /image-stream", func(c *zen.Context) {
    f, err := os.Open("/tmp/image.png")
    if err != nil {
        c.Error(http.StatusInternalServerError, err.Error())
        return
    }
    defer f.Close()
    _ = c.Stream(http.StatusOK, "image/png", f)
})
```

You can also read query and path parameters:

```go
r.Handle("GET /users/{id}", func(c *zen.Context) {
    page := c.QueryParam("page")
    userID := c.Param("id")
    c.JSON(http.StatusOK, map[string]string{"id": userID, "page": page})
})
```

### Context helpers

- `c.Request`, `c.Response`
- `c.QueryParam(key)`
- `c.Param(name)`
- `c.BindJSON(dest)`
- `c.BindForm(dest)`
- `c.BindFile(field)`
- `c.BindFiles(field)`
- `c.Body()`
- `c.JSON(status, data)`
- `c.XML(status, data)`
- `c.HTML(status, html)`
- `c.String(status, text)`
- `c.NoContent(status)`
- `c.Redirect(status, location)`
- `c.File(path)`
- `c.Attachment(path, attachmentName)`
- `c.Inline(path)`
- `c.Blob(status, contentType, data)`
- `c.Stream(status, contentType, body)`
- `c.Error(status, message)`
- `c.Set(key, value)`, `c.Get(key)`

## Middleware

Global middleware is registered with `r.Use(...)`.
Middleware executes in registration order and is pre-chained at startup.

```go
r.Use(middleware.Recover, middleware.Logger, middleware.CORS(corsConfig))

// Limit request body to 2MB
r.Use(middleware.BodyLimit("2M"))

// Or with custom config and skipper
config := middleware.BodyLimitConfig()
config.Limit = "1M"
config.Skipper = func(r *http.Request) bool {
    return r.URL.Path == "/upload/large"
}
r.Use(middleware.BodyLimitWithConfig(config))
```

### Built-in middleware

- `middleware.Logger` — request logging with method, path, status, latency, client IP, and response size
- `middleware.Recover` — recovers from panics and returns a 500 response
- `middleware.CORS` — flexible CORS support for origins, methods, headers, and credentials
- `middleware.BodyLimit` — limits request body size with support for skippers

## Static Files

Zen provides built-in support for serving static files and directories.

### Single File

Serve a single file for a specific route:

```go
r.File("GET /", "public/index.html")
```

### Directory Serving

Serve files from a local directory:

```go
r.Static("/images", "assets/images")
r.Static("/css", "public/css")
```

### Embedded Files

Serve files from an embedded filesystem:

```go
import (
    "embed"
    "io/fs"
)

//go:embed "assets/images"
var images embed.FS

func main() {
    r := zen.New(":8080")

    // Serve embedded files
    imagesFS, _ := fs.Sub(images, "assets/images")
    r.StaticFS("/images", imagesFS)

    r.ListenAndServe()
}
```

## Route Groups

Use route groups to apply prefixes and middleware to sets of handlers.

```go
api := r.Group("/api", authMiddleware)
api.Handle("GET /users", listUsers)
api.Handle("POST /users", createUser)

admin := api.Group("/admin", adminMiddleware)
admin.Handle("GET /stats", adminStats)
```

Group middleware is inherited by subgroup routes.

## Request Binding and Validation

Zen supports request binding for JSON, form data, multipart uploads, and raw request bodies.

```go
r.Handle("POST /submit", func(c *zen.Context) {
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

```go
r.Handle("POST /upload", func(c *zen.Context) {
    header, content, err := c.BindFile("avatar")
    if err != nil {
        c.Error(http.StatusBadRequest, err.Error())
        return
    }

    c.JSON(http.StatusOK, map[string]any{"filename": header.Filename, "size": len(content)})
})
```

```go
r.Handle("POST /data", func(c *zen.Context) {
    body, err := c.Body()
    if err != nil {
        c.Error(http.StatusInternalServerError, "failed to read body")
        return
    }

    c.JSON(http.StatusOK, map[string]int{"bytes": len(body)})
})
```

Built-in validation runs automatically during `BindJSON` when destination is a struct.

## Streaming and Real-time

Zen supports SSE and WebSockets through optional packages.

### SSE

```go
import "github.com/Pavan-Silva/go-zen/sse"

sse.Handle(r, "GET /events", func(c *zen.Context) error {
    return sse.Send(c, "message", map[string]string{"hello": "world"})
})
```

### WebSocket

```go
import "github.com/Pavan-Silva/go-zen/ws"

ws.Handle(r, "GET /ws", func(c *zen.Context, conn *ws.Conn) {
    var msg map[string]any
    conn.ReadJSON(&msg)
    conn.WriteJSON(map[string]string{"pong": "ok"})
})
```

## Authentication

The optional `auth` package supports multiple authenticator types: basic auth, JWT, OAuth2, OIDC, and session-based.
All can be used with HTTP, WebSocket, and SSE routes.

### Basic Auth

```go
basicAuth := &auth.BasicAuth{ValidatePassword: auth.ValidatePassword}
r.Use(auth.RequireAuth(basicAuth))
```

### JWT

```go
jwtAuth := &auth.JWTAuth{SecretKey: "your-secret-key"}
r.Use(auth.RequireAuth(jwtAuth))
```

### OAuth2 Token Introspection

```go
oauth2Auth := &auth.OAuth2Auth{
    TokenIntrospectionEndpoint: "https://oauth.example.com/introspect",
    ClientID: "your-client-id",
    ClientSecret: "your-client-secret",
}
r.Use(auth.RequireAuth(oauth2Auth))
```

### OIDC

```go
oidcAuth := &auth.OIDCAuth{
    Issuer: "https://accounts.google.com",
    ClientID: "your-client-id",
    UserInfoEndpoint: "https://www.googleapis.com/oauth2/v2/userinfo",
}
r.Use(auth.RequireAuth(oidcAuth))
```

### Session-based

```go
sessionStore := auth.NewInMemorySessionStore()
sessionAuth := &auth.SessionAuth{CookieName: "session_id", Store: sessionStore}
r.Use(auth.RequireAuth(sessionAuth))
```

### Using authenticated users

Access the authenticated user in any handler:

```go
r.Handle("GET /profile", func(c *zen.Context) {
    user := auth.GetUser(c)
    c.JSON(http.StatusOK, user)
})
```

Require specific roles with role-based middleware:

```go
r.Use(auth.RequireRole("admin"))
r.Handle("GET /admin/users", func(c *zen.Context) {
    c.JSON(http.StatusOK, map[string]string{"access": "admin only"})
})
```

### SSE authentication

```go
sse.HandleWithAuth(r, "GET /events", myAuthenticator, func(c *zen.Context) error {
    user := auth.GetUser(c)
    return sse.Send(c, "welcome", map[string]string{"user": user.Username})
})
```

### WebSocket authentication

```go
ws.HandleWithAuth(r, "GET /ws", myAuthenticator, func(c *zen.Context, conn *ws.Conn) {
    user := auth.GetUser(c)
    conn.WriteJSON(map[string]string{"welcome": user.Username})
})
```

## Contributing

Zen is designed for simplicity, performance, and clarity.
If you want to contribute, please submit issues or pull requests with focused improvements.

## License

Zen is released under the MIT License.
