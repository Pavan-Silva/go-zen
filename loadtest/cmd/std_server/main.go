package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
)

// ── shared test payloads ────────────────────────────────────────────────────

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
		p[i] = Product{ID: i + 1, Name: "Product " + strconv.Itoa(i+1),
			Price: float64(i+1) * 9.99, Category: "electronics", InStock: true}
	}
	return p
}

// largePayload is ~64 KB JSON used for the large-payload scenario.
var largePayload []byte

func init() {
	items := make([]Product, 500)
	for i := range items {
		items[i] = Product{i + 1, "Product " + strconv.Itoa(i+1),
			float64(i+1) * 9.99, "electronics", true}
	}
	var err error
	largePayload, err = json.Marshal(items)
	if err != nil {
		log.Fatal(err)
	}
}

// ── std-lib server ──────────────────────────────────────────────────────────

func main() {
	addr := ":8084"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	mux := http.NewServeMux()

	// GET /products  — product catalog
	mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(catalog)
		w.Write(b)
	})

	// POST /orders   — place order
	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		var req OrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := OrderResponse{
			OrderID:   "ORD-12345",
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Total:     float64(req.Quantity) * 29.99,
			Status:    "confirmed",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /ping      — concurrent burst (tiny response)
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// GET /large     — large payload
	mux.HandleFunc("GET /large", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(largePayload)
	})

	log.Printf("[stdlib] listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
