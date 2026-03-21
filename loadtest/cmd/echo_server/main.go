package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
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

// Auth is an Echo middleware that validates a Bearer token in the Authorization
// header and stores the resolved user ID in the Echo context under "userID"
// for downstream handlers to retrieve via c.Get("userID").
func Auth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := bearerToken(c.Request().Header.Get("Authorization"))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "missing or malformed Authorization header",
			})
		}

		userID, ok := validateToken(token)
		if !ok {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "invalid or expired token",
			})
		}

		// Store the resolved identity so handlers can retrieve it with
		// c.Get("userID") without re-parsing the header.
		c.Set("userID", userID)

		return next(c)
	}
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
	addr := ":8083"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	api := e.Group("")
	api.Use(Auth)

	api.GET("/products", func(c echo.Context) error {
		return c.JSON(http.StatusOK, catalog)
	})

	api.POST("/orders", func(c echo.Context) error {
		var req OrderRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, OrderResponse{
			OrderID:   "ORD-12345",
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Total:     float64(req.Quantity) * 29.99,
			Status:    "confirmed",
		})
	})

	api.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	items := make([]Product, 500)
	for i := range items {
		items[i] = Product{i + 1, "Product " + strconv.Itoa(i+1),
			float64(i+1) * 9.99, "electronics", true}
	}
	api.GET("/large", func(c echo.Context) error {
		return c.JSON(http.StatusOK, items)
	})

	log.Printf("[echo] listening on %s", addr)
	if err := e.Start(addr); err != nil {
		log.Fatal(err)
	}
}
