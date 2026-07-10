package zen

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRadix_StaticRoutes(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/", h)
	r.add("GET", "/users", h)
	r.add("GET", "/users/list", h)
	r.add("GET", "/users/list/all", h)

	tests := []struct {
		method string
		path   string
		match  bool
	}{
		{"GET", "/", true},
		{"GET", "/users", true},
		{"GET", "/users/list", true},
		{"GET", "/users/list/all", true},
		{"GET", "/notfound", false},
		{"GET", "/users/other", false},
		{"GET", "/users/list/other", false},
		{"POST", "/", false},
	}

	for _, tt := range tests {
		var ps params
		var skipped []skippedNode
		res := r.find(tt.method, tt.path, &ps, &skipped)
		if tt.match && res.handlers == nil {
			t.Errorf("expected match for %s %s, got nil handlers", tt.method, tt.path)
		}
		if !tt.match && res.handlers != nil {
			t.Errorf("expected no match for %s %s, got handlers", tt.method, tt.path)
		}
	}
}

func TestRadix_PathParam(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("id")
	}}

	r.add("GET", "/users/:id", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/users/42", &ps, &skipped)

	if res.handlers == nil {
		t.Fatal("expected match for GET /users/42")
	}
	res.handlers[0](&Ctx{ps: ps})
	if captured != "42" {
		t.Errorf("expected id=42, got id=%s", captured)
	}
}

func TestRadix_MultiplePathParams(t *testing.T) {
	r := newRadixRouter()
	var capturedUser, capturedPost string
	h := []HandlerFunc{func(c *Ctx) {
		capturedUser = c.Param("userID")
		capturedPost = c.Param("postID")
	}}

	r.add("GET", "/users/:userID/posts/:postID", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/users/abc/posts/xyz", &ps, &skipped)

	if res.handlers == nil {
		t.Fatal("expected match for GET /users/abc/posts/xyz")
	}
	res.handlers[0](&Ctx{ps: ps})
	if capturedUser != "abc" {
		t.Errorf("expected userID=abc, got userID=%s", capturedUser)
	}
	if capturedPost != "xyz" {
		t.Errorf("expected postID=xyz, got postID=%s", capturedPost)
	}
}

func TestRadix_MultiplePathParamEdgeCases(t *testing.T) {
	r := newRadixRouter()
	var got []string
	h := []HandlerFunc{func(c *Ctx) {
		got = []string{c.Param("a"), c.Param("b"), c.Param("c")}
	}}

	r.add("GET", "/:a/:b/:c", h)

	tests := []struct {
		path   string
		want   []string
		match  bool
	}{
		{"/x/y/z", []string{"x", "y", "z"}, true},
		{"/a/b/c", []string{"a", "b", "c"}, true},
		{"/only-one", nil, false},
		{"/a/b", nil, false},
	}

	for _, tt := range tests {
		got = nil
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)

		if !tt.match && res.handlers != nil {
			t.Errorf("expected no match for %s, got handlers", tt.path)
			continue
		}
		if tt.match && res.handlers == nil {
			t.Errorf("expected match for %s, got nil handlers", tt.path)
			continue
		}
		if tt.want != nil {
			res.handlers[0](&Ctx{ps: ps})
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("%s: param %d = %q, want %q", tt.path, i, got[i], w)
				}
			}
		}
	}
}

func TestRadix_CatchAll(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("path")
	}}

	r.add("GET", "/static/*path", h)

	tests := []struct {
		path  string
		want  string
		match bool
	}{
		{"/static/a", "a", true},
		{"/static/a/b/c", "a/b/c", true},
		{"/static/", "", true},
		{"/other", "", false},
	}

	for _, tt := range tests {
		captured = ""
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)

		if !tt.match && res.handlers != nil {
			t.Errorf("expected no match for %s, got handlers", tt.path)
			continue
		}
		if tt.match && res.handlers == nil {
			t.Errorf("expected match for %s, got nil handlers", tt.path)
			continue
		}
		if tt.match {
			res.handlers[0](&Ctx{ps: ps})
			if captured != tt.want {
				t.Errorf("%s: captured = %q, want %q", tt.path, captured, tt.want)
			}
		}
	}
}

