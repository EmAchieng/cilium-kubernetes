package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/dgrijalva/jwt-go"
)

// JWTSecret is read from environment variables
var JWTSecret = os.Getenv("JWT_SECRET")

// ValidateToken validates the JWT token and returns the claims
func ValidateToken(tokenString string) (*jwt.Claims, error) {
    // Check if the token is in the correct Bearer format
    if len(tokenString) == 0 || !strings.HasPrefix(tokenString, "Bearer ") {
        log.Println("Invalid token format")
        return nil, fmt.Errorf("invalid token format")
    }

    // Remove "Bearer " prefix to get token
    tokenString = tokenString[7:]

    // Parse the token
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        // Check the signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            log.Println("Unexpected signing method")
            return nil, fmt.Errorf("unexpected signing method")
        }
        return []byte(JWTSecret), nil
    })

    if err != nil {
        log.Println("Error parsing token:", err)
        return nil, err
    }

    // Validate claims
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        // Check for expiration claims
        if exp, ok := claims["exp"].(float64); ok {
            if time.Now().Unix() > int64(exp) {
                log.Println("Token has expired")
                return nil, fmt.Errorf("token expired")
            }
        }
        // Set user context if needed
        ctx := context.WithValue(context.Background(), "user", claims)
        return ctx, nil
    }
    return nil, fmt.Errorf("invalid token")
}