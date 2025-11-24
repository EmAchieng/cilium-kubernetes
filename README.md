# Enterprise-Grade Microservices Platform

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io/)
[![Cilium](https://img.shields.io/badge/Cilium-eBPF-F8C517?style=flat&logo=cilium)](https://cilium.io/)

## 🏗️ Architecture Overview

A production-ready microservices platform engineered for scalability, security, and observability. Built with modern cloud-native technologies and enterprise best practices, this platform demonstrates advanced distributed systems design patterns.

### System Architecture

```
┌─────────────────┐
│   API Gateway   │  ← Single entry point with JWT authentication
│   (Port 8000)   │
└────────┬────────┘
         │
    ┌────┴─────────────────┐
    │                      │
┌───▼──────────┐  ┌───────▼──────┐
│ User Service │  │Product Service│
│ (Port 8001)  │  │  (Port 8002)  │
└──────┬───────┘  └───────┬───────┘
       │                  │
    ┌──▼──────────────────▼───┐
    │   PostgreSQL Database   │
    └─────────────────────────┘
```

## 🚀 Core Components

### API Gateway
- **Purpose**: Unified entry point for all client requests with intelligent routing
- **Features**:
  - JWT-based authentication and authorization
  - Request/response transformation and validation
  - Service discovery and load balancing
  - Structured error handling with error codes
  - CORS configuration for cross-origin requests

### User Service
- **Purpose**: Complete user lifecycle management and authentication
- **Features**:
  - Secure user registration with input validation
  - Bcrypt password hashing (cost factor: 10)
  - JWT token generation with expiration
  - Duplicate username/email prevention
  - Database health monitoring
  - Connection pooling for optimal performance

### Product Service
- **Purpose**: Full-featured product catalog management
- **Features**:
  - Complete CRUD operations for products
  - Advanced search and filtering capabilities
  - Pagination support for large datasets
  - Category-based organization
  - Stock level tracking
  - Comprehensive input validation
  - Timestamp tracking (created/updated)

## 💡 Technology Stack

### Backend Services
- **Language**: Go 1.21+
- **HTTP Framework**: Gin (API Gateway), Gorilla Mux (Services)
- **Database Driver**: pgx v4 with connection pooling
- **Authentication**: JWT (golang-jwt/jwt/v4)
- **Password Security**: bcrypt
- **Logging**: Zerolog (User/Product), Logrus (Gateway)

### Infrastructure
- **Database**: PostgreSQL with connection pooling
- **Container Orchestration**: Kubernetes
- **Networking**: Cilium CNI with eBPF
- **Observability**: Hubble for network visibility
- **Security**: Zero-trust network policies

## 🔐 Security Features

- **JWT Authentication**: Token-based authentication with expiration
- **Password Hashing**: Bcrypt with configurable cost factor
- **Input Validation**: Comprehensive sanitization and validation
- **SQL Injection Prevention**: Parameterized queries throughout
- **Environment-based Secrets**: No hard-coded credentials
- **Structured Error Responses**: Prevents information leakage
- **Connection Security**: Configurable SSL/TLS for database connections

## 📦 Installation & Setup

### Prerequisites
```bash
- Go 1.21 or higher
- PostgreSQL 12+
- Docker (optional)
- Kubernetes cluster (for production deployment)
```

### Environment Configuration

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Configure your environment variables in `.env`:
```bash
# API Gateway
PORT=8000
JWT_SECRET=your-secure-secret-key-here

# User Service
USER_SERVICE_PORT=8001
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your-password
DB_NAME=userdb
DB_SSLMODE=disable

# Product Service
PRODUCT_SERVICE_PORT=8002
PRODUCT_DB_HOST=localhost
PRODUCT_DB_PORT=5432
PRODUCT_DB_USER=postgres
PRODUCT_DB_PASSWORD=your-password
PRODUCT_DB_NAME=productdb
PRODUCT_DB_SSLMODE=disable
```

### Database Setup

Create the required databases:
```sql
CREATE DATABASE userdb;
CREATE DATABASE productdb;
```

The services will automatically create required tables on startup.

### Running the Services

#### Option 1: Run Locally

**Terminal 1 - User Service:**
```bash
cd user-service
go mod download
export $(cat ../.env | xargs)
go run main.go
```

**Terminal 2 - Product Service:**
```bash
cd product-service
go mod download
export $(cat ../.env | xargs)
go run main.go
```

**Terminal 3 - API Gateway:**
```bash
cd api-gateway
go mod download
export $(cat ../.env | xargs)
go run main.go
```

#### Option 2: Build and Run Binaries

```bash
# Build all services
cd api-gateway && go build -o ../bin/api-gateway
cd ../user-service && go build -o ../bin/user-service
cd ../product-service && go build -o ../bin/product-service

# Run services
./bin/user-service &
./bin/product-service &
./bin/api-gateway
```

## 📚 API Documentation

### Authentication Endpoints

#### Register User
```http
POST /api/v1/users/register
Content-Type: application/json

{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "securepassword123"
}
```

**Response (201 Created):**
```json
{
  "message": "User registered successfully",
  "user_id": 1,
  "username": "johndoe"
}
```

#### Login
```http
POST /api/v1/users/login
Content-Type: application/json

{
  "username": "johndoe",
  "password": "securepassword123"
}
```

**Response (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "username": "johndoe",
  "expires_at": 1700000000
}
```

### Product Endpoints

#### List Products (with pagination and search)
```http
GET /api/v1/products?page=1&page_size=10&search=laptop&category=electronics
```

**Response (200 OK):**
```json
{
  "products": [
    {
      "id": 1,
      "name": "Gaming Laptop",
      "description": "High-performance gaming laptop",
      "category": "electronics",
      "price": 1299.99,
      "stock": 15,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total_items": 1,
    "total_pages": 1
  }
}
```

#### Get Product by ID
```http
GET /api/v1/products/{id}
```

#### Create Product (Requires Authentication)
```http
POST /api/v1/products
Authorization: Bearer {jwt_token}
Content-Type: application/json

{
  "name": "Wireless Mouse",
  "description": "Ergonomic wireless mouse",
  "category": "accessories",
  "price": 29.99,
  "stock": 100
}
```

#### Update Product (Requires Authentication)
```http
PUT /api/v1/products/{id}
Authorization: Bearer {jwt_token}
Content-Type: application/json

{
  "price": 24.99,
  "stock": 150
}
```

#### Delete Product (Requires Authentication)
```http
DELETE /api/v1/products/{id}
Authorization: Bearer {jwt_token}
```

### Health Check Endpoints

```http
GET /health                    # API Gateway health
GET /api/v1/users/health      # User Service health (proxied)
GET /api/v1/products/health   # Product Service health (proxied)
```

## 🔧 Development

### Code Quality Standards
- Comprehensive error handling with structured responses
- Input validation and sanitization
- Connection pooling for database efficiency
- Graceful shutdown handling
- Structured logging with contextual fields
- No hard-coded credentials or secrets

### Testing
```bash
# Run tests for each service
cd user-service && go test ./...
cd product-service && go test ./...
cd api-gateway && go test ./...
```

### Building
```bash
# Build all services
make build

# Or build individually
go build -o bin/api-gateway ./api-gateway
go build -o bin/user-service ./user-service
go build -o bin/product-service ./product-service
```

## 🌐 Deployment

### Kubernetes Deployment

The platform is designed for Kubernetes deployment with:
- Horizontal Pod Autoscaling
- Service mesh integration (Cilium)
- ConfigMaps for configuration
- Secrets for sensitive data
- Network policies for zero-trust security

### Docker Deployment

```bash
docker-compose up -d
```

## 📊 Monitoring & Observability

- **Structured Logging**: JSON-formatted logs with contextual information
- **Health Checks**: Dedicated endpoints for service health monitoring
- **Database Connectivity**: Real-time database health monitoring
- **Network Observability**: Hubble integration for eBPF-based visibility

## 🤝 Contributing

Contributions are welcome! Please ensure:
- Code follows Go best practices
- All tests pass
- Documentation is updated
- Commits are well-described

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🎯 Production Readiness Checklist

- [x] JWT authentication with expiration
- [x] Password hashing with bcrypt
- [x] Environment-based configuration
- [x] Connection pooling
- [x] Input validation and sanitization
- [x] Structured error handling
- [x] Health check endpoints
- [x] Graceful shutdown
- [x] Comprehensive logging
- [x] Database migrations
- [x] API documentation
- [x] Zero hard-coded secrets

---

**Built with ❤️ for production deployment and professional portfolio demonstration**