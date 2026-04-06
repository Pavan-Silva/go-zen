package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/auth"
	"github.com/Pavan-Silva/go-zen/sse"
	"github.com/Pavan-Silva/go-zen/ws"
)

func main() {
	r := zen.New(":8082")

	// Example authenticator (in production, use JWT, database lookup, etc.)
	authenticator := &auth.BasicAuth{
		Realm: "Zen Example",
		Validate: func(username, password string) (auth.User, error) {
			if username == "admin" && password == "secret" {
				return auth.User{
					ID:       "1",
					Username: "admin",
					Roles:    []string{"admin"},
				}, nil
			}
			return auth.User{}, fmt.Errorf("invalid credentials")
		},
	}

	// HTTP routes with auth
	r.Use(auth.RequireAuth(authenticator))

	r.Handle("GET /", func(c *zen.Context) {
		user := auth.GetUser(c)
		c.JSON(http.StatusOK, map[string]string{
			"message": "Zen SSE/WebSocket example running",
			"user":    user.Username,
			"sse":     "/events",
			"ws":      "/ws",
		})
	})

	// SSE with auth
	sse.HandleWithAuth(r, "GET /events", authenticator, func(c *zen.Context) error {
		user := auth.GetUser(c)
		for i := 1; i <= 5; i++ {
			if err := sse.Send(c, "message", map[string]any{
				"count": i,
				"user":  user.Username,
				"time":  time.Now().Format(time.RFC3339),
			}); err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
		}
		return nil
	})

	// WebSocket with auth
	ws.HandleWithAuth(r, "GET /ws", authenticator, func(c *zen.Context, conn *ws.Conn) {
		defer conn.Close()

		user := auth.GetUser(c)
		_ = conn.WriteJSON(map[string]any{"welcome": user.Username})

		for {
			var payload map[string]any
			if err := conn.ReadJSON(&payload); err != nil {
				return
			}

			payload["received_at"] = time.Now().Format(time.RFC3339)
			payload["user"] = user.Username
			payload["echo"] = true

			if err := conn.WriteJSON(payload); err != nil {
				return
			}
		}
	})

	fmt.Println("zen SSE/WebSocket example listening on :8082")
	if err := r.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
