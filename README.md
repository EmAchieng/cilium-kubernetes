## Development

...

### Running with Docker

```bash
# Build all Docker images for microservices
make docker

# This will create the following images:
# - cilium-microservices/api-gateway:1.0.0
# - cilium-microservices/user-service:1.0.0
# - cilium-microservices/product-service:1.0.0

# Run individual services with Docker
docker run -p 8000:8000 cilium-microservices/api-gateway:1.0.0
docker run -p 8001:8001 cilium-microservices/user-service:1.0.0
docker run -p 8002:8002 cilium-microservices/product-service:1.0.0
```

...

### Kubernetes Deployment