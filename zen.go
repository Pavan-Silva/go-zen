// Package zen is a lightweight, high-performance HTTP microframework built on Go's
// standard net/http and Go 1.22+ enhanced routing (http.ServeMux with method+pattern).
//
// # Quick Start
//
//	r := zen.New(":8080")
//	r.Handle("GET /hello", func(c *zen.Context) {
//	    c.String(200, "Hello, World!")
//	})
//	r.Run()
//
// # Middleware
//
// Zen supports both global and per-route middleware:
//
//	r.Use(middleware.Recover)
//	r.Use(middleware.Logger)
//	r.HandleWith("GET /admin", adminHandler, authMiddleware)
//
// # Routing
//
// Routes use Go 1.22+ patterns with method prefix and path parameters:
//
//	r.Handle("GET /users/{id}", getUser)
//	r.Handle("POST /users", createUser)
//
// # Route Groups
//
//	api := r.Group("/api")
//	api.Handle("GET /health", healthCheck)
//
// # Binding and Validation
//
//	var req CreateUserRequest
//	if err := c.BindJSON(&req); err != nil {
//	    c.Error(400, "invalid request")
//	    return
//	}
//	if err := zen.Validate(&req); err != nil {
//	    c.Error(400, err.Error())
//	    return
//	}
//
// # Context and Lifecycle
//
// A zen.Context is created per request via sync.Pool and released back after the
// request completes. The hot path avoids context.WithValue entirely; FromRequest
// is only needed for interop with standard http.Handler patterns.
//
// # Performance
//
// Zen is designed for zero-allocation middleware chains, pooled Context objects,
// and streaming JSON/XML encoding. Benchmark against net/http ServeMux shows
// ~95% throughput with comparable middleware at ~110k req/s.
package zen