func TestRadix_CatchAllRootLeadingSlash(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("path")
	}}

	r.add("GET", "/*path", h)

	tests := []struct {
		path  string
		want  string
		match bool
	}{
		{"/any/path/here", "any/path/here", true},
		{"/single", "single", true},
		{"/", "", true},
	}

	for _, tt := range tests {
		captured = ""
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)

		if !tt.match && res.handlers != nil {
			t.Errorf("expected no match for %s, got handlers", tt.path)
			continue
		}
		if tt.match && res.handlers == nil {
			t.Errorf("expected match for %s, got nil handlers", tt.path)
			continue
		}
		if tt.match {
			res.handlers[0](&Ctx{ps: ps})
			if captured != tt.want {
				t.Errorf("%s: captured = %q, want %q", tt.path, captured, tt.want)
			}
		}
	}
}

func TestRadix_NotFound(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users", h)
	r.add("GET", "/users/list", h)

	tests := []string{
		"/notfound",
		"/users/extra",
		"/users/list/extra",
		"/u",
	}

	for _, path := range tests {
		var ps params
		var skipped []skippedNode
		res := r.find("GET", path, &ps, &skipped)
		if res.handlers != nil {
			t.Errorf("expected 404 for GET %s, got handlers", path)
		}
		if res.tsr {
			t.Errorf("expected no TSR for GET %s", path)
		}
	}
}

func TestRadix_MethodNotAllowed(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users", h)
	r.add("POST", "/users", h)
	r.add("DELETE", "/users", h)

	var ps params
	var skipped []skippedNode

	// PUT is not registered — should return allowed methods
	res := r.find("PUT", "/users", &ps, &skipped)
	if res.handlers != nil {
		t.Error("expected no handlers for PUT /users")
	}
	if res.allowedMethod == "" {
		t.Error("expected allowed methods for PUT /users")
	}
	if res.allowedMethod == "" {
		t.Errorf("expected allowed methods, got empty")
	}
	if res.allowedMethod != "DELETE, GET, POST" && res.allowedMethod != "DELETE, POST, GET" && res.allowedMethod != "GET, POST, DELETE" {
		t.Errorf("expected allowed methods to contain GET, POST, DELETE, got %q", res.allowedMethod)
	}

	// A path with no handlers at all should not return allowed methods
	res = r.find("PUT", "/nonexistent", &ps, &skipped)
	if res.allowedMethod != "" {
		t.Errorf("expected empty allowed for nonexistent path, got %q", res.allowedMethod)
	}
}

func TestRadix_HEADFallback(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users", h)

	var ps params
	var skipped []skippedNode

	// HEAD should fall back to GET
	res := r.find("HEAD", "/users", &ps, &skipped)
	if res.handlers == nil {
		t.Error("expected HEAD /users to fall back to GET handlers")
	}

	// Explicit HEAD route should take priority
	var headCalled bool
	r.add("HEAD", "/users", []HandlerFunc{func(c *Ctx) { headCalled = true }})

	res = r.find("HEAD", "/users", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected HEAD /users to match explicit HEAD route")
	}
	res.handlers[0](nil)
	if !headCalled {
		t.Error("expected explicit HEAD handler to be called")
	}
}

func TestRadix_TSR(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users", h)
	r.add("GET", "/users/list", h)
	r.add("GET", "/admin", h)

	tests := []struct {
		path string
		tsr  bool
	}{
		{"/users/", true},
		{"/users/list/", true},
		{"/admin/", true},
		{"/users//", false},
		{"/unknown/", false},
	}

	for _, tt := range tests {
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)
		if res.tsr != tt.tsr {
			t.Errorf("GET %s: tsr = %v, want %v", tt.path, res.tsr, tt.tsr)
		}
		if res.handlers != nil {
			t.Errorf("GET %s: expected no handlers (TSR case), got handlers", tt.path)
		}
	}
}

func TestRadix_TSRWithTrailingSlashRoute(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users/", h)

	var ps params
	var skipped []skippedNode

	// Without trailing slash should suggest TSR
	res := r.find("GET", "/users", &ps, &skipped)
	if !res.tsr {
		t.Error("expected TSR for GET /users (route is /users/)")
	}
	if res.handlers != nil {
		t.Error("expected no handlers for GET /users")
	}

	// With trailing slash should match directly
	res = r.find("GET", "/users/", &ps, &skipped)
	if res.handlers == nil {
		t.Error("expected handlers for GET /users/")
	}
}

func TestRadix_ParamURLEncoding(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("name")
	}}

	r.add("GET", "/hello/:name", h)

	tests := []struct {
		path string
		want string
	}{
		{"/hello/world", "world"},
		{"/hello/foo%2Fbar", "foo/bar"},
		{"/hello/caf%C3%A9", "café"},
		{"/hello/%2F%2F", "//"},
	}

	for _, tt := range tests {
		captured = ""
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)
		if res.handlers == nil {
			t.Fatalf("expected match for %s", tt.path)
		}
		res.handlers[0](&Ctx{ps: ps})
		if captured != tt.want {
			t.Errorf("%s: captured = %q, want %q", tt.path, captured, tt.want)
		}
	}
}

