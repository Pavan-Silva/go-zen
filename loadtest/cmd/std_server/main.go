package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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

// userIDKey is an unexported type used as a context key to avoid collisions
// with keys set by other packages.
type userIDKey struct{}

// Auth is a standard http.Handler middleware that validates a Bearer token in
// the Authorization header and stores the resolved user ID in the request
// context under userIDKey so handlers can retrieve it with
// r.Context().Value(userIDKey{}).(string).
//
// It wraps the entire mux, so all routes are protected. Pass the mux to
// http.ListenAndServe as Auth(mux) to apply it globally.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"missing or malformed Authorization header"}`))
			return
		}

		userID, ok := validateToken(token)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid or expired token"}`))
			return
		}

		// Store the resolved identity in the request context so handlers can
		// retrieve it without re-parsing the header.
		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), userIDKey{}, userID),
		))
	})
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
	addr := ":8084"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(catalog)
		w.Write(b)
	})

	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		var req OrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		userID, _ := r.Context().Value(userIDKey{}).(string)
		log.Printf("order placed by %s", userID)

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

	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("GET /large", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(largePayload)
	})

	// Wrap the entire mux with Auth so every route is protected.
	log.Printf("[stdlib] listening on %s", addr)
	if err := http.ListenAndServe(addr, Auth(mux)); err != nil {
		log.Fatal(err)
	}
}
