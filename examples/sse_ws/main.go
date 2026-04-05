package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/sse"
	"github.com/Pavan-Silva/go-zen/ws"
)

func main() {
	r := zen.New(":8082")

	sse.Handle(r, "GET /events", func(c *zen.Context) error {
		for i := 1; i <= 5; i++ {
			if err := sse.Send(c, "message", map[string]any{
				"count": i,
				"time":  time.Now().Format(time.RFC3339),
			}); err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
		}
		return nil
	})

	ws.Handle(r, "GET /ws", func(c *zen.Context, conn *ws.Conn) {
		defer conn.Close()

		for {
			var payload map[string]any
			if err := conn.ReadJSON(&payload); err != nil {
				return
			}

			payload["received_at"] = time.Now().Format(time.RFC3339)
			payload["echo"] = true

			if err := conn.WriteJSON(payload); err != nil {
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

	r.ListenAndServe()
}