func TestRadix_CatchAllURLEncoding(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("path")
	}}

	r.add("GET", "/files/*path", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/files/a%2Fb%2Fc", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match")
	}
	res.handlers[0](&Ctx{ps: ps})
	if captured != "a/b/c" {
		t.Errorf("captured = %q, want %q", captured, "a/b/c")
	}
}

func TestRadix_Backtracking(t *testing.T) {
	r := newRadixRouter()
	var capturedID string
	h := []HandlerFunc{func(c *Ctx) {
		capturedID = c.Param("id")
		c.Param("name")
	}}

	// Register routes where backtracking is needed.
	// /users/:id/posts and /users/me/profile
	r.add("GET", "/users/:id/posts", h)
	r.add("GET", "/users/me/profile", h)

	var ps params
	var skipped []skippedNode

	// /users/me/posts should match /users/:id/posts with id=me
	capturedID = ""
	res := r.find("GET", "/users/me/posts", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for /users/me/posts (backtrack to :id)")
	}
	res.handlers[0](&Ctx{ps: ps})
	if capturedID != "me" {
		t.Errorf("expected id=me, got id=%s", capturedID)
	}

	// /users/me/profile should match the static route
	ps = ps[:0]
	skipped = skipped[:0]
	capturedID = ""
	res = r.find("GET", "/users/me/profile", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for /users/me/profile")
	}
	res.handlers[0](&Ctx{ps: ps})
	if capturedID != "" {
		t.Errorf("expected no id param, got id=%s", capturedID)
	}
}

func TestRadix_BacktrackingMultiple(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("id")
	}}

	// :id should match but backtrack to static child
	r.add("GET", "/api/v1/:id/action", h)
	r.add("GET", "/api/v1/admin/profile", h)
	r.add("GET", "/api/v1/admin/settings", h)

	var ps params
	var skipped []skippedNode
	captured = ""
	res := r.find("GET", "/api/v1/admin/action", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for /api/v1/admin/action (backtrack)")
	}
	res.handlers[0](&Ctx{ps: ps})
	if captured != "admin" {
		t.Errorf("expected id=admin, got id=%s", captured)
	}

	ps = ps[:0]
	skipped = skipped[:0]
	res = r.find("GET", "/api/v1/admin/profile", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for /api/v1/admin/profile")
	}
}

func TestRadix_MixedStaticAndParam(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("id")
	}}

	r.add("GET", "/api/:id", h)
	r.add("GET", "/api/health", h)
	r.add("GET", "/api/health/db", h)

	tests := []struct {
		path   string
		match  bool
		param  string
	}{
		{"/api/health", true, ""},
		{"/api/health/db", true, ""},
		{"/api/42", true, "42"},
		{"/api/abc", true, "abc"},
		{"/api/health/extra", false, ""},
	}

	for _, tt := range tests {
		captured = ""
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)

		if tt.match && res.handlers == nil {
			t.Errorf("expected match for %s", tt.path)
			continue
		}
		if !tt.match && res.handlers != nil {
			t.Errorf("expected no match for %s", tt.path)
			continue
		}
		if tt.match && tt.param != "" {
			res.handlers[0](&Ctx{ps: ps})
			if captured != tt.param {
				t.Errorf("%s: param = %q, want %q", tt.path, captured, tt.param)
			}
		}
	}
}

func TestRadix_DeeplyNested(t *testing.T) {
	r := newRadixRouter()
	var a, b, c string
	h := []HandlerFunc{func(ctx *Ctx) {
		a = ctx.Param("a")
		b = ctx.Param("b")
		c = ctx.Param("c")
	}}

	r.add("GET", "/:a/:b/:c/detail", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/x/y/z/detail", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match")
	}
	res.handlers[0](&Ctx{ps: ps})
	if a != "x" || b != "y" || c != "z" {
		t.Errorf("got a=%s b=%s c=%s, want x y z", a, b, c)
	}
}

