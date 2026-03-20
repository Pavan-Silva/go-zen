package main

import (
	"net/http"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/middleware"
)

func main() {
	mux := zen.New(":8080")
	mux.Use(middleware.Recover)

	mux.Handle("/signup", func(c *zen.Context) {
		var payload struct {
			Username string `json:"username" validate:"required,alphanum"`
			Email    string `json:"email"    validate:"required,email"`
		}
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"status": "created"})
	})

	mux.Handle("/json", func(c *zen.Context) {
		var payload struct {
			Message string `json:"message"`
		}
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"echo": payload.Message})
	})

	mux.Handle("/form", func(c *zen.Context) {
		var data struct {
			Name string `json:"name"`
		}
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"you": data.Name})
	})

	mux.ListenAndServe()
}
