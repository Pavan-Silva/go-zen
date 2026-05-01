# Zen

A lightweight Go web framework built on the standard library `net/http`.
Zen gives you modern request routing, binding, middleware, and reusable helpers with a small API surface and no external router dependency.

## Why Zen?

- Minimal, stdlib-first framework for Go 1.22+
- Pattern-based routing via `http.ServeMux` and Go's route syntax
- Fast request lifecycle with pooled contexts and zero-allocation middleware chaining
- Secure defaults for timeouts, header limits, and graceful shutdown
- Built-in request binding for JSON, forms, multipart files, and raw bodies
- Optional helper packages for auth, Env, and SSE

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

app := zen.New(":8080", config)
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

app := zen.New(":8080", config)
```

You can also replace the server entirely:

```go
config := zen.Config{
    Server: &http.Server{Addr: ":8080"},
}
app := zen.New(":8080", config)
```

## Context and Handlers

Handlers receive a `*zen.Context` and use request/response helpers directly.

```go
app.Handle("POST /users", func(c *zen.Context) {
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

app.Handle("GET /users/{id}/xml", func(c *zen.Context) {
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
app.Handle("GET /page", func(c *zen.Context) {
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
app.Handle("GET /health", func(c *zen.Context) {
    c.String(http.StatusOK, "OK")
})

app.Handle("GET /token", func(c *zen.Context) {
    c.String(http.StatusOK, "your-secret-token")
})
```

No-content responses:

```go
app.Handle("DELETE /items/{id}", func(c *zen.Context) {
    c.NoContent(http.StatusNoContent)
})
```

Redirects:

```go
app.Handle("GET /old-path", func(c *zen.Context) {
    c.Redirect(http.StatusMovedPermanently, "/new-path")
})
```

Serve files:

```go
app.Handle("GET /download/report", func(c *zen.Context) {
    c.File("/tmp/report.pdf")
})

app.Handle("GET /download/attachment", func(c *zen.Context) {
    c.Attachment("/tmp/report.pdf", "monthly-report.pdf")
})

app.Handle("GET /image", func(c *zen.Context) {
    c.Inline("/tmp/image.png")
})
```

Blob responses:

```go
app.Handle("GET /csv", func(c *zen.Context) {
    data := []byte(`0306703,0035866,NO_ACTION,06/19/2006
0086003,"0005866",UPDATED,06/19/2006`)
    c.Blob(http.StatusOK, "text/csv", data)
})
```

Or Stream responses:

```go
app.Handle("GET /image-stream", func(c *zen.Context) {
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
app.Handle("GET /users/{id}", func(c *zen.Context) {
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
- `c.FormFile(field)`
- `c.FormFiles(field)`
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
app.Use(middleware.Recover, middleware.Logger, middleware.CORS(corsConfig))

// Limit request body to 2MB
app.Use(middleware.BodyLimit("2M"))

// Or with custom config and skipper
config := middleware.BodyLimitConfig()
config.Limit = "1M"
config.Skipper = func(r *http.Request) bool {
    return r.URL.Path == "/upload/large"
}
app.Use(middleware.BodyLimitWithConfig(config))
```

You can also apply a per-handler body limit manually:

```go
app.Handle("POST /upload", func(c *zen.Context) {
    const maxUploadSize = int64(10 << 20) // 10MB
    c.Request.Body = http.MaxBytesReader(c.Response, c.Request.Body, maxUploadSize)
    // proceed with BindJSON / Body / FormFile...
})
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
app.File("GET /", "public/index.html")
```

### Directory Serving

Serve files from a local directory:

```go
app.Static("/images", "assets/images")
app.Static("/css", "public/css")
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
    app := zen.New(":8080")

    // Serve embedded files
    imagesFS, _ := fs.Sub(images, "assets/images")
    app.StaticFS("/images", imagesFS)

    app.Run()
}
```

## Route Groups

Use route groups to apply prefixes and middleware to sets of handlers.

```go
api := app.Group("/api", authMiddleware)
api.Handle("GET /users", listUsers)
api.Handle("POST /users", createUser)

admin := api.Group("/admin", adminMiddleware)
admin.Handle("GET /stats", adminStats)
```

Group middleware is inherited by subgroup routes.

## Request Binding and Validation

Zen supports request binding for JSON, form data, multipart uploads, and raw request bodies.

```go
app.Handle("POST /submit", func(c *zen.Context) {
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
app.Handle("POST /upload", func(c *zen.Context) {
    header, content, err := c.FormFile("avatar")
    if err != nil {
        c.Error(http.StatusBadRequest, err.Error())
        return
    }

    c.JSON(http.StatusOK, map[string]any{"filename": header.Filename, "size": len(content)})
})
```

```go
app.Handle("POST /data", func(c *zen.Context) {
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

Zen supports SSE directly via `Context.SSEvent`. WebSockets integrate with any library via `HandleRaw`.

### SSE

```go
app.Handle("GET /events", func(c *zen.Context) {
    if err := c.SSEvent("message", map[string]string{"hello": "world"}); err != nil {
        c.Error(http.StatusInternalServerError, err.Error())
    }
})
```

### WebSocket Integration

Zen doesn't include a WebSocket implementation. Instead, use `HandleRaw` to integrate any WebSocket library. The auth package provides helpers for authenticated routes.

```go
import "github.com/gorilla/websocket"

upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

app.HandleRaw("GET /ws", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
    conn, err := upgrader.Upgrade(w, req, nil)
    if err != nil { return }
    defer conn.Close()
    // handle connection
}))
```

With authentication using `auth.WithAuthFunc`:

```go
import "github.com/Pavan-Silva/go-zen/auth"

jwtAuth := &auth.JWTAuth{Secret: []byte("your-secret-key")}

app.HandleRaw("GET /ws", auth.WithAuthFunc(func(w http.ResponseWriter, req *http.Request, user auth.User) {
    log.Printf("user %s connected", user.ID)
    conn, err := upgrader.Upgrade(w, req, nil)
    if err != nil { return }
    defer conn.Close()
    // handle connection - user.ID available for authorization
}, jwtAuth))
```

Low-level helpers are also available:

```go
// Wrap any http.Handler
app.HandleRaw("GET /ws", auth.WithAuth(handler, jwtAuth))

// Get user info in handler
app.HandleRaw("GET /ws", auth.WithAuthFunc(func(w, r, user) {
    // user.ID, user.Username, user.Roles available
}, jwtAuth))
```

## Authentication

The optional `auth` package supports multiple authenticator types: basic auth, JWT, OAuth2, OIDC, and session-based.
All can be used with HTTP and SSE routes.

### Basic Auth

```go
basicAuth := &auth.BasicAuth{
    Validate: func(username, password string) (auth.User, error) {
        // Validate credentials against your database
        if username == "admin" && auth.ValidatePassword(storedHash, password) {
            return auth.User{ID: "1", Username: username}, nil
        }
        return auth.User{}, fmt.Errorf("invalid credentials")
    },
}
app.Use(auth.RequireAuth(basicAuth))
```

### JWT

```go
jwtAuth := &auth.JWTAuth{Secret: []byte("your-secret-key")}
app.Use(auth.RequireAuth(jwtAuth))
```

### OAuth2 Token Introspection

```go
oauth2Auth := &auth.OAuth2Auth{
    TokenIntrospectionEndpoint: "https://oauth.example.com/introspect",
    ClientID: "your-client-id",
    ClientSecret: "your-client-secret",
}
app.Use(auth.RequireAuth(oauth2Auth))
```

### OIDC

```go
oidcAuth := &auth.OIDCAuth{
    Issuer: "https://accounts.google.com",
    ClientID: "your-client-id",
    UserInfoEndpoint: "https://www.googleapis.com/oauth2/v2/userinfo",
}
app.Use(auth.RequireAuth(oidcAuth))
```

### Session-based

```go
sessionStore := auth.NewInMemorySessionStore()
sessionAuth := &auth.SessionAuth{CookieName: "session_id", Store: sessionStore}
app.Use(auth.RequireAuth(sessionAuth))
```

### Using authenticated users

Access the authenticated user in any handler:

```go
app.Handle("GET /profile", func(c *zen.Context) {
    user := auth.GetUser(c)
    c.JSON(http.StatusOK, user)
})
```

Require specific roles with role-based middleware:

```go
app.Use(auth.RequireRole("admin"))
app.Handle("GET /admin/users", func(c *zen.Context) {
    c.JSON(http.StatusOK, map[string]string{"access": "admin only"})
})
```

### SSE authentication

```go
myAuthenticator := &auth.JWTAuth{Secret: []byte("your-secret-key")}
app.Handle("GET /events", func(c *zen.Context) {
    authUser, err := myAuthenticator.Authenticate(c.Request)
    if err != nil {
        c.Error(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
        return
    }
    c.Set("user", &authUser)

    user := auth.GetUser(c)
    if err := c.SSEvent("welcome", map[string]string{"user": user.Username}); err != nil {
        c.Error(http.StatusInternalServerError, err.Error())
    }
})
```

## Contributing

Zen is designed for simplicity, performance, and clarity.
If you want to contribute, please submit issues or pull requests with focused improvements.

## License

Zen is released under the MIT License.
