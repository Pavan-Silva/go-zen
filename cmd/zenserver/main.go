//go:build ignore

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Pavan-Silva/zen"
	"github.com/Pavan-Silva/zen/middleware"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	r := zen.New(addr)
	r.Use(middleware.Recover)

	r.Handle("GET /ping", func(c *zen.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	log.Printf("zen server listening on %s", addr)
	r.ListenAndServe()
}
