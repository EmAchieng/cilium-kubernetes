# Cilium Kubernetes Integration

## Overview
Cilium is a networking solution that provides connection security, load balancing, and much more for Kubernetes environments.

## Development
In order to contribute and build Cilium, you need to set up the development environment.

### Prerequisites
- Go 1.16 or higher
- Docker
- Kubernetes cluster (Minikube, EKS, GKE, etc.)

### Docker
Cilium can be built and tested using Docker. Below are the steps to run Cilium in a Docker container:

1. Build the Cilium Docker image:
   ```bash
   make docker-image
   ```

2. Run Cilium in a Docker container:
   ```bash
   docker run --rm -it cilium/cilium:latest
   ```

3. You can also use Docker Compose to run Cilium with other services.

## Kubernetes Deployment
To deploy Cilium in your Kubernetes cluster, follow these instructions:

1. Apply the Cilium CNI manifest:
   ```bash
   kubectl apply -f https://raw.githubusercontent.com/cilium/cilium/master/install/kubernetes/quick-install.yaml
   ```

2. Verify the Cilium pods are running:
   ```bash
   kubectl get pods -n kube-system
   ```

3. Follow the further instructions in the [Cilium documentation](https://docs.cilium.io/en/latest/gettingstarted/).

## Conclusion
Cilium provides powerful networking capabilities for Kubernetes, enabling secure and efficient communication between services.