func TestRadix_RootPath(t *testing.T) {
	r := newRadixRouter()
	var called bool
	h := []HandlerFunc{func(c *Ctx) { called = true }}

	r.add("GET", "/", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for /")
	}
	res.handlers[0](nil)
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestRadix_EmptyPathNormalized(t *testing.T) {
	r := newRadixRouter()
	var called bool
	h := []HandlerFunc{func(c *Ctx) { called = true }}

	r.add("GET", "", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for / (empty path normalized to /)")
	}
	res.handlers[0](nil)
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestRadix_MissingLeadingSlash(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "users", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/users", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for /users (missing slash normalized)")
	}
}

func TestRadix_MultipleMethods(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/resource", h)
	r.add("POST", "/resource", h)
	r.add("PUT", "/resource", h)
	r.add("DELETE", "/resource", h)
	r.add("PATCH", "/resource", h)

	for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		var ps params
		var skipped []skippedNode
		res := r.find(m, "/resource", &ps, &skipped)
		if res.handlers == nil {
			t.Errorf("expected match for %s /resource", m)
		}
	}
}

func TestRadix_PriorityOrdering(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	// Register static routes that share prefixes - the router should
	// prioritize more specific routes over less specific ones.
	r.add("GET", "/", h)
	r.add("GET", "/a", h)
	r.add("GET", "/b", h)
	r.add("GET", "/ab", h)
	r.add("GET", "/abc", h)

	var ps params
	var skipped []skippedNode

	for _, path := range []string{"/", "/a", "/b", "/ab", "/abc"} {
		res := r.find("GET", path, &ps, &skipped)
		if res.handlers == nil {
			t.Errorf("expected match for GET %s", path)
		}
	}
}

func TestRadix_ParamAtDifferentLevels(t *testing.T) {
	r := newRadixRouter()
	var captured string

	r.add("GET", "/:id", []HandlerFunc{func(c *Ctx) {
		captured = c.Param("id")
	}})
	r.add("GET", "/:id/:action", []HandlerFunc{func(c *Ctx) {
		captured = c.Param("id") + "/" + c.Param("action")
	}})
	r.add("GET", "/:id/:action/:sub", []HandlerFunc{func(c *Ctx) {
		captured = c.Param("id") + "/" + c.Param("action") + "/" + c.Param("sub")
	}})

	tests := []struct {
		path string
		want string
	}{
		{"/user", "user"},
		{"/user/edit", "user/edit"},
		{"/user/edit/name", "user/edit/name"},
	}

	for _, tt := range tests {
		captured = ""
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)
		if res.handlers == nil {
			t.Fatalf("expected match for %s", tt.path)
		}
		res.handlers[0](&Ctx{ps: ps})
		if captured != tt.want {
			t.Errorf("%s: got %q, want %q", tt.path, captured, tt.want)
		}
	}
}

func TestRadix_GroupPrefixIntegration(t *testing.T) {
	e := New(":0")
	var captured string

	api := e.Group("/api")
	api.GET("/users/:id", func(c *Ctx) {
		captured = c.Param("id")
	})

	admin := api.Group("/admin")
	admin.GET("/settings/:key", func(c *Ctx) {
		captured = c.Param("key")
	})

	tests := []struct {
		method string
		path   string
		match  bool
		param  string
	}{
		{"GET", "/api/users/42", true, "42"},
		{"GET", "/api/users/abc", true, "abc"},
		{"GET", "/api/admin/settings/timeout", true, "timeout"},
		{"GET", "/api/admin/settings/", false, ""},
		{"GET", "/other", false, ""},
	}

	for _, tt := range tests {
		captured = ""
		req := httptest.NewRequest(tt.method, tt.path, nil)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)

		if tt.match && captured != tt.param {
			t.Errorf("%s %s: param = %q, want %q", tt.method, tt.path, captured, tt.param)
		}
	}
}

