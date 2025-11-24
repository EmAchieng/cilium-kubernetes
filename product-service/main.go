package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Product struct {
	ID          int       `json:"id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type ProductCreateRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
}

type ProductUpdateRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Category    *string  `json:"category,omitempty"`
	Price       *float64 `json:"price,omitempty"`
	Stock       *int     `json:"stock,omitempty"`
}

type ProductListResponse struct {
	Products   []Product          `json:"products"`
	Pagination PaginationMetadata `json:"pagination"`
}

type PaginationMetadata struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

type ErrorResponse struct {
	Error      string `json:"error"`
	ErrorCode  string `json:"error_code"`
	StatusCode int    `json:"status_code"`
	Timestamp  string `json:"timestamp"`
}

type HealthCheckResponse struct {
	Status    string `json:"status"`
	Database  string `json:"database"`
	Timestamp string `json:"timestamp"`
}

var dbPool *pgxpool.Pool

func initDB() {
	var err error

	dbHost := os.Getenv("PRODUCT_DB_HOST")
	dbPort := os.Getenv("PRODUCT_DB_PORT")
	dbUser := os.Getenv("PRODUCT_DB_USER")
	dbPassword := os.Getenv("PRODUCT_DB_PASSWORD")
	dbName := os.Getenv("PRODUCT_DB_NAME")
	dbSSLMode := os.Getenv("PRODUCT_DB_SSLMODE")

	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432"
	}
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

	connString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s pool_max_conns=10",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode,
	)

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to parse database configuration")
	}

	dbPool, err = pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to connect to database")
	}

	if err := createProductsTable(); err != nil {
		log.Fatal().Err(err).Msg("Unable to create products table")
	}

	log.Info().Msg("Database connection established successfully")
}

func createProductsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS products (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		category VARCHAR(100),
		price DECIMAL(10, 2) NOT NULL CHECK (price >= 0),
		stock INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_products_category ON products(category);
	CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
	`
	_, err := dbPool.Exec(context.Background(), query)
	return err
}

