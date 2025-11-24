package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type APIGateway struct {
	router *gin.Engine
	logger *logrus.Logger
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services"`
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

	return &APIGateway{
		router: router,
		logger: logger,
	}
}

func (gw *APIGateway) setupRoutes() {
	v1 := gw.router.Group("/api/v1")

	// Health check endpoint
	gw.router.GET("/health", gw.healthCheck)

	// User service proxy routes
	userGroup := v1.Group("/users")
	userGroup.POST("/register", gw.proxyToUserService)
	userGroup.POST("/login", gw.proxyToUserService)
	userGroup.GET("/profile", gw.authMiddleware(), gw.proxyToUserService)

	// Product service proxy routes
	productGroup := v1.Group("/products")
	productGroup.GET("", gw.proxyToProductService)
	productGroup.POST("", gw.authMiddleware(), gw.proxyToProductService)
	productGroup.GET("/:id", gw.proxyToProductService)
	productGroup.PUT("/:id", gw.authMiddleware(), gw.proxyToProductService)
	productGroup.DELETE("/:id", gw.authMiddleware(), gw.proxyToProductService)
}

func (gw *APIGateway) healthCheck(c *gin.Context) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Services: map[string]string{
			"user-service":    gw.checkServiceHealth("user-service:8001"),
			"product-service": gw.checkServiceHealth("product-service:8002"),
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
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// TODO: Implement JWT token validation
		// For now, accept any non-empty token
		if len(token) < 10 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	})
}

func (gw *APIGateway) proxyToUserService(c *gin.Context) {
	gw.proxyRequest(c, "user-service:8001")
}

func (gw *APIGateway) proxyToProductService(c *gin.Context) {
	gw.proxyRequest(c, "product-service:8002")
}

func (gw *APIGateway) proxyRequest(c *gin.Context, targetService string) {
	// Simple proxy implementation
	// In production, use a proper reverse proxy library
	targetURL := fmt.Sprintf("http://%s%s", targetService, c.Request.URL.Path)
	
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		gw.logger.WithError(err).Error("Failed to create proxy request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Copy headers
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		gw.logger.WithError(err).Error("Failed to proxy request")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service unavailable"})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	c.Status(resp.StatusCode)
	
	// Copy response body
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
		gw.logger.WithField("port", port).Info("Starting API Gateway server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			gw.logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	gw.logger.Info("Shutting down API Gateway server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		gw.logger.WithError(err).Fatal("Server forced to shutdown")
	}

	gw.logger.Info("API Gateway server exited")
	return nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	gateway := NewAPIGateway()
	if err := gateway.Start(port); err != nil {
		log.Fatal(err)
	}
}