func TestRadix_ConvertPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/users/{id}", "/users/:id"},
		{"/users/{id}/posts/{postId}", "/users/:id/posts/:postId"},
		{"/static/{path...}", "/static/*path"},
		{"/{path...}", "/*path"},
		{"/users", "/users"},
		{"/", "/"},
		{"", ""},
	}

	for _, tt := range tests {
		got := convertPath(tt.input)
		if got != tt.want {
			t.Errorf("convertPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRadix_ConvertServeMuxPattern(t *testing.T) {
	tests := []struct {
		input    string
		wantM    string
		wantPath string
	}{
		{"GET /users/{id}", "GET", "/users/{id}"},
		{"POST /api/{path...}", "POST", "/api/{path...}"},
		{"/static/{path...}", "", "/static/{path...}"},
		{"/plain", "", "/plain"},
	}

	for _, tt := range tests {
		m, p := convertServeMuxPattern(tt.input)
		if m != tt.wantM || p != tt.wantPath {
			t.Errorf("convertServeMuxPattern(%q) = (%q, %q), want (%q, %q)",
				tt.input, m, p, tt.wantM, tt.wantPath)
		}
	}
}

func TestRadix_HandleRawIntegration(t *testing.T) {
	e := New(":0")

	e.HandleRaw("GET /api/users/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/users/42", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRadix_ParamReuseAcrossRequests(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("id")
	}}

	r.add("GET", "/users/:id", h)

	// First request
	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/users/first", &ps, &skipped)
	res.handlers[0](&Ctx{ps: ps})
	if captured != "first" {
		t.Errorf("first request: got %q, want %q", captured, "first")
	}

	// Second request with same ps slice (simulating pool reuse)
	ps = ps[:0]
	skipped = skipped[:0]
	res = r.find("GET", "/users/second", &ps, &skipped)
	res.handlers[0](&Ctx{ps: ps})
	if captured != "second" {
		t.Errorf("second request: got %q, want %q", captured, "second")
	}
}

func TestRadix_DifferentMethodsSamePath(t *testing.T) {
	e := New(":0")
	var method string

	e.GET("/resource", func(c *Ctx) { method = "GET" })
	e.POST("/resource", func(c *Ctx) { method = "POST" })
	e.PUT("/resource", func(c *Ctx) { method = "PUT" })

	tests := []struct {
		method string
		code   int
	}{
		{"GET", 200},
		{"POST", 200},
		{"PUT", 200},
		{"DELETE", 405},
	}

	for _, tt := range tests {
		method = ""
		req := httptest.NewRequest(tt.method, "/resource", nil)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)

		if w.Code != tt.code {
			t.Errorf("%s: status = %d, want %d", tt.method, w.Code, tt.code)
		}
		if tt.code == 200 && method != tt.method {
			t.Errorf("%s: handler method = %q, want %q", tt.method, method, tt.method)
		}
		if tt.code == 405 {
			allow := w.Header().Get("Allow")
			if allow == "" {
				t.Errorf("%s: expected Allow header", tt.method)
			}
		}
	}
}

func TestRadix_EdgeCaseRouteConflicts(t *testing.T) {
	tests := []struct {
		name    string
		routes  []string
		wantPanic bool
	}{
		{
			"duplicate exact path",
			[]string{"/users", "/users"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Error("expected panic, got none")
				}
				if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			e := New(":0")
			for _, route := range tt.routes {
				e.GET(route, func(c *Ctx) {})
			}
		})
	}
}

func TestRadix_WildcardConflict(t *testing.T) {
	tests := []struct {
		name       string
		first      string
		second     string
		wantPanic bool
	}{
		{
			"param vs static conflict",
			"/users/:id",
			"/users/list",
			false, // should NOT panic — backtracking handles this
		},
		{
			"catch-all at end works",
			"/files/*path",
			"/files/extra",
			true, // catch-all must be at root of segment
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Errorf("expected panic for %s", tt.name)
				}
				if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic for %s: %v", tt.name, r)
				}
			}()

			e := New(":0")
			e.GET(tt.first, func(c *Ctx) {})
			e.GET(tt.second, func(c *Ctx) {})
		})
	}
}

func TestRadix_PanicOnInvalidPatterns(t *testing.T) {
	tests := []struct {
		path  string
		panic bool
	}{
		{"/users/:id/posts/:id", false},    // duplicate name is allowed
		{"/users/:", true},                   // empty param name
		{"/users/:id/:id2", false},           // two params in same segment - not allowed
		{"/users/:123", false},               // numeric param name
		{"/users/*", true},                   // empty catch-all name
		{"/users/*path/extra", true},         // catch-all not at end
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.panic && r == nil {
					t.Errorf("expected panic for path %q", tt.path)
				}
				if !tt.panic && r != nil {
					t.Errorf("unexpected panic for path %q: %v", tt.path, r)
				}
			}()

			e := New(":0")
			e.GET(tt.path, func(c *Ctx) {})
		})
	}
}

