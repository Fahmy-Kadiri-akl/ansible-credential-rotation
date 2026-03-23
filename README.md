# Akeyless Infrastructure & Tooling

Tooling and infrastructure for integrating with the [Akeyless](https://www.akeyless.io/) identity security platform.

## Components

### cred-server

Go service that automatically discovers credential management endpoints from any REST API and creates Akeyless dynamic/rotated secrets via custom producers. See [cred-server/README.md](cred-server/README.md).

### kubernetes

Akeyless Gateway deployment manifests for Kubernetes, managed via ArgoCD and Helm. Includes TLS, Prometheus monitoring, and OpenTelemetry metrics. See [kubernetes/README.md](kubernetes/README.md).

### sync-folder-to-usc.py

CLI tool to sync all static/rotated secrets under an Akeyless folder to a Universal Secret Connector (USC). Supports recursive folder traversal, drift detection, and dry-run mode.

```bash
# Sync all secrets under a folder to a USC
./sync-folder-to-usc.py --folder /prod/secrets --usc my-aws-usc

# Dry run with recursion
./sync-folder-to-usc.py --folder /prod/secrets --usc my-aws-usc --dry-run --recursive true

# Check for drift between Akeyless and remote USC values
./sync-folder-to-usc.py --folder /prod/secrets --usc my-aws-usc --check-drift
```

See [sync-folder-to-usc.conf.example](sync-folder-to-usc.conf.example) for configuration options.

## License

MIT
