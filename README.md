# Zen

[![Go Reference](https://pkg.go.dev/badge/github.com/Pavan-Silva/go-zen.svg)](https://pkg.go.dev/github.com/Pavan-Silva/go-zen)

A lightweight, high-performance HTTP microframework for Go built on `net/http`. Zero-allocation radix tree routing, batteries-included middleware, and a minimal API.

## Features

- **Radix tree router** - per-method radix tree with zero-alloc path params, 405 detection, trailing-slash redirect
- **Request binding** - bind JSON, XML, form, query, headers, and path params via struct tags
- **Pluggable validation** - `go-playground/validator` integration with optional auto-validation
- **OpenAPI generation** - auto-generated 3.0.3 spec + Swagger UI, zero cost when unused
- **Auth** - JWT, Basic, API Key, Session, OAuth2, OIDC with RBAC/PBAC (separate `auth` package)
- **8 built-in middleware** - Recover, Logger, CORS, CSRF, Compress, Rate Limit, Body Limit, Pprof
- **Structured logger** - colored output, slog levels (FATAL/ERROR/WARN/INFO/DEBUG/TRACE), NO_COLOR support
- **Zero external router dependencies** - pure `net/http` + `slog`

## Installation

```bash
go get github.com/Pavan-Silva/go-zen
```

## Quick Start

```go
package main

import (
    "github.com/Pavan-Silva/go-zen"
    "github.com/Pavan-Silva/go-zen/middleware"
)

func main() {
    r := zen.New(":8080")
    r.Use(middleware.Recover, middleware.Logger)

    r.GET("/hello", func(c *zen.Ctx) {
        c.String(200, "Hello, World!")
    })

    r.GET("/users/:id", func(c *zen.Ctx) {
        id := c.Param("id")
        c.JSON(200, map[string]string{"id": id})
    })

    r.Run()
}
```

## Documentation

Full docs at **[zen-docs-steel.vercel.app](https://zen-docs-steel.vercel.app)**.

## License

MIT
