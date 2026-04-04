package zen

var responses = struct {
	Pong             []byte
	UnauthorizedBody []byte
}{
	Pong:             []byte(`{"status":"ok"}`),
	UnauthorizedBody: []byte(`{"error":"unauthorized"}`),
}

func (c *Context) Pong() {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(200)
	c.Response.Write(responses.Pong)
}

func (c *Context) Unauthorized() {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(401)
	c.Response.Write(responses.UnauthorizedBody)
}
