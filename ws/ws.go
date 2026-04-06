package ws

import (
	"net/http"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/auth"
	"golang.org/x/net/websocket"
)

// Conn wraps a websocket connection with JSON helpers.
type Conn struct {
	conn *websocket.Conn
}

// ReadJSON reads a JSON message from the WebSocket.
func (c *Conn) ReadJSON(dest any) error {
	return websocket.JSON.Receive(c.conn, dest)
}

// WriteJSON writes a JSON message to the WebSocket.
func (c *Conn) WriteJSON(message any) error {
	return websocket.JSON.Send(c.conn, message)
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Handler converts the provided function into an http.Handler.
func Handler(fn func(*zen.Context, *Conn)) http.Handler {
	return websocket.Handler(func(conn *websocket.Conn) {
		c := zen.FromRequest(conn.Request())
		if c == nil {
			_ = conn.Close()
			return
		}
		fn(c, &Conn{conn: conn})
	})
}

// Handle registers a WebSocket route on the provided Router.
func Handle(r *zen.Router, pattern string, fn func(*zen.Context, *Conn)) {
	r.HandleRaw(pattern, Handler(fn))
}

// HandleWithAuth registers an authenticated WebSocket route.
// The authenticator is called during the connection upgrade, and the authenticated User
// is stored in the Context under "user".
func HandleWithAuth(r *zen.Router, pattern string, auth auth.Authenticator, fn func(*zen.Context, *Conn)) {
	authHandler := websocket.Handler(func(conn *websocket.Conn) {
		c := zen.FromRequest(conn.Request())
		if c == nil {
			_ = conn.Close()
			return
		}

		user, err := auth.AuthenticateWS(conn.Request())
		if err != nil {
			_ = conn.Close()
			return
		}

		c.Set("user", user)
		fn(c, &Conn{conn: conn})
	})

	r.HandleRaw(pattern, authHandler)
}