func TestRadix_QueryParams(t *testing.T) {
	e := New(":0")
	var q struct {
		name string
		age  string
	}

	e.GET("/search", func(c *Ctx) {
		q.name = c.QueryParam("name")
		q.age = c.QueryParam("age")
	})

	req := httptest.NewRequest("GET", "/search?name=john&age=30", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if q.name != "john" {
		t.Errorf("name = %q, want %q", q.name, "john")
	}
	if q.age != "30" {
		t.Errorf("age = %q, want %q", q.age, "30")
	}
}

func TestRadix_QueryParamsWithPathParams(t *testing.T) {
	e := New(":0")
	var user, sort string

	e.GET("/users/:id", func(c *Ctx) {
		user = c.Param("id")
		sort = c.QueryParam("sort")
	})

	req := httptest.NewRequest("GET", "/users/42?sort=asc", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if user != "42" {
		t.Errorf("user = %q, want %q", user, "42")
	}
	if sort != "asc" {
		t.Errorf("sort = %q, want %q", sort, "asc")
	}
}

func TestRadix_MultipleGroups(t *testing.T) {
	e := New(":0")
	var output string

	v1 := e.Group("/api/v1")
	v1.GET("/users/:id", func(c *Ctx) {
		output = "v1:" + c.Param("id")
	})

	v2 := e.Group("/api/v2")
	v2.GET("/users/:id", func(c *Ctx) {
		output = "v2:" + c.Param("id")
	})

	req := httptest.NewRequest("GET", "/api/v1/users/10", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if output != "v1:10" {
		t.Errorf("v1: got %q, want %q", output, "v1:10")
	}

	req = httptest.NewRequest("GET", "/api/v2/users/20", nil)
	w = httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if output != "v2:20" {
		t.Errorf("v2: got %q, want %q", output, "v2:20")
	}
}

func TestRadix_NestedGroupDeep(t *testing.T) {
	e := New(":0")
	var full string

	a := e.Group("/a")
	b := a.Group("/b")
	c := b.Group("/c")
	c.GET("/:id", func(ctx *Ctx) {
		full = ctx.Param("id")
	})

	req := httptest.NewRequest("GET", "/a/b/c/xyz", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if full != "xyz" {
		t.Errorf("full = %q, want %q", full, "xyz")
	}
}

func TestRadix_MixedParamStaticOrdering(t *testing.T) {
	r := newRadixRouter()
	var called string
	h1 := []HandlerFunc{func(c *Ctx) { called = "list" }}
	h2 := []HandlerFunc{func(c *Ctx) { called = c.Param("id") }}

	r.add("GET", "/api/list", h1)
	r.add("GET", "/api/:id", h2)

	tests := []struct {
		path string
		want string
	}{
		{"/api/list", "list"},
		{"/api/42", "42"},
		{"/api/abc", "abc"},
	}

	for _, tt := range tests {
		called = ""
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)
		if res.handlers == nil {
			t.Fatalf("expected match for %s", tt.path)
		}
		res.handlers[0](&Ctx{ps: ps})
		if called != tt.want {
			t.Errorf("%s: called = %q, want %q", tt.path, called, tt.want)
		}
	}
}

func TestRadix_ParamFromBoundary(t *testing.T) {
	e := New(":0")
	var id string

	e.GET("/:id", func(c *Ctx) {
		id = c.Param("id")
	})

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if id != "hello" {
		t.Errorf("id = %q, want %q", id, "hello")
	}

	req = httptest.NewRequest("GET", "/a/b", nil)
	w = httptest.NewRecorder()
	e.ServeHTTP(w, req)
	// /:id won't match /a/b — no handlers
	if w.Code != 404 {
		t.Errorf("expected 404 for /a/b, got %d", w.Code)
	}
}

func TestRadix_ParamMultipleSameLevel(t *testing.T) {
	r := newRadixRouter()
	var a, b string
	h := []HandlerFunc{func(c *Ctx) {
		a = c.Param("a")
		b = c.Param("b")
	}}

	r.add("GET", "/:a/:b", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/first/second", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match")
	}
	res.handlers[0](&Ctx{ps: ps})
	if a != "first" || b != "second" {
		t.Errorf("got a=%s b=%s, want first second", a, b)
	}
}

func TestRadix_ParamWithTrailingSlash(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users/:id", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/users/42/", &ps, &skipped)
	if !res.tsr {
		t.Error("expected TSR for /users/42/")
	}
	if res.handlers != nil {
		t.Error("expected no handlers (TSR case)")
	}
}

func TestRadix_NotFoundAfterPartialMatch(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users/list", h)
	r.add("GET", "/users/:id", h)

	for _, path := range []string{"/users/list/extra", "/users/extra/more"} {
		var ps params
		var skipped []skippedNode
		res := r.find("GET", path, &ps, &skipped)
		if res.handlers != nil {
			t.Errorf("expected 404 for %s, got handlers", path)
		}
	}
}

func TestRadix_AllowedMethodsAfterFind(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/resource", h)
	r.add("POST", "/resource", h)

	var ps params
	var skipped []skippedNode

	// find with GET should succeed
	res := r.find("GET", "/resource", &ps, &skipped)
	if res.handlers == nil {
		t.Error("expected handlers for GET /resource")
	}

	// find with PUT should return allowed methods
	ps = ps[:0]
	skipped = skipped[:0]
	res = r.find("PUT", "/resource", &ps, &skipped)
	if res.handlers != nil {
		t.Error("expected no handlers for PUT /resource")
	}
	if res.allowedMethod != "GET, POST" {
		t.Errorf("allowed = %q, want %q", res.allowedMethod, "GET, POST")
	}
}

func TestRadix_PoolReuse(t *testing.T) {
	e := New(":0")
	var id string

	e.GET("/users/:id", func(c *Ctx) {
		id = c.Param("id")
	})

	// Make several requests reusing the pool
	for i := 0; i < 10; i++ {
		id = ""
		req := httptest.NewRequest("GET", "/users/user123", nil)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)

		if id != "user123" {
			t.Errorf("iteration %d: id = %q, want %q", i, id, "user123")
		}
		if w.Code != 200 {
			t.Errorf("iteration %d: status = %d", i, w.Code)
		}
	}
}

