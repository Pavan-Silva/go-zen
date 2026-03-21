package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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

// Auth is a Gin middleware that validates a Bearer token in the Authorization
// header and stores the resolved user ID in the Gin context under "userID"
// for downstream handlers to retrieve via c.GetString("userID").
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			c.Abort()
			return
		}

		userID, ok := validateToken(token)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Store the resolved identity so handlers can retrieve it with
		// c.GetString("userID") without re-parsing the header.
		c.Set("userID", userID)
		c.Next()
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
	addr := ":8082"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/")
	api.Use(Auth())

	api.GET("/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, catalog)
	})

	api.POST("/orders", func(c *gin.Context) {
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

	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	items := make([]Product, 500)
	for i := range items {
		items[i] = Product{i + 1, "Product " + strconv.Itoa(i+1),
			float64(i+1) * 9.99, "electronics", true}
	}
	api.GET("/large", func(c *gin.Context) {
		c.JSON(http.StatusOK, items)
	})

	log.Printf("[gin] listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
