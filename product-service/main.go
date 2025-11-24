package main

import (
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "os"

    _ "github.com/lib/pq"
    "github.com/gorilla/mux"
)

// Product represents the product model
// Add additional fields as necessary

type Product struct {
    ID    int     `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
}

// DB connection function
func connectDB() (*sql.DB, error) {
    // Replace with your actual PostgreSQL credentials
    connStr := "user=yourusername dbname=yourdbname sslmode=disable"
    return sql.Open("postgres", connStr)
}

// GetProducts - Get list of products
func GetProducts(w http.ResponseWriter, r *http.Request) {
    db, err := connectDB()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer db.Close()

    rows, err := db.Query("SELECT id, name, price FROM products")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var products []Product
    for rows.Next() {
        var product Product
        if err := rows.Scan(&product.ID, &product.Name, &product.Price); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        products = append(products, product)
    }

    json.NewEncoder(w).Encode(products)
}

// HealthCheck - Health check endpoint
func HealthCheck(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func main() {
    r := mux.NewRouter()
    r.HandleFunc("/products", GetProducts).Methods("GET")
    r.HandleFunc("/health", HealthCheck).Methods("GET")

    log.Println("Server starting on :8080")
    if err := http.ListenAndServe(":8080", r); err != nil {
        log.Fatal(err)
    }
}