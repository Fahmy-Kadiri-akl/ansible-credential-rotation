#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Akeyless Cred Server Deployment ===${NC}"

# Configuration
NAMESPACE="cred-server"
IMAGE_NAME="cred-server"
IMAGE_TAG="latest"
REGISTRY="localhost:32000"  # MicroK8s built-in registry

# Check if we're on the titan server
if [[ "$(hostname)" != "titan" ]]; then
    echo -e "${YELLOW}Not on titan server. Syncing files...${NC}"
    # Sync files to titan
    rsync -avz --exclude='.git' \
        "$(dirname "$0")/" \
        titan:/tmp/cred-server/

    # Run deploy on titan
    ssh titan "cd /tmp/cred-server && ./deploy.sh"
    exit $?
fi

echo -e "${GREEN}Running on titan server${NC}"

# Navigate to script directory
cd "$(dirname "$0")"

# Step 1: Generate go.sum
echo -e "${YELLOW}Step 1: Downloading Go dependencies...${NC}"
go mod tidy

# Step 2: Build Docker image
echo -e "${YELLOW}Step 2: Building Docker image...${NC}"
docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .

# Step 3: Tag and push to local registry
echo -e "${YELLOW}Step 3: Pushing to local registry...${NC}"
docker tag ${IMAGE_NAME}:${IMAGE_TAG} ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}
docker push ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}

# Step 4: Apply Kubernetes manifests
echo -e "${YELLOW}Step 4: Applying Kubernetes manifests...${NC}"

# Create namespace first
kubectl apply -f kubernetes/namespace.yaml

# Apply secrets
kubectl apply -f kubernetes/secrets.yaml

# Update deployment to use registry image
sed "s|image: cred-server:latest|image: ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}|g" \
    kubernetes/deployment.yaml | kubectl apply -f -

# Step 5: Wait for deployment
echo -e "${YELLOW}Step 5: Waiting for deployment to be ready...${NC}"
kubectl rollout status deployment/${IMAGE_NAME} -n ${NAMESPACE} --timeout=120s

# Step 6: Show status
echo -e "${GREEN}=== Deployment Complete ===${NC}"
echo ""
kubectl get pods -n ${NAMESPACE}
echo ""
kubectl get svc -n ${NAMESPACE}
echo ""
kubectl get ingress -n ${NAMESPACE}
echo ""

# Get the service URL
echo -e "${GREEN}Service available at:${NC}"
echo "  - Internal: http://cred-server.cred-server.svc.cluster.local:8080"
echo "  - External: https://cred-server.fklab.local"
echo ""
echo -e "${YELLOW}To register with Akeyless Gateway, create a Dynamic Secret with:${NC}"
echo "  Create Sync URL: https://cred-server.fklab.local/sync/create"
echo "  Revoke Sync URL: https://cred-server.fklab.local/sync/revoke"
echo "  Rotate Sync URL: https://cred-server.fklab.local/sync/rotate"
