# zen

`zen` is a tiny Go micro-framework that wraps the standard library's `net/http`
and related packages to reduce boilerplate for common tasks like graceful shutdown,
JSON/form parsing, and response helpers. It intentionally avoids reinventing routing
or introducing dependencies; everything is just thin helpers around the stock libs.

> **Name:** zen

## Features

- `zen.Context` for easy JSON responses, request body parsing, form binding, etc.
- `zen.HandlerFunc` + `Adapt` to convert your handlers for the standard mux.
- `zen.Server` with built-in graceful shutdown on SIGINT/SIGTERM.
- Zero external dependencies; just Go 1.21+.

## Example

```go
package main

import (
    "log"
    "net/http"

    "github.com/pavansilva/zen/zen"
)

func main() {
    mux := http.NewServeMux()

    mux.HandleFunc("/json", zen.Adapt(func(c *zen.Context) {
        var payload struct{ Message string `json:"message"` }
        _ = c.BindJSON(&payload)
        c.JSON(http.StatusOK, map[string]string{"echo": payload.Message})
    }))

    srv := zen.NewServer(":8080", mux)
    log.Println("listening on :8080")
    srv.ListenAndServe()
}
```

## Usage

Import the package and wrap your existing handlers with `zen.Adapt`. Create a
`zen.Server` instead of `http.Server` to get graceful shutdown out of the box.

The `Context` object carries `http.ResponseWriter` and `*http.Request` and adds
utility methods such as `JSON`, `BindJSON`, `BindForm`, `FormValue`, `QueryParam`,
and `Body`.

Server methods now automatically print the banner (like Gin/Echo)
when you call `ListenAndServe`/`ListenAndServeTLS`, so there’s no need to log
it yourself.

You can also show a startup banner with a clickable localhost link using
`zen.Banner(addr)` directly if you prefer; it's purely a helper.

### Request validation

`zen.Context` provides a small helper for validating structs based on
`go-playground/validator/v10` tags:

```go
var payload struct {
    Name  string `validate:"required"`
    Email string `validate:"required,email"`
}
if err := ctx.BindJSON(&payload); err != nil {
    // handle bad JSON
}
if err := ctx.Validate(&payload); err != nil {
    // validation failed
}
```

The global validator instance is lazily initialized and reused, so the cost
when validation is not in use is effectively zero. This keeps the framework
lean while offering familiar tag‑based checking when you need it.

Server methods automatically print the ASCII banner when you start them – you
no longer have to call `zen.Banner` yourself; the output is handled for you
during `ListenAndServe`/`ListenAndServeTLS`, mimicking the behaviour of Gin
and Echo.

### Tests & benchmarks

To avoid shipping test code with the library when other projects import it,
the tests and benchmarks now live in a top‑level `tests` directory (out of the
`zen` package). This mirrors how frameworks like [Gin](https://github.com/gin-gonic/gin)
and [Echo](https://github.com/labstack/echo) keep their test suites separate.

Run them with:

```sh
cd tests
go test             # unit tests
go test -bench=.    # performance and throughput benchmarks
```

The benchmarks include both serial and parallel (throughput) variations; they
compare `zen.Adapt` to a raw `http.HandlerFunc` and consistently show
single‑digit nanoseconds overhead per request. There is also a validation
scenario (`BenchmarkZenValidate`) which exercises JSON binding + tag
validation, running at roughly 300 ns per request.

## Philosophy

Keep it minimal. Don’t touch routing or introduce other abstractions that the
standard library already handles well. The goal is a small convenience layer
for applications that want less boilerplate without committing to a full
framework.

## License

MIT
