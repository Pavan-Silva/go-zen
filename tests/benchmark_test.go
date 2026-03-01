package zen_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "bytes"
    "github.com/go-playground/validator/v10"
    "github.com/gin-gonic/gin/binding"
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

// Validation performance
// shared named type for all three
type validatePayload struct {
    Username string `json:"username" validate:"required,alphanum"`
    Email    string `json:"email"    validate:"required,email"`
}

// Zen — already correct
func BenchmarkZenValidate(b *testing.B) {
    handler := zen.Adapt(func(c *zen.Context) {
        var payload validatePayload
        _ = c.BindJSON(&payload)
        _ = c.Validate(&payload)
        c.JSON(200, "ok")
    })
    body := []byte(`{"username":"foo","email":"foo@bar.com"}`)
    for i := 0; i < b.N; i++ {
        req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
        w := httptest.NewRecorder()
        handler(w, req)
    }
}

// Gin — register validator
func BenchmarkGinValidate(b *testing.B) {
    gin.SetMode(gin.ReleaseMode)
    r := gin.New()
    r.POST("/", func(c *gin.Context) {
        var payload validatePayload
        if err := c.ShouldBindJSON(&payload); err != nil {
            c.JSON(400, err.Error())
            return
        }
        if err := binding.Validator.ValidateStruct(&payload); err != nil {
            c.JSON(400, err.Error())
            return
        }
        c.JSON(200, "ok")
    })
    body := []byte(`{"username":"foo","email":"foo@bar.com"}`)
    for i := 0; i < b.N; i++ {
        req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
        w := httptest.NewRecorder()
        r.ServeHTTP(w, req)
    }
}

// Echo — register validator
type echoValidator struct {
    v *validator.Validate
}
func (ev *echoValidator) Validate(i interface{}) error {
    return ev.v.Struct(i)
}

func BenchmarkEchoValidate(b *testing.B) {
    e := echo.New()
    e.Validator = &echoValidator{v: validator.New()}
    e.POST("/", func(c echo.Context) error {
        var payload validatePayload
        if err := c.Bind(&payload); err != nil {
            return err
        }
        if err := c.Validate(&payload); err != nil {
            return err
        }
        return c.JSON(200, "ok")
    })
    body := []byte(`{"username":"foo","email":"foo@bar.com"}`)
    for i := 0; i < b.N; i++ {
        req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
        w := httptest.NewRecorder()
        e.ServeHTTP(w, req)
    }
}