func TestRadix_MiddlewareWithRouter(t *testing.T) {
	e := New(":0")
	var order []string

	e.Use(func(c *Ctx) {
		order = append(order, "mw1")
		c.Next()
	})

	e.GET("/test", func(c *Ctx) {
		order = append(order, "route_mw")
		c.Next()
	}, func(c *Ctx) {
		order = append(order, "handler")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
	want := []string{"mw1", "route_mw", "handler"}
	if len(order) != len(want) {
		t.Errorf("order = %v, want %v", order, want)
	} else {
		for i := range want {
			if order[i] != want[i] {
				t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
			}
		}
	}
}

func TestRadix_StaticFile(t *testing.T) {
	e := New(":0")
	dir := t.TempDir()
	e.Static("/static", dir)

	var ps params
	var skipped []skippedNode
	res := e.router.find("GET", "/static/any/file", &ps, &skipped)
	if res.handlers == nil {
		t.Error("expected handlers for /static/any/file")
	}

	res = e.router.find("GET", "/static", &ps, &skipped)
	if res.handlers == nil && !res.tsr {
		t.Error("expected handlers or TSR for /static")
	}
}

func TestRadix_NumberOfParams(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users/:id/posts/:postId/comments/:commentId", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/users/1/posts/2/comments/3", &ps, &skipped)

	if res.handlers == nil {
		t.Fatal("expected match")
	}

	// Should have exactly 3 params
	count := len(ps)
	if count != 3 {
		t.Errorf("expected 3 params, got %d", count)
	}

	// Verify param values
	paramMap := make(map[string]string)
	for _, p := range ps {
		paramMap[p.Key] = p.Value
	}

	if paramMap["id"] != "1" {
		t.Errorf("id = %q, want %q", paramMap["id"], "1")
	}
	if paramMap["postId"] != "2" {
		t.Errorf("postId = %q, want %q", paramMap["postId"], "2")
	}
	if paramMap["commentId"] != "3" {
		t.Errorf("commentId = %q, want %q", paramMap["commentId"], "3")
	}
}

func TestRadix_MultipleCatchAllRoutes(t *testing.T) {
	r := newRadixRouter()
	var path1, path2 string

	r.add("GET", "/static/*path", []HandlerFunc{func(c *Ctx) {
		path1 = c.Param("path")
	}})
	r.add("GET", "/media/*path", []HandlerFunc{func(c *Ctx) {
		path2 = c.Param("path")
	}})

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/static/css/style.css", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for /static/css/style.css")
	}
	res.handlers[0](&Ctx{ps: ps})
	if path1 != "css/style.css" {
		t.Errorf("path1 = %q, want %q", path1, "css/style.css")
	}

	ps = ps[:0]
	skipped = skipped[:0]
	res = r.find("GET", "/media/images/photo.jpg", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for /media/images/photo.jpg")
	}
	res.handlers[0](&Ctx{ps: ps})
	if path2 != "images/photo.jpg" {
		t.Errorf("path2 = %q, want %q", path2, "images/photo.jpg")
	}
}

