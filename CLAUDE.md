# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Automated Ansible AAP/AWX credential rotation using Akeyless rotated secrets. The [custom-producer](https://github.com/Fahmy-Kadiri-akl/custom-producer) handles password and API token rotation on AWX, while Akeyless manages the schedule, encrypted storage, and event notifications. Optionally, Ansible Event-Driven Automation (EDA) receives webhooks to push credential updates immediately.

## Architecture

The system has three layers:

1. **Custom Producer** (external - [Fahmy-Kadiri-akl/custom-producer](https://github.com/Fahmy-Kadiri-akl/custom-producer)) - A single container that rotates credentials across 19+ target systems. For this project, the relevant payload types are `password` and `api_key`. Deployed via `kubernetes/custom-producer/deployment.yaml` using the pre-built image `ghcr.io/fahmy-kadiri-akl/custom-producer/rotator:latest`.

2. **Ansible Playbooks** (`ansible/`) - Two credential sync models:
   - **Push model**: EDA rulebook (`ansible/eda/rulebooks/akeyless-rotation.yml`) listens for Akeyless Event Center webhooks and triggers `update-credential.yml` to update AWX credential objects immediately.
   - **Pull model**: `fetch-credentials.yml` fetches current rotated values from Akeyless and updates AWX credentials on a schedule.

3. **CI/CD Integration** (`cicd/`) - Pipeline scripts that authenticate to Akeyless via certificate auth at runtime, fetch the latest rotated credentials, and use them to launch AWX jobs.

## Authentication

This project uses **certificate-based authentication** to Akeyless (not API key).

- **CI/CD scripts**: Use `AKEYLESS_ACCESS_ID`, `AKEYLESS_CERT_DATA` (base64 cert), and `AKEYLESS_KEY_DATA` (base64 key)
- **Ansible playbooks**: Use `akeyless_access_id`, `akeyless_cert_file`, and `akeyless_key_file`
- **Akeyless CLI**: `akeyless auth --access-type cert --cert-file-name <path> --key-file-name <path>`

## Build and Run Commands

### Custom Producer

The custom producer is not built from this repo. It is pulled as a pre-built image:

```bash
# Deploy to Kubernetes
kubectl apply -f kubernetes/custom-producer/deployment.yaml

# Or run locally with Docker
docker run -p 8080:8080 -e SKIP_AUTH=true ghcr.io/fahmy-kadiri-akl/custom-producer/rotator:latest

# Update to latest version
kubectl rollout restart deployment/rotator -n rotator
```

### Ansible

```bash
# Install required collections
ansible-galaxy collection install -r ansible/collections/requirements.yml

# Run the pull-model credential sync
ansible-playbook ansible/playbooks/fetch-credentials.yml \
  -e akeyless_access_id=p-xxxx \
  -e akeyless_cert_file=/path/to/cert.pem \
  -e akeyless_key_file=/path/to/key.pem \
  -e controller_host=https://ansible.example.com \
  -e controller_username=admin \
  -e controller_password=secret

# Start EDA rulebook (push model)
ansible-rulebook --rulebook ansible/eda/rulebooks/akeyless-rotation.yml \
  -i ansible/inventory/hosts \
  --vars ansible/eda/vars/akeyless.yml
```

### Akeyless Setup

```bash
# Configure Akeyless targets, rotated secrets, and event forwarder
# Requires authenticated akeyless CLI
bash akeyless-setup/setup.sh
```

### Kubernetes

```bash
# Deploy AWX via kustomize
kubectl apply -k kubernetes/awx/

# Deploy custom producer
kubectl apply -f kubernetes/custom-producer/deployment.yaml
```

### Testing

```bash
# End-to-end validation (requires live Akeyless + AWX)
export AKEYLESS_ACCESS_ID=p-xxxx
export AKEYLESS_CERT_DATA=$(base64 -w0 /path/to/cert.pem)
export AKEYLESS_KEY_DATA=$(base64 -w0 /path/to/key.pem)
./cicd/e2e-test.sh

# CI/CD pipeline test
./cicd/pipeline-server-build.sh [server_name] [server_role]
```

### CI/CD Workflows

- **Server Build Pipeline** (`.github/workflows/server-build.yml`) - Manual workflow_dispatch that fetches rotated creds from Akeyless and triggers an AWX job.

## Custom Producer Protocol

The producer handles two payload types, distinguished by the `type` field in the encrypted Akeyless payload:

- **`password`** - Generates a random 24-char password, calls `PATCH /api/v2/users/{id}/` on AWX. Old password is immediately invalidated.
- **`api_key`** - Creates a new personal access token via `POST /api/v2/users/{id}/personal_tokens/`, then revokes the old token (create-before-revoke pattern).

Authentication: The Akeyless Gateway sends a JWT in the `AkeylessCreds` header, validated against `https://auth.akeyless.io/validate-producer-credentials`. Set `SKIP_AUTH=true` for local testing.

## Key Environment Variables

| Variable | Used By | Purpose |
|----------|---------|---------|
| `PORT` | custom-producer | HTTP listen port (default: 8080) |
| `AKEYLESS_ACCESS_ID` | custom-producer, cicd scripts | Gateway access ID / cert auth method access ID |
| `SKIP_AUTH` | custom-producer | Disable auth validation for testing |
| `AKEYLESS_CERT_DATA` | cicd scripts | Base64-encoded client certificate (PEM) |
| `AKEYLESS_KEY_DATA` | cicd scripts | Base64-encoded client private key (PEM) |
| `AWX_URL` | cicd scripts | Ansible AWX controller URL |

## Code Conventions

- **Ansible**: Uses `akeyless.secrets_management` and `ansible.controller` collections. All playbooks run on `localhost` with `connection: local`.
- **No em dashes**: Use hyphens (-) not em dashes in all text content.
