// Package zen is a lightweight, high-performance HTTP microframework built on
// net/http with a custom radix tree router.
//
// # Quick Start
//
//	r := zen.New(":8080")
//	r.GET("/hello", func(c *zen.Ctx) {
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
//
// # Routing
//
// Routes use Go 1.22+ patterns with path parameters:
//
//	r.GET("/users/{id}", getUser)
//	r.POST("/users", createUser)
//	r.PUT("/users/{id}", updateUser)
//	r.DELETE("/users/{id}", deleteUser)
//
// # Route Groups
//
//	api := r.Group("/api")
//	api.HandleRaw("GET /health", healthCheck)
//
// # Binding and Validation
//
//	var req CreateUserRequest
//	if err := c.BindJSON(&req); err != nil {
//	    c.Error(400, "invalid request")
//	    return
//	}
//	if err := c.Validate(&req); err != nil {
//	    c.Error(400, err.Error())
//	    return
//	}
//
// # Context and Lifecycle
//
// A zen.Ctx is created per request via sync.Pool and released back after the
// request completes. The hot path avoids context.WithValue entirely; FromRequest
// is only needed for interop with standard http.Handler patterns.
//
// # Performance
//
// Zen is designed for zero-allocation middleware chains, pooled Ctx
// and streaming JSON/XML encoding.
package zen
