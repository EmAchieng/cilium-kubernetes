package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int    `json:"id,omitempty"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expires_at"`
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
	
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbSSLMode := os.Getenv("DB_SSLMODE")

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

	if err := createUsersTable(); err != nil {
		log.Fatal().Err(err).Msg("Unable to create users table")
	}

	log.Info().Msg("Database connection established successfully")
}

func createUsersTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(100) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	`
	_, err := dbPool.Exec(context.Background(), query)
	return err
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func registerUser(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "REG_001", "Invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" || len(req.Username) < 3 {
		respondWithError(w, http.StatusBadRequest, "REG_002", "Username must be at least 3 characters long")
		return
	}

	if req.Email == "" || !isValidEmail(req.Email) {
		respondWithError(w, http.StatusBadRequest, "REG_003", "Valid email address is required")
		return
	}

	if req.Password == "" || len(req.Password) < 6 {
		respondWithError(w, http.StatusBadRequest, "REG_004", "Password must be at least 6 characters long")
		return
	}

	var exists bool
	err := dbPool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 OR email = $2)",
		req.Username, req.Email).Scan(&exists)
	if err != nil {
		log.Error().Err(err).Msg("Database query error during user check")
		respondWithError(w, http.StatusInternalServerError, "REG_005", "Database error")
		return
	}

	if exists {
		respondWithError(w, http.StatusConflict, "REG_006", "Username or email already exists")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error().Err(err).Msg("Password hashing error")
		respondWithError(w, http.StatusInternalServerError, "REG_007", "Error processing password")
		return
	}

	var userID int
	err = dbPool.QueryRow(context.Background(),
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		req.Username, req.Email, string(hashedPassword)).Scan(&userID)
	if err != nil {
		log.Error().Err(err).Msg("Database insert error")
		respondWithError(w, http.StatusInternalServerError, "REG_008", "Failed to create user")
		return
	}

	log.Info().
		Str("username", req.Username).
		Int("user_id", userID).
		Msg("User registered successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "User registered successfully",
		"user_id":  userID,
		"username": req.Username,
	})
}

func authenticateUser(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "AUTH_001", "Invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)

	if req.Username == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "AUTH_002", "Username and password are required")
		return
	}

	var user User
	var passwordHash string
	err := dbPool.QueryRow(context.Background(),
		"SELECT id, username, email, password_hash FROM users WHERE username = $1",
		req.Username).Scan(&user.ID, &user.Username, &user.Email, &passwordHash)

	if err != nil {
		if err == pgx.ErrNoRows {
			respondWithError(w, http.StatusUnauthorized, "AUTH_003", "Invalid credentials")
			return
		}
		log.Error().Err(err).Msg("Database query error during authentication")
		respondWithError(w, http.StatusInternalServerError, "AUTH_004", "Database error")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "AUTH_005", "Invalid credentials")
		return
	}

	token, expiresAt, err := generateJWT(user.Username)
	if err != nil {
		log.Error().Err(err).Msg("JWT generation error")
		respondWithError(w, http.StatusInternalServerError, "AUTH_006", "Failed to generate token")
		return
	}

	log.Info().
		Str("username", user.Username).
		Int("user_id", user.ID).
		Msg("User authenticated successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(LoginResponse{
		Token:     token,
		Username:  user.Username,
		ExpiresAt: expiresAt,
	})
}

func generateJWT(username string) (string, int64, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", 0, fmt.Errorf("JWT_SECRET environment variable not set")
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := jwt.MapClaims{
		"username": username,
		"exp":      expirationTime.Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", 0, err
	}

	return tokenString, expirationTime.Unix(), nil
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

	port := os.Getenv("USER_SERVICE_PORT")
	if port == "" {
		port = "8001"
	}

	initDB()
	defer dbPool.Close()

	r := mux.NewRouter()
	r.HandleFunc("/register", registerUser).Methods("POST")
	r.HandleFunc("/login", authenticateUser).Methods("POST")
	r.HandleFunc("/health", healthCheck).Methods("GET")

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
			log.Fatal().Err(err).Msg("ListenAndServe failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info().Msg("Shutting down user service...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}
	log.Info().Msg("User service exited gracefully")
}