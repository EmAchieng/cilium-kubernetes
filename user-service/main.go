package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/gorilla/mux"
	"github.com/dgrijalva/jwt-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var db *pgx.Conn

func initDB() {
	var err error
	db, err = pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to connect to database")
	}
}

func registerUser(w http.ResponseWriter, r *http.Request) {
	// Registration logic here
}

func authenticateUser(w http.ResponseWriter, r *http.Request) {
	// Authentication logic here
}

func generateJWT(username string) (string, error) {
	// JWT generation logic here
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	// Health check logic
	w.WriteHeader(http.StatusOK)
}

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	initDB()

	r := mux.NewRouter()
	r.HandleFunc("/register", registerUser).Methods("POST")
	r.HandleFunc("/login", authenticateUser).Methods("POST")
	r.HandleFunc("/health", healthCheck).Methods("GET")

	srv := &http.Server{
		Addr: ":8080",
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("ListenAndServe failed")
		}
	}()

	log.Info().Msg("Server started on port 8080")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info().Msg("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}
	log.Info().Msg("Server exited")
}