func getProducts(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))

	offset := (page - 1) * pageSize

	var totalItems int
	countQuery := "SELECT COUNT(*) FROM products WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if search != "" {
		countQuery += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argIndex, argIndex)
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if category != "" {
		countQuery += fmt.Sprintf(" AND category = $%d", argIndex)
		args = append(args, category)
		argIndex++
	}

	err := dbPool.QueryRow(context.Background(), countQuery, args...).Scan(&totalItems)
	if err != nil {
		log.Error().Err(err).Msg("Error counting products")
		respondWithError(w, http.StatusInternalServerError, "PROD_001", "Database error")
		return
	}

	query := `
		SELECT id, name, description, category, price, stock, created_at, updated_at 
		FROM products WHERE 1=1
	`
	
	if search != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", 1, 1)
	}

	if category != "" {
		if search != "" {
			query += fmt.Sprintf(" AND category = $%d", 2)
		} else {
			query += fmt.Sprintf(" AND category = $%d", 1)
		}
	}

	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, pageSize, offset)

	rows, err := dbPool.Query(context.Background(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Error querying products")
		respondWithError(w, http.StatusInternalServerError, "PROD_002", "Database error")
		return
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&product.ID, &product.Name, &product.Description, &product.Category,
			&product.Price, &product.Stock, &product.CreatedAt, &product.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Error scanning product")
			continue
		}
		products = append(products, product)
	}

	totalPages := (totalItems + pageSize - 1) / pageSize

	response := ProductListResponse{
		Products: products,
		Pagination: PaginationMetadata{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "PROD_003", "Invalid product ID")
		return
	}

	var product Product
	query := `
		SELECT id, name, description, category, price, stock, created_at, updated_at 
		FROM products WHERE id = $1
	`
	err = dbPool.QueryRow(context.Background(), query, id).Scan(
		&product.ID, &product.Name, &product.Description, &product.Category,
		&product.Price, &product.Stock, &product.CreatedAt, &product.UpdatedAt,
	)

	if err != nil {
		log.Error().Err(err).Msg("Product not found")
		respondWithError(w, http.StatusNotFound, "PROD_004", "Product not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

func createProduct(w http.ResponseWriter, r *http.Request) {
	var req ProductCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "PROD_005", "Invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Category = strings.TrimSpace(req.Category)

	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "PROD_006", "Product name is required")
		return
	}

	if req.Price < 0 {
		respondWithError(w, http.StatusBadRequest, "PROD_007", "Price must be non-negative")
		return
	}

	if req.Stock < 0 {
		respondWithError(w, http.StatusBadRequest, "PROD_008", "Stock must be non-negative")
		return
	}

	var product Product
	query := `
		INSERT INTO products (name, description, category, price, stock) 
		VALUES ($1, $2, $3, $4, $5) 
		RETURNING id, name, description, category, price, stock, created_at, updated_at
	`
	err := dbPool.QueryRow(context.Background(), query,
		req.Name, req.Description, req.Category, req.Price, req.Stock).Scan(
		&product.ID, &product.Name, &product.Description, &product.Category,
		&product.Price, &product.Stock, &product.CreatedAt, &product.UpdatedAt,
	)

	if err != nil {
		log.Error().Err(err).Msg("Error creating product")
		respondWithError(w, http.StatusInternalServerError, "PROD_009", "Failed to create product")
		return
	}

	log.Info().Int("product_id", product.ID).Str("name", product.Name).Msg("Product created successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

func updateProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "PROD_010", "Invalid product ID")
		return
	}

	var req ProductUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "PROD_011", "Invalid request body")
		return
	}

	var exists bool
	err = dbPool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM products WHERE id = $1)", id).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "PROD_012", "Product not found")
		return
	}

	updates := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.Name != nil {
		*req.Name = strings.TrimSpace(*req.Name)
		if *req.Name == "" {
			respondWithError(w, http.StatusBadRequest, "PROD_013", "Product name cannot be empty")
			return
		}
		updates = append(updates, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *req.Name)
		argIndex++
	}

	if req.Description != nil {
		*req.Description = strings.TrimSpace(*req.Description)
		updates = append(updates, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, *req.Description)
		argIndex++
	}

	if req.Category != nil {
		*req.Category = strings.TrimSpace(*req.Category)
		updates = append(updates, fmt.Sprintf("category = $%d", argIndex))
		args = append(args, *req.Category)
		argIndex++
	}

	if req.Price != nil {
		if *req.Price < 0 {
			respondWithError(w, http.StatusBadRequest, "PROD_014", "Price must be non-negative")
			return
		}
		updates = append(updates, fmt.Sprintf("price = $%d", argIndex))
		args = append(args, *req.Price)
		argIndex++
	}

	if req.Stock != nil {
		if *req.Stock < 0 {
			respondWithError(w, http.StatusBadRequest, "PROD_015", "Stock must be non-negative")
			return
		}
		updates = append(updates, fmt.Sprintf("stock = $%d", argIndex))
		args = append(args, *req.Stock)
		argIndex++
	}

	if len(updates) == 0 {
		respondWithError(w, http.StatusBadRequest, "PROD_016", "No fields to update")
		return
	}

	updates = append(updates, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now())
	argIndex++

	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE products SET %s 
		WHERE id = $%d
		RETURNING id, name, description, category, price, stock, created_at, updated_at
	`, strings.Join(updates, ", "), argIndex)

	var product Product
	err = dbPool.QueryRow(context.Background(), query, args...).Scan(
		&product.ID, &product.Name, &product.Description, &product.Category,
		&product.Price, &product.Stock, &product.CreatedAt, &product.UpdatedAt,
	)

	if err != nil {
		log.Error().Err(err).Msg("Error updating product")
		respondWithError(w, http.StatusInternalServerError, "PROD_017", "Failed to update product")
		return
	}

	log.Info().Int("product_id", product.ID).Msg("Product updated successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

func deleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "PROD_018", "Invalid product ID")
		return
	}

	result, err := dbPool.Exec(context.Background(), "DELETE FROM products WHERE id = $1", id)
	if err != nil {
		log.Error().Err(err).Msg("Error deleting product")
		respondWithError(w, http.StatusInternalServerError, "PROD_019", "Failed to delete product")
		return
	}

	if result.RowsAffected() == 0 {
		respondWithError(w, http.StatusNotFound, "PROD_020", "Product not found")
		return
	}

	log.Info().Int("product_id", id).Msg("Product deleted successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Product deleted successfully",
	})
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	response := HealthCheckResponse{
		Status:    "healthy",
		Database:  "disconnected",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if dbPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := dbPool.Ping(ctx); err == nil {
			response.Database = "connected"
		} else {
			log.Warn().Err(err).Msg("Database health check failed")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if response.Database != "connected" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	json.NewEncoder(w).Encode(response)
}

func respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	errorResponse := ErrorResponse{
		Error:      message,
		ErrorCode:  errorCode,
		StatusCode: statusCode,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse)
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	port := os.Getenv("PRODUCT_SERVICE_PORT")
	if port == "" {
		port = "8002"
	}

	initDB()
	defer dbPool.Close()

	r := mux.NewRouter()
	r.HandleFunc("/products", getProducts).Methods("GET")
	r.HandleFunc("/products", createProduct).Methods("POST")
	r.HandleFunc("/products/{id}", getProduct).Methods("GET")
	r.HandleFunc("/products/{id}", updateProduct).Methods("PUT")
	r.HandleFunc("/products/{id}", deleteProduct).Methods("DELETE")
	r.HandleFunc("/health", healthCheck).Methods("GET")

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("Product service starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("ListenAndServe failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info().Msg("Shutting down product service...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}
	log.Info().Msg("Product service exited gracefully")
}