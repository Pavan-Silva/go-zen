package zen_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/Pavan-Silva/zen/zen"
    "github.com/gin-gonic/gin"
    "github.com/labstack/echo/v4"
)

func BenchmarkZenAdapt(b *testing.B) {
    handler := zen.Adapt(func(c *zen.Context) {
        c.Response.Write([]byte("ok"))
    })
    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()
    for i := 0; i < b.N; i++ {
        handler(w, req)
    }
}

func BenchmarkStdLib(b *testing.B) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })
    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()
    for i := 0; i < b.N; i++ {
        handler(w, req)
    }
}

// Parallel throughput benchmarks
func BenchmarkParallelZen(b *testing.B) {
    handler := zen.Adapt(func(c *zen.Context) {
        c.Response.Write([]byte("ok"))
    })
    b.RunParallel(func(pb *testing.PB) {
        req := httptest.NewRequest("GET", "/", nil)
        w := httptest.NewRecorder()
        for pb.Next() {
            handler(w, req)
        }
    })
}

func BenchmarkParallelStdLib(b *testing.B) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })
    b.RunParallel(func(pb *testing.PB) {
        req := httptest.NewRequest("GET", "/", nil)
        w := httptest.NewRecorder()
        for pb.Next() {
            handler(w, req)
        }
    })
}

// Gin benchmarks
func BenchmarkGin(b *testing.B) {
    gin.SetMode(gin.ReleaseMode)
    r := gin.New()
    r.GET("/", func(c *gin.Context) {
        c.String(200, "ok")
    })
    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()
    for i := 0; i < b.N; i++ {
        r.ServeHTTP(w, req)
    }
}

func BenchmarkParallelGin(b *testing.B) {
    gin.SetMode(gin.ReleaseMode)
    r := gin.New()
    r.GET("/", func(c *gin.Context) {
        c.String(200, "ok")
    })
    b.RunParallel(func(pb *testing.PB) {
        req := httptest.NewRequest("GET", "/", nil)
        w := httptest.NewRecorder()
        for pb.Next() {
            r.ServeHTTP(w, req)
        }
    })
}

// Echo benchmarks
func BenchmarkEcho(b *testing.B) {
    e := echo.New()
    e.GET("/", func(c echo.Context) error {
        return c.String(200, "ok")
    })
    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()
    for i := 0; i < b.N; i++ {
        e.ServeHTTP(w, req)
    }
}

func BenchmarkParallelEcho(b *testing.B) {
    e := echo.New()
    e.GET("/", func(c echo.Context) error {
        return c.String(200, "ok")
    })
    b.RunParallel(func(pb *testing.PB) {
        req := httptest.NewRequest("GET", "/", nil)
        w := httptest.NewRecorder()
        for pb.Next() {
            e.ServeHTTP(w, req)
        }
    })
}
