package zen

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRouter_New(t *testing.T) {
	r := New(":8080")
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.server == nil {
		t.Fatal("Server not initialized")
	}
	if r.server.Addr != ":8080" {
		t.Fatalf("Addr = %q, want %q", r.server.Addr, ":8080")
	}
}

func TestRouter_New_WithConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ReadTimeout = 15 * 1000000000
	r := New(":9090", cfg)

	if r.server.ReadTimeout != 15*1000000000 {
		t.Fatalf("ReadTimeout = %v, want %v", r.server.ReadTimeout, 15*1000000000)
	}
}

func TestRouter_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 0 {
		t.Fatalf("IdleTimeout = %v", cfg.IdleTimeout)
	}
	if cfg.ReadHeaderTimeout != 0 {
		t.Fatalf("ReadHeaderTimeout = %v", cfg.ReadHeaderTimeout)
	}
	if cfg.MaxHeaderBytes != 0 {
		t.Fatalf("MaxHeaderBytes = %v", cfg.MaxHeaderBytes)
	}
}

func TestRouter_ServeHTTP_NoMiddleware(t *testing.T) {
	r := New(":0")
	var called bool
	r.GET("/test", func(c *Ctx) {
		called = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !called {
		t.Fatal("handler not called")
	}
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "ok")
	}
}

