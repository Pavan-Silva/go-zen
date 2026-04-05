package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Pavan-Silva/go-zen"
)

func main() {
	r := zen.New(":8082")

	r.Handle("GET /events", func(c *zen.Context) {
		// Keep the connection open and send five SSE events.
		for i := 1; i <= 5; i++ {
			if err := c.SSEvent("message", map[string]any{
				"count": i,
				"time":  time.Now().Format(time.RFC3339),
			}); err != nil {
				return
			}
			time.Sleep(1 * time.Second)
		}
	})

	r.HandleWebSocket("GET /ws", func(c *zen.Context, ws *zen.WebSocketConn) {
		defer ws.Close()

		for {
			var payload map[string]any
			if err := ws.ReadJSON(&payload); err != nil {
				return
			}

			payload["received_at"] = time.Now().Format(time.RFC3339)
			payload["echo"] = true

			if err := ws.WriteJSON(payload); err != nil {
				return
			}
		}
	})

	r.Handle("GET /", func(c *zen.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Zen SSE/WebSocket example running",
			"sse":     "/events",
			"ws":      "/ws",
		})
	})

	r.ListenAndServe();
}
