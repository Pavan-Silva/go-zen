# Zen

[![Go Reference](https://pkg.go.dev/badge/github.com/Pavan-Silva/go-zen.svg)](https://pkg.go.dev/github.com/Pavan-Silva/go-zen)

A lightweight, high-performance HTTP microframework for Go built on `net/http`. Performance on par with Gin and Echo, zero external router dependency.

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
    r.Run()
}
```

## Documentation

Full documentation is available at **[zen-docs-steel.vercel.app](https://zen-docs-steel.vercel.app)**.

## License

MIT
