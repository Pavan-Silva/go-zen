package zen_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pavan-Silva/zen/zen"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// ─── Shared types ─────────────────────────────────────────────────────────────

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username" validate:"required,alphanum,min=3,max=32"`
	Email    string `json:"email"    validate:"required,email"`
	Age      int    `json:"age"      validate:"required,gte=1,lte=120"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}

type echoValidator struct{ v *validator.Validate }

func (ev *echoValidator) Validate(i interface{}) error { return ev.v.Struct(i) }

var mockUsers = map[string]User{
	"alice": {ID: 1, Username: "alice", Email: "alice@example.com", Age: 30},
	"bob":   {ID: 2, Username: "bob", Email: "bob@example.com", Age: 25},
}

func simulateDBLookup(username string) (User, bool) {
	u, ok := mockUsers[username]
	return u, ok
}

// ─── 1. Hello World ───────────────────────────────────────────────────────────

func BenchmarkZen_HelloWorld(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/", func(c *zen.Context) {
		c.Response.Write([]byte("Hello, World!"))
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.Handler.ServeHTTP(w, req)
	}
}

func BenchmarkGin_HelloWorld(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/", func(c *gin.Context) { c.String(200, "Hello, World!") })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_HelloWorld(b *testing.B) {
	e := echo.New()
	e.GET("/", func(c echo.Context) error { return c.String(200, "Hello, World!") })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(w, req)
	}
}

func BenchmarkStdLib_HelloWorld(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(w, req)
	}
}

// ─── 2. JSON Response ─────────────────────────────────────────────────────────

func BenchmarkZen_JSONResponse(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/users/1", func(c *zen.Context) {
		c.JSON(200, User{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30})
	})
	req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.Handler.ServeHTTP(w, req)
	}
}

func BenchmarkGin_JSONResponse(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/users/:id", func(c *gin.Context) {
		c.JSON(200, User{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30})
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_JSONResponse(b *testing.B) {
	e := echo.New()
	e.GET("/users/:id", func(c echo.Context) error {
		return c.JSON(200, User{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30})
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkStdLib_JSONResponse(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/1", func(w http.ResponseWriter, r *http.Request) {
		u := User{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	})
	req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(w, req)
	}
}

// ─── 3. JSON Bind + Validate + Respond ───────────────────────────────────────

var createUserBody = mustMarshal(User{Username: "charlie", Email: "charlie@example.com", Age: 28})

func BenchmarkZen_CreateUser(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/users", func(c *zen.Context) {
		var u User
		if err := c.BindJSON(&u); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		u.ID = 99
		c.JSON(201, u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(createUserBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.Handler.ServeHTTP(w, req)
	}
}

func BenchmarkGin_CreateUser(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/users", func(c *gin.Context) {
		var u User
		if err := c.ShouldBindJSON(&u); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := binding.Validator.ValidateStruct(&u); err != nil {
			c.JSON(422, gin.H{"error": err.Error()})
			return
		}
		u.ID = 99
		c.JSON(201, u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(createUserBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_CreateUser(b *testing.B) {
	e := echo.New()
	e.Validator = &echoValidator{v: validator.New()}
	e.POST("/users", func(c echo.Context) error {
		var u User
		if err := c.Bind(&u); err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		if err := c.Validate(&u); err != nil {
			return c.JSON(422, map[string]string{"error": err.Error()})
		}
		u.ID = 99
		return c.JSON(201, u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(createUserBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkStdLib_CreateUser(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		u.ID = 99
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(createUserBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}
}

// ─── 4. Login Flow ────────────────────────────────────────────────────────────

var loginBody = mustMarshal(LoginRequest{Username: "alice", Password: "supersecret123"})

func BenchmarkZen_Login(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/login", func(c *zen.Context) {
		var req LoginRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		if _, ok := simulateDBLookup(req.Username); !ok {
			c.JSON(401, map[string]string{"error": "unauthorized"})
			return
		}
		c.JSON(200, LoginResponse{Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.mock", Message: "login successful"})
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.Handler.ServeHTTP(w, r)
	}
}

func BenchmarkGin_Login(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.POST("/login", func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid body"})
			return
		}
		if err := binding.Validator.ValidateStruct(&req); err != nil {
			c.JSON(422, gin.H{"error": err.Error()})
			return
		}
		if _, ok := simulateDBLookup(req.Username); !ok {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(200, LoginResponse{Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.mock", Message: "login successful"})
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
	}
}

func BenchmarkEcho_Login(b *testing.B) {
	e := echo.New()
	e.Validator = &echoValidator{v: validator.New()}
	e.POST("/login", func(c echo.Context) error {
		var req LoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid body"})
		}
		if err := c.Validate(&req); err != nil {
			return c.JSON(422, map[string]string{"error": err.Error()})
		}
		if _, ok := simulateDBLookup(req.Username); !ok {
			return c.JSON(401, map[string]string{"error": "unauthorized"})
		}
		return c.JSON(200, LoginResponse{Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.mock", Message: "login successful"})
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
	}
}

func BenchmarkStdLib_Login(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if _, ok := simulateDBLookup(req.Username); !ok {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.mock", Message: "login successful"})
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
	}
}

// ─── 5. Middleware Chain ──────────────────────────────────────────────────────

func BenchmarkZen_MiddlewareChain(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Use(zen.Recover)
	mux.Use(func(next func(*zen.Context)) func(*zen.Context) {
		return func(c *zen.Context) {
			if c.Request.Header.Get("Authorization") == "" {
				http.Error(c.Response, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(c)
		}
	})
	mux.Handle("/protected", func(c *zen.Context) {
		c.JSON(200, map[string]string{"status": "ok"})
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.Handler.ServeHTTP(w, req)
	}
}

func BenchmarkGin_MiddlewareChain(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	})
	r.GET("/protected", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_MiddlewareChain(b *testing.B) {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("Authorization") == "" {
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			return next(c)
		}
	})
	e.GET("/protected", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(w, req)
	}
}

func BenchmarkStdLib_MiddlewareChain(b *testing.B) {
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/protected", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})))
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(w, req)
	}
}

// ─── 6. List Users ────────────────────────────────────────────────────────────

var userList = []User{
	{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30},
	{ID: 2, Username: "bob", Email: "bob@example.com", Age: 25},
	{ID: 3, Username: "charlie", Email: "charlie@example.com", Age: 35},
	{ID: 4, Username: "diana", Email: "diana@example.com", Age: 28},
	{ID: 5, Username: "eve", Email: "eve@example.com", Age: 22},
}

func BenchmarkZen_ListUsers(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/users", func(c *zen.Context) { c.JSON(200, userList) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.Handler.ServeHTTP(w, req)
	}
}

func BenchmarkGin_ListUsers(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/users", func(c *gin.Context) { c.JSON(200, userList) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_ListUsers(b *testing.B) {
	e := echo.New()
	e.GET("/users", func(c echo.Context) error { return c.JSON(200, userList) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(w, req)
	}
}

func BenchmarkStdLib_ListUsers(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userList)
	})
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(w, req)
	}
}

// ─── 7. Query Params ──────────────────────────────────────────────────────────

func BenchmarkZen_QueryParams(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/search", func(c *zen.Context) {
		page := c.QueryParam("page")
		limit := c.QueryParam("limit")
		search := c.QueryParam("search")
		c.JSON(200, map[string]string{"page": page, "limit": limit, "search": search})
	})
	req := httptest.NewRequest(http.MethodGet, "/search?page=1&limit=20&search=alice", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.Handler.ServeHTTP(w, req)
	}
}

func BenchmarkGin_QueryParams(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/search", func(c *gin.Context) {
		c.JSON(200, gin.H{"page": c.Query("page"), "limit": c.Query("limit"), "search": c.Query("search")})
	})
	req := httptest.NewRequest(http.MethodGet, "/search?page=1&limit=20&search=alice", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_QueryParams(b *testing.B) {
	e := echo.New()
	e.GET("/search", func(c echo.Context) error {
		return c.JSON(200, map[string]string{
			"page": c.QueryParam("page"), "limit": c.QueryParam("limit"), "search": c.QueryParam("search"),
		})
	})
	req := httptest.NewRequest(http.MethodGet, "/search?page=1&limit=20&search=alice", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(w, req)
	}
}

func BenchmarkStdLib_QueryParams(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"page": q.Get("page"), "limit": q.Get("limit"), "search": q.Get("search"),
		})
	})
	req := httptest.NewRequest(http.MethodGet, "/search?page=1&limit=20&search=alice", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(w, req)
	}
}

// ─── 8. Validation Error Path ─────────────────────────────────────────────────

var badUserBody = []byte(`{"username":"a","email":"not-an-email","age":-1}`)

func BenchmarkZen_ValidationError(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/users", func(c *zen.Context) {
		var u User
		if err := c.BindJSON(&u); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(201, u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(badUserBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.Handler.ServeHTTP(w, req)
	}
}

func BenchmarkGin_ValidationError(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/users", func(c *gin.Context) {
		var u User
		if err := c.ShouldBindJSON(&u); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := binding.Validator.ValidateStruct(&u); err != nil {
			c.JSON(422, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(badUserBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_ValidationError(b *testing.B) {
	e := echo.New()
	e.Validator = &echoValidator{v: validator.New()}
	e.POST("/users", func(c echo.Context) error {
		var u User
		if err := c.Bind(&u); err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		if err := c.Validate(&u); err != nil {
			return c.JSON(422, map[string]string{"error": err.Error()})
		}
		return c.JSON(201, u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(badUserBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkStdLib_ValidationError(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(badUserBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}
}

// ─── 9. Parallel Throughput ───────────────────────────────────────────────────

func BenchmarkZen_Parallel(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/users/1", func(c *zen.Context) {
		c.JSON(200, User{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30})
	})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
			w := httptest.NewRecorder()
			mux.Handler.ServeHTTP(w, req)
		}
	})
}

func BenchmarkGin_Parallel(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/users/:id", func(c *gin.Context) {
		c.JSON(200, User{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30})
	})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkEcho_Parallel(b *testing.B) {
	e := echo.New()
	e.GET("/users/:id", func(c echo.Context) error {
		return c.JSON(200, User{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30})
	})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)
		}
	})
}

func BenchmarkStdLib_Parallel(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/1", func(w http.ResponseWriter, r *http.Request) {
		u := User{ID: 1, Username: "alice", Email: "alice@example.com", Age: 30}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
		}
	})
}

// ─── 10. Large JSON Payload ───────────────────────────────────────────────────

func makeLargeUserList(n int) []User {
	users := make([]User, n)
	for i := range users {
		users[i] = User{
			ID:       i + 1,
			Username: "user" + strings.Repeat("x", i%10),
			Email:    "user@example.com",
			Age:      20 + (i % 50),
		}
	}
	return users
}

var largeUserList = makeLargeUserList(100)

func BenchmarkZen_LargeJSONResponse(b *testing.B) {
	mux := zen.NewServer(":8080")
	mux.Handle("/users", func(c *zen.Context) { c.JSON(200, largeUserList) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.Handler.ServeHTTP(w, req)
	}
}

func BenchmarkGin_LargeJSONResponse(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/users", func(c *gin.Context) { c.JSON(200, largeUserList) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_LargeJSONResponse(b *testing.B) {
	e := echo.New()
	e.GET("/users", func(c echo.Context) error { return c.JSON(200, largeUserList) })
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ServeHTTP(w, req)
	}
}

func BenchmarkStdLib_LargeJSONResponse(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(largeUserList)
	})
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(w, req)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