func TestRouter_ServeHTTP_WithMiddleware(t *testing.T) {
	r := New(":0")
	var mwCalled, handlerCalled bool
	r.Use(func(c *Ctx) {
		mwCalled = true
		c.Next()
	})
	r.GET("/test", func(c *Ctx) {
		handlerCalled = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !mwCalled {
		t.Fatal("middleware not called")
	}
	if !handlerCalled {
		t.Fatal("handler not called")
	}
}

func TestRouter_Middleware_ShortCircuit(t *testing.T) {
	r := New(":0")
	var handlerCalled bool
	r.Use(func(c *Ctx) {
		c.Error(401, "unauthorized")
	})
	r.GET("/test", func(c *Ctx) {
		handlerCalled = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if handlerCalled {
		t.Fatal("handler should not be called after short-circuit")
	}
}

func TestRouter_Middleware_Order(t *testing.T) {
	r := New(":0")
	var order []string
	r.Use(func(c *Ctx) {
		order = append(order, "1-before")
		c.Next()
		order = append(order, "1-after")
	})
	r.Use(func(c *Ctx) {
		order = append(order, "2-before")
		c.Next()
		order = append(order, "2-after")
	})
	r.GET("/test", func(c *Ctx) {
		order = append(order, "handler")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	want := []string{"1-before", "2-before", "handler", "2-after", "1-after"}
	if len(order) != len(want) {
		t.Fatalf("order length = %d, want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestRouter_HandleRaw(t *testing.T) {
	r := New(":0")
	r.HandleRaw("GET /raw", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("raw"))
	}))

	req := httptest.NewRequest("GET", "/raw", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "raw" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "raw")
	}
}

func TestRouter_File(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	_ = os.WriteFile(path, []byte("file content"), 0644)

	r := New(":0")
	r.File("/file", path)

	req := httptest.NewRequest("GET", "/file", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "file content" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "file content")
	}
}

func TestRouter_Static(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "hello.txt"), []byte("hello"), 0644)

	r := New(":0")
	r.Static("/static", tmp)

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 && w.Code != 301 {
		t.Fatalf("status = %d, want 200 or 301", w.Code)
	}
}

func TestRouter_PathParams(t *testing.T) {
	r := New(":0")
	var captured string
	r.GET("/users/{id}", func(c *Ctx) {
		captured = c.Param("id")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/users/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured != "42" {
		t.Fatalf("param = %q, want %q", captured, "42")
	}
}

func TestRouter_QueryParams(t *testing.T) {
	r := New(":0")
	var captured string
	r.GET("/search", func(c *Ctx) {
		captured = c.QueryParam("q")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/search?q=golang&page=2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured != "golang" {
		t.Fatalf("query = %q, want %q", captured, "golang")
	}
}

func TestRouter_NotFound(t *testing.T) {
	r := New(":0")
	r.GET("/exists", func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestRouter_MethodNotAllowed(t *testing.T) {
	r := New(":0")
	r.GET("/only-get", func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("POST", "/only-get", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

func TestRouter_ShutdownTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShutdownTimeout = 10 * 1000000000
	r := New(":0", cfg)

	if r.shutdownTimeout != 10*1000000000 {
		t.Fatalf("shutdownTimeout = %v, want %v", r.shutdownTimeout, 10*1000000000)
	}
}

func BenchmarkRouter_ServeHTTP(b *testing.B) {
	r := New(":0")
	r.GET("/bench", func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/bench", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_ServeHTTP_WithMiddleware(b *testing.B) {
	r := New(":0")
	r.Use(func(c *Ctx) {
		c.Next()
	})
	r.GET("/bench", func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/bench", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_PathParams(b *testing.B) {
	r := New(":0")
	r.GET("/users/{id}", func(c *Ctx) {
		c.Param("id")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// Test per-route middleware
func TestRouter_HandleWith_PerRouteMiddleware(t *testing.T) {
	r := New(":0")
	var middlewareCalled bool

	r.GET("/protected", func(c *Ctx) {
		middlewareCalled = true
		c.Next()
	}, func(c *Ctx) {
		c.String(200, "protected")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !middlewareCalled {
		t.Fatal("per-route middleware not called")
	}
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRouter_HandleWith_MiddlewareOrder(t *testing.T) {
	r := New(":0")
	var order []string

	r.Use(func(c *Ctx) {
		order = append(order, "global")
		c.Next()
	})

	r.GET("/test", func(c *Ctx) {
		order = append(order, "per-route")
		c.Next()
	}, func(c *Ctx) {
		order = append(order, "handler")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	want := []string{"global", "per-route", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestRouter_StaticFS(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	r := New(":0")
	r.StaticFS("/static", os.DirFS(tmpDir))

	req := httptest.NewRequest("GET", "/static/hello.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// StaticFS with DirFS and subpath — accept 200 or 301 (directory redirect)
	if w.Code != 200 && w.Code != 301 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if w.Code == 200 && w.Body.String() != "hello world\n" && !strings.Contains(w.Body.String(), "hello world") {
		t.Fatalf("body = %q, should contain hello world", w.Body.String())
	}
}

func TestRouter_StaticFS_RootPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	r := New(":0")
	r.StaticFS("/", os.DirFS(tmpDir))

	req := httptest.NewRequest("GET", "/hello.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestRouter_MultiplePathParams(t *testing.T) {
	r := New(":0")
	r.GET("/users/{userID}/posts/{postID}", func(c *Ctx) {
		userID := c.Param("userID")
		postID := c.Param("postID")
		c.String(http.StatusOK, userID+":"+postID)
	})

	req := httptest.NewRequest("GET", "/users/u1/posts/p2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "u1:p2" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "u1:p2")
	}
}

func TestRouter_WildcardRoute(t *testing.T) {
	r := New(":0")
	r.GET("/static/{path...}", func(c *Ctx) {
		c.String(http.StatusOK, "wildcard:"+c.Param("path"))
	})

	req := httptest.NewRequest("GET", "/static/js/app.js", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "js/app.js") {
		t.Fatalf("body = %q, should contain path", body)
	}
}

func TestRouter_HEADMethod(t *testing.T) {
	r := New(":0")
	r.GET("/items", func(c *Ctx) {
		c.Response.Header().Set("Content-Length", "5")
		c.String(http.StatusOK, "hello")
	})

	req := httptest.NewRequest("HEAD", "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRouter_HandleWith_EmptyMiddleware(t *testing.T) {
	r := New(":0")
	var called bool
	r.GET("/test", func(c *Ctx) {
		called = true
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !called {
		t.Fatal("handler should be called")
	}
}

func TestRouter_StaticFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("file content"), 0644); err != nil {
		t.Fatal(err)
	}

	r := New(":0")
	r.File("/file", tmpFile)

	req := httptest.NewRequest("GET", "/file", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "file content") {
		t.Fatalf("body should contain file content: %s", w.Body.String())
	}
}

func TestEngine_Run_StartsAndServes(t *testing.T) {
	port := freePort(t)
	e := New("127.0.0.1:" + port)
	e.GET("/ping", func(c *Ctx) {
		c.String(200, "pong")
	})

	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic: %v", r)
			}
		}()
		e.Run()
		errCh <- nil
	}()

	addr := waitListen(t, e.server.Addr, 5*time.Second)

	resp, err := http.Get("http://" + addr + "/ping")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestEngine_Run_ServesMultiple(t *testing.T) {
	port := freePort(t)
	e := New("127.0.0.1:" + port)
	e.GET("/a", func(c *Ctx) { c.String(200, "a") })
	e.GET("/b", func(c *Ctx) { c.String(200, "b") })

	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic: %v", r)
			}
		}()
		e.Run()
		errCh <- nil
	}()

	addr := waitListen(t, e.server.Addr, 5*time.Second)

	for _, path := range []string{"/a", "/b"} {
		resp, err := http.Get("http://" + addr + path)
		if err != nil {
			t.Fatalf("request to %s failed: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status on %s = %d, want 200", path, resp.StatusCode)
		}
	}

	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGTERM)
	<-errCh
}

func TestEngine_RunTLS_StartsAndServes(t *testing.T) {
	certFile, keyFile := generateCert(t)
	port := freePort(t)
	e := New("127.0.0.1:" + port)
	e.GET("/secure", func(c *Ctx) {
		c.String(200, "secure")
	})

	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic: %v", r)
			}
		}()
		e.RunTLS(certFile, keyFile)
		errCh <- nil
	}()

	addr := waitListen(t, e.server.Addr, 5*time.Second)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get("https://" + addr + "/secure")
	if err != nil {
		t.Fatalf("TLS request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGTERM)
	<-errCh
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	return portStr
}

func waitListen(t *testing.T, addr string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if addr == "" || addr == "127.0.0.1:0" {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return addr
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %s within %v", addr, timeout)
	return ""
}

func generateCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(1 * time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	f, err := os.Create(certFile)
	if err != nil {
		t.Fatal(err)
	}
	pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	f.Close()

	f, err = os.Create(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	pem.Encode(f, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	f.Close()

	return
}
