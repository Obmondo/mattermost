# Mattermost Docker Build Guide

This directory contains the Dockerfiles and configurations required to build Mattermost server images. For Kubernetes deployments using Helm, you will primarily need the **Runtime Image**.

---

## 1. Build Environment Image
This image contains all necessary tools (Go, Node.js, compilers) to build Mattermost from source. It is used for compilation and local development.

### How to Build
Run this from the **project root**:
```bash
docker build --platform linux/amd64 -t mattermost-build-server:local -f build/Dockerfile.buildenv .
```
*Note: Using `--platform linux/amd64` is recommended on Apple Silicon (M1/M2/M3) because the base image is optimized for amd64.*

---

## 2. Runtime Image (Production/K8s)
This is the lightweight image used for deployment. By default, it downloads the latest stable Mattermost Enterprise package.

### How to Build
Run this from the **project root**:
```bash
docker build -t mattermost-server:local -f build/Dockerfile build/
```

### Build Arguments
You can customize the version by passing the `MM_PACKAGE` build argument:
```bash
docker build \
  --build-arg MM_PACKAGE="https://releases.mattermost.com/9.7.1/mattermost-9.7.1-linux-amd64.tar.gz" \
  -t mattermost-server:9.7.1 \
  -f build/Dockerfile build/
```

---

## 3. Building and Running from Local Source (Quick Start)
If you have made changes to the local server or webapp, use these `make` commands for a streamlined workflow.

### Step A: Build the Local Image
This command compiles the server for Linux, bundles webapp assets, and builds the Docker image.
```bash
make docker-build-local LOCAL_IMAGE_TAG=11.6.0_oidc
```

### Step B: Run Locally with Dependencies
This starts the built image along with all required services (Postgres, Minio, etc.).

1. **Create a `.env` file** in the project root with your OIDC settings:
```bash
# OIDC Configuration
OIDC_ISSUER=https://keycloak-staging.kcm.obmondo.com/auth/realms/staging
OIDC_CLIENT_ID=mattermost
OIDC_CLIENT_SECRET=******
OIDC_REDIRECT_URL=http://localhost:8065/api/v4/auth/oidc/complete

# Enable OIDC Feature
MM_OPENIDSETTINGS_ENABLE=true
```

2. **Start the environment:**
```bash
make docker-run-local
```
- **Access:** http://localhost:8065
- **Logs:** `make docker-logs-local`
- **Stop:** `make docker-stop-local`

---

## 4. Kubernetes Deployment via Helm
When deploying to Kubernetes using the Mattermost Helm chart:

1.  **Tag and Push:** Tag your built image and push it to your container registry (e.g., Docker Hub, ECR, GCR).
    ```bash
    docker tag mattermost-server:local your-registry/mattermost-server:custom
    docker push your-registry/mattermost-server:custom
    ```
2.  **Update Helm Values:** In your `values.yaml` for the Helm chart, update the image repository and tag:
    ```yaml
    image:
      repository: your-registry/mattermost-server
      tag: custom
    ```

---

## 5. Troubleshooting Guide

### Issue: `no match for platform in manifest: not found`
**Symptoms:** Docker fails to pull base images (e.g., `mattermost/golang-bullseye`).
**Cause:** You are building on an ARM-based machine (Apple Silicon), but the base image only supports `linux/amd64`.
**Solution:** Always include the `--platform linux/amd64` flag in your `docker build` command.

### Issue: `COPY failed: file not found in build context`
**Symptoms:** Docker fails at a `COPY` instruction (e.g., `COPY build/passwd` or `COPY dist/mattermost`).
**Cause:**
1.  You are running `docker build` from inside the `build/` directory instead of the project root.
2.  The `dist/mattermost` folder hasn't been created yet.
**Solution:**
- Run all `docker build` commands from the **project root**.
- Ensure you have run `make package-prep` and `make package-general ...` before building `Dockerfile.local`.

### Issue: `make package-prep` fails (Missing webapp)
**Symptoms:** `cp: ../webapp/channels/dist/*: No such file or directory`.
**Cause:** The Makefile expects the `webapp` repository to be located at `../webapp` relative to the `server` directory.
**Solution:** Ensure the `mattermost-webapp` repository is cloned and built (`make dist` inside webapp) in the sibling directory.

### Issue: `curl: (6) Could not resolve host`
**Symptoms:** Docker build fails during the `curl` command inside the Dockerfile.
**Cause:** DNS or networking issues inside the Docker container.
**Solution:**
- Restart Docker Desktop.
- Check if you are behind a proxy (you may need to pass `--build-arg http_proxy=...`).

### Issue: `Permission denied` on Kubernetes
**Symptoms:** The Mattermost pod fails to start with log errors about writing to `/mattermost/data` or `/mattermost/config`.
**Cause:** The image runs as user `mattermost` (UID 2000). If your Kubernetes volume mount is owned by `root`, the app cannot write to it.
**Solution:**
- Use a `securityContext` in your Kubernetes pod spec to set `fsGroup: 2000`.
- Or, manually change ownership of the volume mount points on the host (if possible).
