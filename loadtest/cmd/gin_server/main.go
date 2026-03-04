package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
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
	addr := ":8082"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.GET("/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, catalog)
	})

	r.POST("/orders", func(c *gin.Context) {
		var req OrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	items := make([]Product, 500)
	for i := range items {
		items[i] = Product{i + 1, "Product " + strconv.Itoa(i+1),
			float64(i+1) * 9.99, "electronics", true}
	}
	r.GET("/large", func(c *gin.Context) {
		c.JSON(http.StatusOK, items)
	})

	log.Printf("[gin] listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