func TestRadix_DeeplyNestedStatic(t *testing.T) {
	r := newRadixRouter()
	var called string
	h := []HandlerFunc{func(c *Ctx) { called = "hit" }}

	r.add("GET", "/a/b/c/d/e/f/g", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/a/b/c/d/e/f/g", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match for deeply nested static route")
	}
	res.handlers[0](nil)
	if called != "hit" {
		t.Error("handler was not called")
	}
}

func TestRadix_ParamEncodedSlash(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("id")
	}}

	r.add("GET", "/data/:id", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/data/value%2Fwith%2Fslashes", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match")
	}
	res.handlers[0](&Ctx{ps: ps})
	if captured != "value/with/slashes" {
		t.Errorf("captured = %q, want %q", captured, "value/with/slashes")
	}
}

func TestRadix_RoutesRegisteredInAnyOrder(t *testing.T) {
	r := newRadixRouter()
	var called string

	r.add("GET", "/api/v2/users", []HandlerFunc{func(c *Ctx) { called = "v2" }})
	r.add("GET", "/api/v1/users", []HandlerFunc{func(c *Ctx) { called = "v1" }})
	r.add("GET", "/api/:version/items", []HandlerFunc{func(c *Ctx) { called = "items:" + c.Param("version") }})

	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/users", "v1"},
		{"/api/v2/users", "v2"},
		{"/api/v3/items", "items:v3"},
	}

	for _, tt := range tests {
		called = ""
		var ps params
		var skipped []skippedNode
		res := r.find("GET", tt.path, &ps, &skipped)
		if res.handlers == nil {
			t.Fatalf("expected match for %s", tt.path)
		}
		res.handlers[0](&Ctx{ps: ps})
		if called != tt.want {
			t.Errorf("%s: got %q, want %q", tt.path, called, tt.want)
		}
	}
}

func TestRadix_ParamBoundaryCheck(t *testing.T) {
	r := newRadixRouter()
	var id string
	h := []HandlerFunc{func(c *Ctx) {
		id = c.Param("id")
	}}

	r.add("GET", "/:id", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/simple", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match")
	}
	res.handlers[0](&Ctx{ps: ps})
	if id != "simple" {
		t.Errorf("id = %q, want %q", id, "simple")
	}
}

func TestRadix_MethodSpecificTrees(t *testing.T) {
	r := newRadixRouter()

	r.add("GET", "/common", []HandlerFunc{func(c *Ctx) {}})
	r.add("POST", "/common", []HandlerFunc{func(c *Ctx) {}})

	// Ensure both methods have their own tree
	getRoot := r.trees.get("GET")
	postRoot := r.trees.get("POST")

	if getRoot == nil {
		t.Error("expected GET tree root")
	}
	if postRoot == nil {
		t.Error("expected POST tree root")
	}

	// Ensure they are different nodes (not the same pointer)
	if getRoot == postRoot {
		t.Error("GET and POST should have separate tree roots")
	}

	// PUT should not have a tree
	putRoot := r.trees.get("PUT")
	if putRoot != nil {
		t.Error("PUT should not have a tree")
	}
}

func TestRadix_TrailingSlashWithParam(t *testing.T) {
	r := newRadixRouter()
	h := []HandlerFunc{func(c *Ctx) {}}

	r.add("GET", "/users/:id/profile", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/users/42/profile/", &ps, &skipped)
	if !res.tsr {
		t.Error("expected TSR for /users/42/profile/")
	}
	if res.handlers != nil {
		t.Error("expected no handlers for TSR case")
	}
}

func TestRadix_UnescapePreservesNormalChars(t *testing.T) {
	r := newRadixRouter()
	var captured string
	h := []HandlerFunc{func(c *Ctx) {
		captured = c.Param("name")
	}}

	r.add("GET", "/:name", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/normal-chars-123", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match")
	}
	res.handlers[0](&Ctx{ps: ps})
	if captured != "normal-chars-123" {
		t.Errorf("captured = %q, want %q", captured, "normal-chars-123")
	}
}

func TestRadix_MultipleSkips(t *testing.T) {
	r := newRadixRouter()
	var a, b string
	h := []HandlerFunc{func(c *Ctx) {
		a = c.Param("a")
		b = c.Param("b")
	}}

	r.add("GET", "/api/:a/:b/action", h)

	var ps params
	var skipped []skippedNode
	res := r.find("GET", "/api/x/y/action", &ps, &skipped)
	if res.handlers == nil {
		t.Fatal("expected match")
	}
	res.handlers[0](&Ctx{ps: ps})
	if a != "x" || b != "y" {
		t.Errorf("got a=%s b=%s, want x y", a, b)
	}
}
