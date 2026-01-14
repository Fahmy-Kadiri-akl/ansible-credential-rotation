# Akeyless Gateway Kubernetes Deployment

This directory contains the Kubernetes manifests and Helm values for deploying Akeyless Gateway on the Titan server.

## Directory Structure

```
kubernetes/
├── argocd/
│   └── akeyless-gateway-app.yaml    # ArgoCD Application manifest
└── infra-security/
    ├── namespace.yaml                # Namespace definition
    ├── akeyless-gateway-values.yaml  # Helm values for the gateway
    ├── secrets.yaml                  # Credentials secret (UID token)
    ├── certificate.yaml              # TLS certificate (cert-manager)
    ├── servicemonitor.yaml           # Prometheus ServiceMonitor
    └── metrics-config.yaml           # OpenTelemetry metrics config
```

## Prerequisites

1. **Kubernetes cluster** with the following installed:
   - nginx-ingress controller
   - cert-manager with `mkcert-ca-issuer` ClusterIssuer
   - ArgoCD in the `cicd` namespace
   - Prometheus Operator (for ServiceMonitor)

2. **DNS**: `akeyless.fklab.local` pointing to the ingress controller

3. **Akeyless Universal Identity Token**: Generate with:
   ```bash
   akeyless uid-generate-token --uid-token-type=dynamic
   ```

## Deployment Steps

### Manual Deployment

1. Apply the namespace and prerequisites:
   ```bash
   kubectl apply -f kubernetes/infra-security/namespace.yaml
   kubectl apply -f kubernetes/infra-security/certificate.yaml
   kubectl apply -f kubernetes/infra-security/servicemonitor.yaml
   kubectl apply -f kubernetes/infra-security/metrics-config.yaml
   ```

2. Create the credentials secret with your UID token:
   ```bash
   kubectl create secret generic akeyless-gateway-credentials \
     --namespace infra-security \
     --from-literal=gateway-uid-token="YOUR_UID_TOKEN"
   ```

3. Deploy the ArgoCD Application:
   ```bash
   kubectl apply -f kubernetes/argocd/akeyless-gateway-app.yaml
   ```

### GitLab CI/CD Deployment

1. Set the `KUBECONFIG_DATA` CI/CD variable (base64 encoded kubeconfig)
2. Push to main branch
3. Manually trigger the deploy jobs

## Configuration

### Gateway Access
- **Access ID**: `p-qzj686col15oum` (Universal Identity)
- **Admin Access**: SAML with `p-4hr352lth7m5sm`

### Endpoints
- **Gateway URL**: https://akeyless.fklab.local
- **Metrics**: Port 8200 at `/metrics`

### Monitoring
- Prometheus scrapes metrics via ServiceMonitor
- Metrics are forwarded to OpenObserve at `https://o2.fklab.local`

## Troubleshooting

Check ArgoCD application status:
```bash
kubectl get application akeyless-gateway -n cicd
argocd app get akeyless-gateway
```

Check gateway pods:
```bash
kubectl get pods -n infra-security
kubectl logs -n infra-security -l app=akeyless-gateway
```
