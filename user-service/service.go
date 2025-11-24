package userservice

import (
    "net/http"
    "github.com/gorilla/mux"
    "github.com/jinzhu/gorm"
    _ "github.com/jinzhu/gorm/dialects/postgres"
    "golang.org/x/crypto/bcrypt"
)

type User struct {
    ID       uint   `json:"id" gorm:"primary_key"`
    Username string `json:"username"`
    Password string `json:"password"`
}

var db *gorm.DB

// Initialize database connection
func InitDB() {
    var err error
    db, err = gorm.Open("postgres", "host=localhost port=5432 user=youruser dbname=yourdb password=yourpass")
    if err != nil {
        panic(err)
    }
    db.AutoMigrate(&User{})
}

// Hash password
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
    return string(bytes), err
}

// Register new user
func RegisterUser(w http.ResponseWriter, r *http.Request) {
    var user User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    hashedPassword, err := HashPassword(user.Password)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    user.Password = hashedPassword
    db.Create(&user)
    w.WriteHeader(http.StatusCreated)
}

// Login user
func LoginUser(w http.ResponseWriter, r *http.Request) {
    var user User
    var foundUser User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    db.Where("username = ?", user.Username).First(&foundUser)
    if err := bcrypt.CompareHashAndPassword([]byte(foundUser.Password), []byte(user.Password)); err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    // Generate and return JWT (skipped for brevity)
}

// Middleware to protect routes
func Protected(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check JWT token (skipped for brevity)
    })
}

// Health check endpoint
func HealthCheck(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("OK"))
}

func main() {
    InitDB()
    r := mux.NewRouter()
    r.HandleFunc("/register", RegisterUser).Methods("POST")
    r.HandleFunc("/login", LoginUser).Methods("POST")
    r.HandleFunc("/health", HealthCheck).Methods("GET")
    r.Use(Protected)

    srv := &http.Server{
        Addr: "0.0.0.0:8080",
        Handler: r,
    }

    // Graceful shutdown
    go func() {
        if err := srv.ListenAndServe(); err != nil {
            panic(err)
        }
    }()

    // Handle shutdown (skipped for brevity)
}