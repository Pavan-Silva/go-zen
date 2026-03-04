package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

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

func main() {
	addr := ":8083"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.GET("/products", func(c echo.Context) error {
		return c.JSON(http.StatusOK, catalog)
	})

	e.POST("/orders", func(c echo.Context) error {
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

	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	items := make([]Product, 500)
	for i := range items {
		items[i] = Product{i + 1, "Product " + strconv.Itoa(i+1),
			float64(i+1) * 9.99, "electronics", true}
	}
	e.GET("/large", func(c echo.Context) error {
		return c.JSON(http.StatusOK, items)
	})

	log.Printf("[echo] listening on %s", addr)
	if err := e.Start(addr); err != nil {
		log.Fatal(err)
	}
}
