package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
)

type APIGateway struct {
	router    *gin.Engine
	logger    *logrus.Logger
	jwtSecret []byte
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services"`
	Version   string            `json:"version"`
}

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func NewAPIGateway() *APIGateway {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "cilium-default-secret"
		logger.Warn("JWT_SECRET not configured")
	}

	return &APIGateway{
		router:    router,
		logger:    logger,
		jwtSecret: []byte(jwtSecret),
	}
}

func (gw *APIGateway) setupRoutes() {
	v1 := gw.router.Group("/api/v1")

	gw.router.GET("/health", gw.healthCheck)

	userGroup := v1.Group("/users")
	userGroup.POST("/register", gw.proxyToUserService)
	userGroup.POST("/login", gw.proxyToUserService)
	userGroup.GET("/profile", gw.authMiddleware(), gw.proxyToUserService)

	productGroup := v1.Group("/products")
	productGroup.GET("", gw.proxyToProductService)
	productGroup.POST("", gw.authMiddleware(), gw.proxyToProductService)
	productGroup.GET("/:id", gw.proxyToProductService)
	productGroup.PUT("/:id", gw.authMiddleware(), gw.proxyToProductService)
	productGroup.DELETE("/:id", gw.authMiddleware(), gw.proxyToProductService)
}

func (gw *APIGateway) healthCheck(c *gin.Context) {
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "user-service:8001"
	}

	productServiceURL := os.Getenv("PRODUCT_SERVICE_URL")
	if productServiceURL == "" {
		productServiceURL = "product-service:8002"
	}

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   "1.0.0",
		Services: map[string]string{
			"user-service":    gw.checkServiceHealth(userServiceURL),
			"product-service": gw.checkServiceHealth(productServiceURL),
		},
	}

	c.JSON(http.StatusOK, response)
}

func (gw *APIGateway) checkServiceHealth(serviceURL string) string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/health", serviceURL))
	if err != nil {
		return "unhealthy"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "healthy"
	}
	return "unhealthy"
}

func (gw *APIGateway) authMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization required",
				"code":  "AUTH_MISSING",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Bearer token required",
				"code":  "INVALID_FORMAT",
			})
			c.Abort()
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return gw.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
				"code":  "TOKEN_INVALID",
			})
			c.Abort()
			return
		}

		if time.Now().Unix() > claims.ExpiresAt.Unix() {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token expired",
				"code":  "TOKEN_EXPIRED",
			})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	})
}

func (gw *APIGateway) proxyToUserService(c *gin.Context) {
	serviceURL := os.Getenv("USER_SERVICE_URL")
	if serviceURL == "" {
		serviceURL = "user-service:8001"
	}
	gw.proxyRequest(c, serviceURL)
}

func (gw *APIGateway) proxyToProductService(c *gin.Context) {
	serviceURL := os.Getenv("PRODUCT_SERVICE_URL")
	if serviceURL == "" {
		serviceURL = "product-service:8002"
	}
	gw.proxyRequest(c, serviceURL)
}

func (gw *APIGateway) proxyRequest(c *gin.Context, targetService string) {
	targetURL := fmt.Sprintf("http://%s%s", targetService, c.Request.URL.Path)
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		gw.logger.WithError(err).Error("Proxy request failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Request processing failed",
		})
		return
	}

	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		gw.logger.WithError(err).Error("Service unavailable")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Service unavailable",
		})
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	c.Status(resp.StatusCode)

	buffer := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			c.Writer.Write(buffer[:n])
		}
		if err != nil {
			break
		}
	}
}

func (gw *APIGateway) Start(port string) error {
	gw.setupRoutes()

	server := &http.Server{
		Addr:    ":" + port,
		Handler: gw.router,
	}

	go func() {
		gw.logger.WithField("port", port).Info("Starting API Gateway")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			gw.logger.WithError(err).Fatal("Server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	gw.logger.Info("Shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}

func main() {
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8000"
	}

	gateway := NewAPIGateway()
	if err := gateway.Start(port); err != nil {
		log.Fatal(err)
	}
}
