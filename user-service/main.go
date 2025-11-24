 package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/gorilla/mux"
	"github.com/golang-jwt/jwt/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"`
	Email    string `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

var db *pgx.Conn

func initDB() {
	var err error
	
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "cilium_microservices"
	}
	
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		log.Fatal().Msg("DB_PASSWORD environment variable is required")
	}
	
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}
	
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)
	
	db, err = pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to connect to database")
	}
	
	// Create users table if not exists
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			email VARCHAR(100) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	
	_, err = db.Exec(context.Background(), createTableSQL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create users table")
	}
	
	log.Info().Msg("Database connected successfully")
}

func hashPassword(password string) (string, error) {
	cost := 12
	if costStr := os.Getenv("BCRYPT_COST"); costStr != "" {
		if parsedCost, err := strconv.Atoi(costStr); err == nil {
			cost = parsedCost
		}
	}
	
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

func verifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func registerUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request body",
			"code":  "INVALID_REQUEST",
		})
		return
	}
	
	// Validate required fields
	if req.Username == "" || req.Password == "" || req.Email == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Username, password, and email are required",
			"code":  "MISSING_FIELDS",
		})
		return
	}
	
	// Validate minimum password length
	if len(req.Password) < 6 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Password must be at least 6 characters",
			"code":  "WEAK_PASSWORD",
		})
		return
	}
	
	// Check if user already exists
	var existingID int
	checkSQL := "SELECT id FROM users WHERE username = $1 OR email = $2"
	err := db.QueryRow(context.Background(), checkSQL, req.Username, req.Email).Scan(&existingID)
	if err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "User with this username or email already exists",
			"code":  "USER_EXISTS",
		})
		return
	}
	
	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Password processing failed",
			"code":  "HASH_ERROR",
		})
		return
	}
	
	// Create user
	var newUser User
	insertSQL := `
		INSERT INTO users (username, password, email, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, email, created_at`
	
	err = db.QueryRow(context.Background(), insertSQL,
		req.Username, hashedPassword, req.Email, time.Now()).
		Scan(&newUser.ID, &newUser.Username, &newUser.Email, &newUser.CreatedAt)
	
	if err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to create user",
			"code":  "CREATE_ERROR",
		})
		return
	}
	
	log.Info().Str("username", newUser.Username).Msg("User registered successfully")
	
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "User registered successfully",
		"user":    newUser,
	})
}

func authenticateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request body",
			"code":  "INVALID_REQUEST",
		})
		return
	}
	
	// Validate required fields
	if req.Username == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Username and password are required",
			"code":  "MISSING_CREDENTIALS",
		})
		return
	}
	
	// Find user
	var user User
	var hashedPassword string
	selectSQL := `
		SELECT id, username, password, email, created_at
		FROM users WHERE username = $1`
	
	err := db.QueryRow(context.Background(), selectSQL, req.Username).
		Scan(&user.ID, &user.Username, &hashedPassword, &user.Email, &user.CreatedAt)
	
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid credentials",
			"code":  "INVALID_CREDENTIALS",
		})
		return
	}
	
	// Verify password
	if err := verifyPassword(req.Password, hashedPassword); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid credentials",
			"code":  "INVALID_CREDENTIALS",
		})
		return
	}
	
	// Generate JWT token
	token, err := generateJWT(user.Username, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate JWT token")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Token generation failed",
			"code":  "TOKEN_ERROR",
		})
		return
	}
	
	log.Info().Str("username", user.Username).Msg("User authenticated successfully")
	
	response := LoginResponse{
		Token: token,
		User:  user,
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func generateJWT(username string, userID int) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET required")
	}
	
	expiryHours := 24
	if hoursStr := os.Getenv("TOKEN_EXPIRY_HOURS"); hoursStr != "" {
		if parsed, err := strconv.Atoi(hoursStr); err == nil {
			expiryHours = parsed
		}
	}
	
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(time.Duration(expiryHours) * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
		"iss":      "cilium-user-service",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Test database connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	if err := db.Ping(ctx); err != nil {
		log.Error().Err(err).Msg("Database health check failed")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "unhealthy",
			"error":     "database connection failed",
			"service":   "user-service",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"service":   "user-service",
		"version":   "1.0.0",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(startTime).String(),
	})
}

var startTime = time.Now()

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	initDB()
	defer db.Close(context.Background())

	r := mux.NewRouter()
	r.HandleFunc("/register", registerUser).Methods("POST")
	r.HandleFunc("/login", authenticateUser).Methods("POST")
	r.HandleFunc("/health", healthCheck).Methods("GET")

	port := os.Getenv("USER_SERVICE_PORT")
	if port == "" {
		port = "8001"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("User service starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server startup failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info().Msg("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}
	log.Info().Msg("Server exited")
}
