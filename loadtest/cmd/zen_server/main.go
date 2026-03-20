package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Pavan-Silva/zen/zen"
)

type Product struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Category string  `json:"category"`
	InStock  bool    `json:"in_stock"`
}

type OrderRequest struct {
	ProductID int    `json:"product_id"`
	Quantity  int    `json:"quantity"`
	UserID    string `json:"user_id"`
}

type OrderResponse struct {
	OrderID   string  `json:"order_id"`
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Total     float64 `json:"total"`
	Status    string  `json:"status"`
}

var catalog = makeCatalog()

func makeCatalog() []Product {
	p := make([]Product, 20)
	for i := range p {
		p[i] = Product{i + 1, "Product " + strconv.Itoa(i+1),
			float64(i+1) * 9.99, "electronics", true}
	}
	return p
}

// Auth is a global middleware that validates a Bearer token in the
// Authorization header and stores the resolved user ID in the zen.Context
// under the "userID" key for downstream handlers to retrieve via c.Get("userID").
//
// Usage:
//
//	s.Use(middleware.Auth)
//
// In production, replace the validateToken stub with real JWT verification,
// a session lookup, or a cache check.
func Auth(c *zen.Context, next http.Handler) {
	token := bearerToken(c.Request.Header.Get("Authorization"))
	if token == "" {
		c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "missing or malformed Authorization header",
		})
		return
	}

	userID, ok := validateToken(token)
	if !ok {
		c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "invalid or expired token",
		})
		return
	}

	c.Set("userID", userID)
	next.ServeHTTP(c.Response, c.Request)
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return ""
	}
	return token
}

// validateToken is a stub — replace with real JWT or session validation.
func validateToken(token string) (userID string, ok bool) {
	if token == "secret" {
		return "user-123", true
	}
	return "", false
}

func main() {
	addr := ":8081"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	s := zen.New(addr)
	s.Use(Auth)

	s.Handle("GET /products", func(c *zen.Context) {
		c.JSON(http.StatusOK, catalog)
	})

	s.Handle("POST /orders", func(c *zen.Context) {
		var req OrderRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, OrderResponse{
			OrderID:   "ORD-12345",
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Total:     float64(req.Quantity) * 29.99,
			Status:    "confirmed",
		})
	})

	s.Handle("GET /ping", func(c *zen.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	items := make([]Product, 500)
	for i := range items {
		items[i] = Product{i + 1, "Product " + strconv.Itoa(i+1),
			float64(i+1) * 9.99, "electronics", true}
	}
	// pre-marshal large payload once
	large, err := json.Marshal(items)
	if err != nil {
		log.Fatal(err)
	}
	s.HandleRaw("GET /large", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(large)
	}))

	log.Printf("[zen] listening on %s", addr)
	s.Server.ListenAndServe() // call directly to avoid banner+graceful overhead
}
