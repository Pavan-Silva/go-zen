package ws

import (
	"log"
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
func (c *Conn) Close() {
	err := c.conn.Close()
	if err != nil {
		log.Printf("ws: failed to close connection: %v", err)
	}
}

// Handler converts the provided function into an http.Handler.
func Handler(fn func(*zen.Context, *Conn)) http.Handler {
	return websocket.Handler(func(conn *websocket.Conn) {
		c := zen.FromRequest(conn.Request())
		if c == nil {
			if err := conn.Close(); err != nil {
				log.Printf("ws: failed to close connection: %v", err)
			}
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
func HandleWithAuth(r *zen.Router, pattern string, authenticator auth.Authenticator, fn func(*zen.Context, *Conn)) {
	authHandler := websocket.Handler(func(conn *websocket.Conn) {
		c := zen.FromRequest(conn.Request())
		if c == nil {
			if err := conn.Close(); err != nil {
				log.Printf("ws: failed to close connection: %v", err)
			}
			return
		}

		user, err := auth.AuthenticateWS(authenticator, conn.Request())
		if err != nil {
			if cerr := conn.Close(); cerr != nil {
				log.Printf("ws: failed to close connection after auth failure: %v", cerr)
			}
			return
		}

		c.Set("user", user)
		fn(c, &Conn{conn: conn})
	})

	r.HandleRaw(pattern, authHandler)
}
