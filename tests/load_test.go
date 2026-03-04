package zen_test

// Run all:
//   go test -bench=. -benchtime=5s -benchmem -run=^$ ./tests/
//
// Run throughput only:
//   go test -bench=BenchmarkThroughput -benchtime=5s -benchmem -run=^$ ./tests/
//
// Run latency only:
//   go test -bench=BenchmarkLatency -benchtime=5s -benchmem -run=^$ ./tests/
//
// Run one scenario across all frameworks:
//   go test -bench=BenchmarkThroughput_.*_PlaceOrder    -benchtime=5s -benchmem -run=^$ ./tests/
//   go test -bench=BenchmarkLatency_.*_AuthGatedAPI     -benchtime=5s -benchmem -run=^$ ./tests/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Pavan-Silva/zen/zen"
	middleware2 "github.com/Pavan-Silva/zen/zen/middleware"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Domain types
// ═══════════════════════════════════════════════════════════════════════════════

type tlUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Age      int    `json:"age"`
}

type tlLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tlLoginResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}

type tlProduct struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Stock    int     `json:"stock"`
	Category string  `json:"category"`
}

type tlOrder struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Total     float64 `json:"total"`
}

type tlSearchResult struct {
	Items []tlProduct `json:"items"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Mock data
// ═══════════════════════════════════════════════════════════════════════════════

var tlUsers = map[string]tlUser{
	"alice": {ID: 1, Username: "alice", Email: "alice@example.com", Age: 30},
	"bob":   {ID: 2, Username: "bob", Email: "bob@example.com", Age: 25},
}

var tlCatalog = func() []tlProduct {
	p := make([]tlProduct, 50)
	categories := []string{"electronics", "clothing", "food", "books", "sports"}
	for i := range p {
		p[i] = tlProduct{
			ID:       i + 1,
			Name:     fmt.Sprintf("Product %d", i+1),
			Price:    float64(i+1) * 9.99,
			Stock:    100 - i,
			Category: categories[i%len(categories)],
		}
	}
	return p
}()

var tlLargeUsers = func() []tlUser {
	u := make([]tlUser, 100)
	for i := range u {
		u[i] = tlUser{
			ID:       i + 1,
			Username: "user" + strings.Repeat("x", i%10),
			Email:    "user@example.com",
			Age:      20 + (i % 50),
		}
	}
	return u
}()

var (
	tlOrderBody   = tlMustMarshal(tlOrder{UserID: 1, ProductID: 3, Quantity: 2})
	tlProductBody = tlMustMarshal(tlProduct{Name: "Wireless Mouse", Price: 29.99, Stock: 50, Category: "electronics"})
	tlLoginBody   = tlMustMarshal(tlLoginRequest{Username: "alice", Password: "supersecret123"})
)

func tlDBLookup(username string) (tlUser, bool) {
	u, ok := tlUsers[username]
	return u, ok
}

// ═══════════════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════════════

func tlMustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func tlBody(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

// tlRPS spins up `workers` goroutines for duration d.
// Each goroutine allocates its request ONCE and reuses it across iterations,
// eliminating httptest.NewRequest allocation noise from the measurement.
// For GET requests, the request is fully reusable with no reset needed.
func tlRPS(handler http.Handler, makeReq func() *http.Request, workers int, d time.Duration) float64 {
	var total atomic.Int64
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := makeReq() // allocate once per goroutine, reuse across iterations
			for {
				select {
				case <-stop:
					return
				default:
					w.Body.Reset()
					handler.ServeHTTP(w, req)
					total.Add(1)
				}
			}
		}()
	}
	time.Sleep(d)
	close(stop)
	wg.Wait()
	return float64(total.Load()) / d.Seconds()
}

// tlRPSPost is like tlRPS but for POST requests where the body is consumed
// each iteration. resetBody resets the body reader without allocating a new request.
func tlRPSPost(handler http.Handler, makeReq func() *http.Request, resetBody func(*http.Request), workers int, d time.Duration) float64 {
	var total atomic.Int64
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := makeReq() // allocate once per goroutine
			for {
				select {
				case <-stop:
					return
				default:
					w.Body.Reset()
					resetBody(req) // reset body reader only, no new request alloc
					handler.ServeHTTP(w, req)
					total.Add(1)
				}
			}
		}()
	}
	time.Sleep(d)
	close(stop)
	wg.Wait()
	return float64(total.Load()) / d.Seconds()
}

// latRec records per-request durations and reports p50/p95/p99/mean/stddev.
type latRec struct {
	mu      sync.Mutex
	samples []time.Duration
}

func (l *latRec) add(d time.Duration) {
	l.mu.Lock()
	l.samples = append(l.samples, d)
	l.mu.Unlock()
}

func (l *latRec) report(b *testing.B) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.samples) == 0 {
		return
	}
	sort.Slice(l.samples, func(i, j int) bool { return l.samples[i] < l.samples[j] })
	n := len(l.samples)
	var sum time.Duration
	for _, s := range l.samples {
		sum += s
	}
	mean := sum / time.Duration(n)
	p50 := l.samples[int(float64(n)*0.50)]
	p95 := l.samples[int(float64(n)*0.95)]
	p99 := l.samples[int(float64(n)*0.99)]
	var variance float64
	for _, s := range l.samples {
		diff := float64(s - mean)
		variance += diff * diff
	}
	stddev := time.Duration(math.Sqrt(variance / float64(n)))
	b.ReportMetric(float64(mean.Microseconds()), "mean_µs")
	b.ReportMetric(float64(p50.Microseconds()), "p50_µs")
	b.ReportMetric(float64(p95.Microseconds()), "p95_µs")
	b.ReportMetric(float64(p99.Microseconds()), "p99_µs")
	b.ReportMetric(float64(l.samples[0].Microseconds()), "min_µs")
	b.ReportMetric(float64(l.samples[n-1].Microseconds()), "max_µs")
	b.ReportMetric(float64(stddev.Microseconds()), "stddev_µs")
}

// ═══════════════════════════════════════════════════════════════════════════════
// THROUGHPUT — 1. Product Catalog
// Read-heavy paginated listing with category filter.
// 10 concurrent workers, 2s window. GET request reused per goroutine.
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkThroughput_Zen_ProductCatalog(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/products", func(c *zen.Context) {
		cat := c.QueryParam("category")
		out := make([]tlProduct, 0, 10)
		for _, p := range tlCatalog {
			if cat == "" || p.Category == cat {
				out = append(out, p)
				if len(out) == 10 {
					break
				}
			}
		}
		c.JSON(200, tlSearchResult{Items: out, Total: len(tlCatalog), Page: 1, Limit: 10})
	})
	req := httptest.NewRequest(http.MethodGet, "/products?page=1&category=electronics", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mux.Handler.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(mux.Handler, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/products?page=1&category=electronics", nil)
	}, 10, 2*time.Second), "req/s")
}

func BenchmarkThroughput_Gin_ProductCatalog(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/products", func(c *gin.Context) {
		cat := c.Query("category")
		out := make([]tlProduct, 0, 10)
		for _, p := range tlCatalog {
			if cat == "" || p.Category == cat {
				out = append(out, p)
				if len(out) == 10 {
					break
				}
			}
		}
		c.JSON(200, tlSearchResult{Items: out, Total: len(tlCatalog), Page: 1, Limit: 10})
	})
	req := httptest.NewRequest(http.MethodGet, "/products?page=1&category=electronics", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(r, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/products?page=1&category=electronics", nil)
	}, 10, 2*time.Second), "req/s")
}

func BenchmarkThroughput_Echo_ProductCatalog(b *testing.B) {
	e := echo.New()
	e.GET("/products", func(c echo.Context) error {
		cat := c.QueryParam("category")
		out := make([]tlProduct, 0, 10)
		for _, p := range tlCatalog {
			if cat == "" || p.Category == cat {
				out = append(out, p)
				if len(out) == 10 {
					break
				}
			}
		}
		return c.JSON(200, tlSearchResult{Items: out, Total: len(tlCatalog), Page: 1, Limit: 10})
	})
	req := httptest.NewRequest(http.MethodGet, "/products?page=1&category=electronics", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		e.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(e, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/products?page=1&category=electronics", nil)
	}, 10, 2*time.Second), "req/s")
}

func BenchmarkThroughput_StdLib_ProductCatalog(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		cat := r.URL.Query().Get("category")
		out := make([]tlProduct, 0, 10)
		for _, p := range tlCatalog {
			if cat == "" || p.Category == cat {
				out = append(out, p)
				if len(out) == 10 {
					break
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tlSearchResult{Items: out, Total: len(tlCatalog), Page: 1, Limit: 10})
	})
	req := httptest.NewRequest(http.MethodGet, "/products?page=1&category=electronics", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mux.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(mux, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/products?page=1&category=electronics", nil)
	}, 10, 2*time.Second), "req/s")
}

// ═══════════════════════════════════════════════════════════════════════════════
// THROUGHPUT — 2. Place Order
// Write-heavy POST: decode body, bounds-check product, compute total, 201.
// Uses tlRPSPost so body is reset per iteration without allocating a new request.
// 10 concurrent workers, 2s window.
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkThroughput_Zen_PlaceOrder(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/orders", func(c *zen.Context) {
		var o tlOrder
		if err := c.BindJSON(&o); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		if o.ProductID < 1 || o.ProductID > len(tlCatalog) {
			c.JSON(400, map[string]string{"error": "invalid product_id"})
			return
		}
		o.ID = 1001
		o.Total = float64(o.Quantity) * tlCatalog[o.ProductID-1].Price
		c.JSON(201, o)
	})
	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = tlBody(tlOrderBody)
		w.Body.Reset()
		mux.Handler.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPSPost(mux.Handler,
		func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/orders", nil)
			r.Header.Set("Content-Type", "application/json")
			return r
		},
		func(r *http.Request) { r.Body = tlBody(tlOrderBody) },
		10, 2*time.Second,
	), "req/s")
}

func BenchmarkThroughput_Gin_PlaceOrder(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/orders", func(c *gin.Context) {
		var o tlOrder
		if err := c.ShouldBindJSON(&o); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if o.ProductID < 1 || o.ProductID > len(tlCatalog) {
			c.JSON(400, gin.H{"error": "invalid product_id"})
			return
		}
		o.ID = 1001
		o.Total = float64(o.Quantity) * tlCatalog[o.ProductID-1].Price
		c.JSON(201, o)
	})
	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = tlBody(tlOrderBody)
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPSPost(r,
		func() *http.Request {
			rr := httptest.NewRequest(http.MethodPost, "/orders", nil)
			rr.Header.Set("Content-Type", "application/json")
			return rr
		},
		func(rr *http.Request) { rr.Body = tlBody(tlOrderBody) },
		10, 2*time.Second,
	), "req/s")
}

func BenchmarkThroughput_Echo_PlaceOrder(b *testing.B) {
	e := echo.New()
	e.POST("/orders", func(c echo.Context) error {
		var o tlOrder
		if err := c.Bind(&o); err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		if o.ProductID < 1 || o.ProductID > len(tlCatalog) {
			return c.JSON(400, map[string]string{"error": "invalid product_id"})
		}
		o.ID = 1001
		o.Total = float64(o.Quantity) * tlCatalog[o.ProductID-1].Price
		return c.JSON(201, o)
	})
	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = tlBody(tlOrderBody)
		w.Body.Reset()
		e.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPSPost(e,
		func() *http.Request {
			rr := httptest.NewRequest(http.MethodPost, "/orders", nil)
			rr.Header.Set("Content-Type", "application/json")
			return rr
		},
		func(rr *http.Request) { rr.Body = tlBody(tlOrderBody) },
		10, 2*time.Second,
	), "req/s")
}

func BenchmarkThroughput_StdLib_PlaceOrder(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		var o tlOrder
		if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if o.ProductID < 1 || o.ProductID > len(tlCatalog) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid product_id"})
			return
		}
		o.ID = 1001
		o.Total = float64(o.Quantity) * tlCatalog[o.ProductID-1].Price
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(o)
	})
	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = tlBody(tlOrderBody)
		w.Body.Reset()
		mux.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPSPost(mux,
		func() *http.Request {
			rr := httptest.NewRequest(http.MethodPost, "/orders", nil)
			rr.Header.Set("Content-Type", "application/json")
			return rr
		},
		func(rr *http.Request) { rr.Body = tlBody(tlOrderBody) },
		10, 2*time.Second,
	), "req/s")
}

// ═══════════════════════════════════════════════════════════════════════════════
// THROUGHPUT — 3. Concurrent Burst
// 50 goroutines hammering a parameterised GET for 3s.
// Request allocated once per goroutine — measures pure framework dispatch cost
// under goroutine scheduling pressure, not httptest.NewRequest overhead.
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkThroughput_Zen_ConcurrentBurst(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/products/:id", func(c *zen.Context) { c.JSON(200, tlCatalog[0]) })
	req := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mux.Handler.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(mux.Handler, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/products/1", nil)
	}, 50, 3*time.Second), "req/s")
}

func BenchmarkThroughput_Gin_ConcurrentBurst(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/products/:id", func(c *gin.Context) { c.JSON(200, tlCatalog[0]) })
	req := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(r, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/products/1", nil)
	}, 50, 3*time.Second), "req/s")
}

func BenchmarkThroughput_Echo_ConcurrentBurst(b *testing.B) {
	e := echo.New()
	e.GET("/products/:id", func(c echo.Context) error { return c.JSON(200, tlCatalog[0]) })
	req := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		e.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(e, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/products/1", nil)
	}, 50, 3*time.Second), "req/s")
}

func BenchmarkThroughput_StdLib_ConcurrentBurst(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tlCatalog[0])
	})
	req := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mux.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(mux, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/products/1", nil)
	}, 50, 3*time.Second), "req/s")
}

// ═══════════════════════════════════════════════════════════════════════════════
// THROUGHPUT — 4. Large Payload
// Serialise 100 users — tests JSON throughput at scale (dashboards, exports).
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkThroughput_Zen_LargePayload(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/users", func(c *zen.Context) { c.JSON(200, tlLargeUsers) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mux.Handler.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(mux.Handler, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/users", nil)
	}, 10, 2*time.Second), "req/s")
}

func BenchmarkThroughput_Gin_LargePayload(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/users", func(c *gin.Context) { c.JSON(200, tlLargeUsers) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(r, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/users", nil)
	}, 10, 2*time.Second), "req/s")
}

func BenchmarkThroughput_Echo_LargePayload(b *testing.B) {
	e := echo.New()
	e.GET("/users", func(c echo.Context) error { return c.JSON(200, tlLargeUsers) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		e.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(e, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/users", nil)
	}, 10, 2*time.Second), "req/s")
}

func BenchmarkThroughput_StdLib_LargePayload(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tlLargeUsers)
	})
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mux.ServeHTTP(w, req)
	}
	b.ReportMetric(tlRPS(mux, func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/users", nil)
	}, 10, 2*time.Second), "req/s")
}

// ═══════════════════════════════════════════════════════════════════════════════
// LATENCY — 1. Auth-Gated API
// Recover + token-check middleware → JSON profile response.
// p95/p99 reveal tail cost of the middleware chain.
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkLatency_Zen_AuthGatedAPI(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Use(middleware2.Recover)
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := r.Header.Get("Authorization")
			if len(tok) < 7 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	mux.Handle("/api/profile", func(c *zen.Context) { c.JSON(200, tlUsers["alice"]) })
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer mock.jwt.token")
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		start := time.Now()
		mux.Handler.ServeHTTP(w, req)
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_Gin_AuthGatedAPI(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		tok := c.GetHeader("Authorization")
		if len(tok) < 7 {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	})
	r.GET("/api/profile", func(c *gin.Context) { c.JSON(200, tlUsers["alice"]) })
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer mock.jwt.token")
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		start := time.Now()
		r.ServeHTTP(w, req)
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_Echo_AuthGatedAPI(b *testing.B) {
	e := echo.New()
	e.Use(echomiddleware.Recover())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tok := c.Request().Header.Get("Authorization")
			if len(tok) < 7 {
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			return next(c)
		}
	})
	e.GET("/api/profile", func(c echo.Context) error { return c.JSON(200, tlUsers["alice"]) })
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer mock.jwt.token")
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		start := time.Now()
		e.ServeHTTP(w, req)
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_StdLib_AuthGatedAPI(b *testing.B) {
	recover_ := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					http.Error(w, "internal server error", 500)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := r.Header.Get("Authorization")
			if len(tok) < 7 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/api/profile", recover_(auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tlUsers["alice"])
	}))))
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer mock.jwt.token")
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		start := time.Now()
		mux.ServeHTTP(w, req)
		rec.add(time.Since(start))
	}
	rec.report(b)
}

// ═══════════════════════════════════════════════════════════════════════════════
// LATENCY — 2. Mixed Read/Write
// 2 GETs then 1 POST in rotation. POST decodes a product body.
// p99 shows how write latency bleeds into tail distribution.
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkLatency_Zen_MixedReadWrite(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/products/:id", func(c *zen.Context) { c.JSON(200, tlCatalog[0]) })
	mux.Handle("/products", func(c *zen.Context) {
		var p tlProduct
		if err := c.BindJSON(&p); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		p.ID = 999
		c.JSON(201, p)
	})
	getReq := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		start := time.Now()
		if i%3 == 0 {
			r := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(tlProductBody))
			r.Header.Set("Content-Type", "application/json")
			mux.Handler.ServeHTTP(w, r)
		} else {
			mux.Handler.ServeHTTP(w, getReq)
		}
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_Gin_MixedReadWrite(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/products/:id", func(c *gin.Context) { c.JSON(200, tlCatalog[0]) })
	r.POST("/products", func(c *gin.Context) {
		var p tlProduct
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		p.ID = 999
		c.JSON(201, p)
	})
	getReq := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		start := time.Now()
		if i%3 == 0 {
			rr := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(tlProductBody))
			rr.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, rr)
		} else {
			r.ServeHTTP(w, getReq)
		}
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_Echo_MixedReadWrite(b *testing.B) {
	e := echo.New()
	e.GET("/products/:id", func(c echo.Context) error { return c.JSON(200, tlCatalog[0]) })
	e.POST("/products", func(c echo.Context) error {
		var p tlProduct
		if err := c.Bind(&p); err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		p.ID = 999
		return c.JSON(201, p)
	})
	getReq := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		start := time.Now()
		if i%3 == 0 {
			rr := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(tlProductBody))
			rr.Header.Set("Content-Type", "application/json")
			e.ServeHTTP(w, rr)
		} else {
			e.ServeHTTP(w, getReq)
		}
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_StdLib_MixedReadWrite(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tlCatalog[0])
	})
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		var p tlProduct
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		p.ID = 999
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(p)
	})
	getReq := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		start := time.Now()
		if i%3 == 0 {
			rr := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(tlProductBody))
			rr.Header.Set("Content-Type", "application/json")
			mux.ServeHTTP(w, rr)
		} else {
			mux.ServeHTTP(w, getReq)
		}
		rec.add(time.Since(start))
	}
	rec.report(b)
}

// ═══════════════════════════════════════════════════════════════════════════════
// LATENCY — 3. Login Flow
// Decode credentials → map lookup → JWT-style 200.
// Most common authenticated entry point — latency here directly affects UX.
// ═══════════════════════════════════════════════════════════════════════════════

func BenchmarkLatency_Zen_Login(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/login", func(c *zen.Context) {
		var req tlLoginRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		if _, ok := tlDBLookup(req.Username); !ok {
			c.JSON(401, map[string]string{"error": "unauthorized"})
			return
		}
		c.JSON(200, tlLoginResponse{Token: "eyJhbGciOiJIUzI1NiJ9.mock", Message: "login successful"})
	})
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = tlBody(tlLoginBody)
		w.Body.Reset()
		start := time.Now()
		mux.Handler.ServeHTTP(w, req)
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_Gin_Login(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.POST("/login", func(c *gin.Context) {
		var req tlLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if _, ok := tlDBLookup(req.Username); !ok {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(200, tlLoginResponse{Token: "eyJhbGciOiJIUzI1NiJ9.mock", Message: "login successful"})
	})
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = tlBody(tlLoginBody)
		w.Body.Reset()
		start := time.Now()
		router.ServeHTTP(w, req)
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_Echo_Login(b *testing.B) {
	e := echo.New()
	e.POST("/login", func(c echo.Context) error {
		var req tlLoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		if _, ok := tlDBLookup(req.Username); !ok {
			return c.JSON(401, map[string]string{"error": "unauthorized"})
		}
		return c.JSON(200, tlLoginResponse{Token: "eyJhbGciOiJIUzI1NiJ9.mock", Message: "login successful"})
	})
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = tlBody(tlLoginBody)
		w.Body.Reset()
		start := time.Now()
		e.ServeHTTP(w, req)
		rec.add(time.Since(start))
	}
	rec.report(b)
}

func BenchmarkLatency_StdLib_Login(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		var req tlLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if _, ok := tlDBLookup(req.Username); !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tlLoginResponse{Token: "eyJhbGciOiJIUzI1NiJ9.mock", Message: "login successful"})
	})
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rec := &latRec{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Body = tlBody(tlLoginBody)
		w.Body.Reset()
		start := time.Now()
		mux.ServeHTTP(w, req)
		rec.add(time.Since(start))
	}
	rec.report(b)
}
