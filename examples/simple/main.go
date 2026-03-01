package main

import (
	"log"
	"net/http"

	"github.com/Pavan-Silva/zen/zen"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/signup", zen.Adapt(func(c *zen.Context) {
		var payload struct {
			Username string `json:"username" validate:"required,alphanum"`
			Email    string `json:"email"    validate:"required,email"`
		}
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if errs := c.ValidateErr(&payload); errs != nil {
			c.JSON(http.StatusBadRequest, map[string]interface{}{"errors": errs})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"status": "created"})
	}))

	mux.HandleFunc("/json", zen.Adapt(func(c *zen.Context) {
		var payload struct {
			Message string `json:"message"`
		}
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"echo": payload.Message})
	}))

	mux.HandleFunc("/form", zen.Adapt(func(c *zen.Context) {
		var data struct {
			Name string `json:"name"`
		}
		if err := c.BindForm(&data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"you": data.Name})
	}))

	srv := zen.NewServer(":8080", mux)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
