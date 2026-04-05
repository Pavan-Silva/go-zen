// Package zen is a lightweight, high-performance microframework built on Go's standard net/http.
//
// Design Philosophy
//
// Zen prioritizes performance and simplicity over feature richness. It provides essential
// helpers (request context pooling, middleware, JSON/form binding) while leaving routing
// and advanced features to the standard library or your choice of packages.
//
// Performance Optimizations
//
// - Context pooling: Reuses Context allocations across requests to reduce GC pressure.
// - Zero-allocation middleware: Middleware chains are pre-built at setup time (no closures per request).
// - Reflection caching: Form/validator metadata is cached per type, not per request.
// - Buffer pooling: JSON response buffers are reused via sync.Pool.
// - Lazy initialization: Validators are created only when first used.
//
// Getting Started
//
//	r := zen.New(":8080")
//	r.Use(middleware.Logger, middleware.Recover)
//
//	r.Handle("GET /users/{id}", func(c *zen.Context) {
//	    id := c.Param("id")
//	    c.JSON(http.StatusOK, map[string]string{"id": id})
//	})
//
//	r.ListenAndServe()  // Blocks until Ctrl+C
//
// Request Binding
//
//	type UserRequest struct {
//	    Email string `json:"email" validate:"required,email"`
//	    Age   int    `json:"age" validate:"required,gte=18"`
//	}
//
//	var req UserRequest
//	if err := c.BindJSON(&req); err != nil {
//	    c.SendError(zen.BadRequest(err.Error()))
//	    return
//	}
//
// Route Groups
//
//	api := r.Group("/api/v1")
//	api.Handle("GET /health", healthHandler)
//
//	users := r.Group("/api/v1/users", authMiddleware)
//	users.Handle("GET /{id}", getUserHandler)
//
// Comparable to Gin/Echo in throughput while maintaining the simplicity and
// safety of Go's standard library.
package zen

