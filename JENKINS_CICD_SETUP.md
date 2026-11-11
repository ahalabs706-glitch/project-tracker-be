# Docker Compose & Jenkins CI/CD Setup

Dokumentasi untuk setup Docker Compose backend-only dan Jenkins CI/CD pipeline untuk branch `develop` dan `main`.

## 📋 Daftar Isi

- [Struktur File](#struktur-file)
- [Docker Compose](#docker-compose)
- [Jenkins Setup](#jenkins-setup)
- [CI Pipeline](#ci-pipeline)
- [CD Pipeline](#cd-pipeline)
- [Jenkins Credentials](#jenkins-credentials)
- [Deployment Flow](#deployment-flow)

---

## 📁 Struktur File

```
project-tracker-be/
├── docker-compose.yml           # Backend only compose (dev, testing)
├── docker-compose.prod.yml      # Production compose (existing)
├── Jenkinsfile.ci               # CI Pipeline (testing, linting, security scan)
├── Jenkinsfile.cd               # CD Pipeline (build, push, deploy)
├── .env.example                 # Environment variables template
├── Dockerfile                   # Application image
└── README.md                    # This file
```

---

## 🐳 Docker Compose

### Backend-Only Setup (`docker-compose.yml`)

File `docker-compose.yml` hanya berisi service backend tanpa frontend:

**Services:**
- **postgres**: Database PostgreSQL 16-alpine
- **redis**: Cache Redis 7-alpine
- **db-setup**: Database initialization & migration
- **api**: Main API service (projectflow-api)
- **worker**: Background job worker

**Network:**
- `projectflow-network`: Custom bridge network untuk komunikasi antar service

**Volumes:**
- `postgres_data`: Persistent PostgreSQL data
- `redis_data`: Persistent Redis data

### Environment Variables

```bash
# Database
POSTGRES_DB=projectflow
POSTGRES_USER=projectflow
POSTGRES_PASSWORD=projectflow_dev
POSTGRES_PORT=5432

# Redis
REDIS_PASSWORD=local-redis-development-only
REDIS_PORT=6379

# Application
APP_ENV=development
APP_PORT=4000
APP_TIMEZONE=Asia/Jakarta
DATABASE_URL=postgres://projectflow:projectflow_dev@postgres:5432/projectflow?sslmode=disable
WEB_ORIGIN=http://127.0.0.1:5175

# JWT Secrets
JWT_ACCESS_SECRET=local-development-only-change-this-secret
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h

# SMTP (Optional)
SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=
```

### Memulai Development

```bash
# Clone repository
git clone <repository-url>
cd project-tracker-be

# Setup environment
cp .env.example .env

# Start services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f api

# Stop services
docker-compose down

# Stop services dan hapus volume
docker-compose down -v
```

---

## 🔧 Jenkins Setup

### Prerequisites

- Jenkins 2.350+ dengan agent yang memiliki:
  - Docker & Docker Compose
  - Go 1.26+ for lint and tests (see `go.mod` for the minimum version)
  - Git
  - SSH client
  - `gosec`, `gofmt`, `trivy` (optional untuk security scan)

### Plugin yang Diperlukan

```
- Pipeline
- Blue Ocean
- Git
- Docker
- SSH Agent
- Timestamper
- Log Parser
```

### Jenkins Credentials Setup

Buat credentials berikut di Jenkins:

#### 1. Docker Registry Credentials

**ID:** `DOCKER_REGISTRY_CREDENTIALS`
- Username: Docker Hub username
- Password: Docker Hub token/password

**ID:** `DOCKER_DEV_USERNAME`
- Type: Secret text
- Secret: Username untuk dev registry

**ID:** `DOCKER_DEV_PASSWORD`
- Type: Secret text
- Secret: Password/token untuk dev registry

**ID:** `DOCKER_PROD_USERNAME`
- Type: Secret text
- Secret: Username untuk prod registry

**ID:** `DOCKER_PROD_PASSWORD`
- Type: Secret text
- Secret: Password/token untuk prod registry

#### 2. Docker Repository Credentials

**ID:** `API_DOCKER_DEV`
- Type: Secret text
- Secret: `docker-username/projectflow-api-dev`

**ID:** `API_DOCKER_PROD`
- Type: Secret text
- Secret: `docker-username/projectflow-api`

#### 3. SSH Deployment Credentials

**ID:** `SSH_PRIVATE_KEY`
- Type: Secret text
- Secret: Base64-encoded private SSH key
  ```bash
  cat ~/.ssh/id_rsa | base64 | tr -d '\n'
  ```

#### 4. Deployment Server Credentials

**ID:** `DEPLOYMENT_SERVER_DEV`
- Type: Secret text
- Secret: `deployer@dev-server.example.com` atau IP address

**ID:** `DEPLOYMENT_SERVER_PROD`
- Type: Secret text
- Secret: `deployer@prod-server.example.com` atau IP address

### Membuat Jenkins Jobs

#### Job 1: CI Pipeline

1. Buat Pipeline job baru: `projectflow-backend-ci`
2. Configure:
   - **Branch Sources**: GitHub
     - Repository: `<repository-url>`
     - Credentials: GitHub credentials
     - Behaviors:
       - Discover branches from origin
       - Filter by name: `develop|main`
   
   - **Build Configuration**:
     - Mode: by Jenkinsfile
     - Script Path: `project-tracker-be/Jenkinsfile.ci`
     - Lightweight checkout: unchecked

3. **Additional Options**:
   - Suppress automatic SCM triggering: unchecked
   - Trigger builds with PRs and pushes: checked

#### Job 2: CD Pipeline

1. Buat Pipeline job baru: `projectflow-backend-cd`
2. Configure:
   - **Branch Sources**: Sama seperti CI
   - **Build Configuration**:
     - Mode: by Jenkinsfile
     - Script Path: `project-tracker-be/Jenkinsfile.cd`
   
3. **Trigger**:
   - Event: `push`
   - Branch filter: `develop|main` (regex)

---

## 🔄 CI Pipeline (`Jenkinsfile.ci`)

### Stages

1. **Checkout**
   - Clone repository
   - Extract commit info (hash, author, message)

2. **Validate Branch**
   - Verifikasi branch adalah `develop` atau `main`
   - Tentukan environment (dev/prod)

3. **Setup Go**
   - Download dependencies
   - Verify module integrity

4. **Lint & Format Check**
   - Run `go vet`
   - Check code formatting dengan `gofmt`

5. **Security Scan**
   - Run `gosec` untuk security vulnerabilities

6. **Unit Tests**
   - Run `go test` dengan coverage report

7. **Build Docker Image**
   - Build image dengan tags: `BUILD_NUMBER`, `COMMIT_HASH`, `latest`

8. **Scan Docker Image**
   - Run `trivy` untuk vulnerability scanning

9. **Integration Test**
   - Start Docker Compose stack
   - Test API health endpoint
   - Verify database connectivity
   - Cleanup stack

10. **Prepare Metadata**
    - Simpan image metadata ke `image.properties`

### Output

- Docker image siap untuk push ke registry
- Coverage report
- Security scan results
- Health check verification

### Failure Handling

Pipeline akan fail jika:
- Code formatting tidak valid
- Security vulnerabilities ditemukan
- Tests tidak pass
- API health check gagal

---

## 🚀 CD Pipeline (`Jenkinsfile.cd`)

### Stages

1. **Checkout**
   - Clone repository dengan metadata extraction

2. **Validate Branch & Environment**
   - Tentukan environment berdasarkan branch
   - Setup Docker registry credentials

3. **Load Metadata**
   - Load dari `image.properties` (dari CI) atau fallback ke BUILD_NUMBER

4. **Docker Login**
   - Authenticate ke Docker registry

5. **Build Image**
   - Build Docker image dengan build args (BUILD_NUMBER, COMMIT_HASH, timestamp)

6. **Scan Image**
   - Run `trivy` untuk vulnerability scanning

7. **Push to Registry**
   - Push ke Docker Hub dengan tags:
     - `BUILD_NUMBER`
     - `COMMIT_HASH`
     - `latest`

8. **Update Deployment Config**
   - Generate `deployment-manifest.env` dengan configuration

9. **Deploy to Dev** (jika branch = `develop`)
   - Deploy ke development server via SSH
   - Update docker-compose
   - Health check

10. **Deploy to Production** (jika branch = `main`)
    - **MANUAL APPROVAL REQUIRED**
    - Tampilkan confirmation dialog
    - Create database backup
    - Deploy ke production server
    - Health check

11. **Health Check**
    - Verify API health
    - Verify database connectivity
    - Verify Redis connectivity

### Manual Approval

Untuk production deployment (`main` branch):
1. Pipeline menampilkan approval dialog
2. User review: commit hash, author, message
3. Click "Deploy to Production" untuk melanjutkan
4. Atau click "Abort" untuk cancel

### Deployment via SSH

SSH ke server deployment:
```bash
ssh deployer@deployment-server
cd /opt/projectflow/backend
docker-compose up -d
```

---

## 📊 Deployment Flow

### Branch: `develop`

```
Code Push (develop)
    ↓
CI Pipeline (Jenkinsfile.ci)
├─ Checkout
├─ Validate & Setup
├─ Lint, Test, Security Scan
├─ Build Docker Image
├─ Integration Test
└─ Prepare Metadata (image.properties)
    ↓
CD Pipeline (Jenkinsfile.cd)
├─ Checkout
├─ Load Metadata
├─ Build & Scan Image
├─ Push to Dev Registry
└─ Auto Deploy to Dev Server
    ↓
Dev Environment Running
└─ Health Check Verified
```

### Branch: `main` (Merge Required)

```
Code Push (main) [MUST BE MERGE COMMIT]
    ↓
CI Pipeline (Jenkinsfile.ci)
├─ Checkout
├─ Validate & Setup
├─ Lint, Test, Security Scan
├─ Build Docker Image
├─ Integration Test
└─ Prepare Metadata
    ↓
CD Pipeline (Jenkinsfile.cd)
├─ Checkout
├─ Load Metadata
├─ Build & Scan Image
├─ Push to Prod Registry
└─ ⏸️ MANUAL APPROVAL REQUIRED
    │
    ├─ Approved ✓
    │   └─ Create DB Backup
    │   └─ Deploy to Prod
    │   └─ Health Check
    │   └─ Prod Environment Running
    │
    └─ Rejected ✗
        └─ Pipeline Aborted
```

---

## 📝 Git Workflow dengan CI/CD

### 1. Feature Development

```bash
# Create feature branch
git checkout -b feature/new-feature

# Make changes and commit
git add .
git commit -m "Add new feature"

# Push to remote
git push origin feature/new-feature
```

### 2. Create Pull Request

- Create PR ke `develop`
- Add description, reviewers
- Await reviews

### 3. Merge to Develop

```bash
# Merge PR ke develop
git checkout develop
git pull origin develop
git merge --no-ff feature/new-feature
git push origin develop
```

**Trigger:** CI Pipeline runs automatically

**Result:** Tested image pushed to dev registry
**Deploy:** Auto deploy ke dev server

### 4. Release to Main

```bash
# Create release branch
git checkout -b release/v1.0.0

# Version bump, changelog update
git commit -am "Bump version to 1.0.0"

# Merge to main with merge commit
git checkout main
git pull origin main
git merge --no-ff release/v1.0.0
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin main --tags
```

**Trigger:** CI Pipeline runs
**Result:** Image pushed to prod registry
**Deploy:** ⏸️ Waiting for manual approval

### 5. Approve Production Deployment

1. Go to CD Pipeline in Jenkins
2. Click on build number
3. Click "Proceed" on approval dialog
4. Review deployment details
5. Confirm deployment

**Result:** Deployed to production with backup

---

## 🔒 Security Best Practices

### 1. Secrets Management

- Gunakan Jenkins credentials untuk sensitive data
- Never commit `.env` files dengan actual secrets
- Use `.env.example` untuk template

### 2. SSH Key Management

- Generate dedicated SSH key untuk deployment
- Use restricted SSH user `deployer` dengan limited permissions
- Rotate keys regularly

### 3. Docker Registry

- Use private Docker Hub repository
- Use strong, unique registry credentials
- Implement registry authentication

### 4. Database Backups

- Auto backup sebelum production deployment
- Store backups di secure location
- Test restore process regularly

### 5. Image Scanning

- Run `trivy` sebelum push ke registry
- Address HIGH & CRITICAL vulnerabilities
- Keep base images updated

---

## 🐛 Troubleshooting

### Pipeline gagal di stage "Setup Go"

```bash
# Cek Go version
go version

# Cek module dependencies
go mod download
go mod verify
```

### Docker build gagal

```bash
# Cek Dockerfile
docker build -t test:latest .

# View build logs
docker build --progress=plain -t test:latest .
```

### SSH deployment gagal

```bash
# Test SSH connection
ssh -i ~/.ssh/deploy_key deployer@deployment-server "echo 'SSH OK'"

# Check SSH key permissions
ls -la ~/.ssh/deploy_key  # Should be 600
```

### Service health check fails

```bash
# Check service logs
docker-compose logs api
docker-compose logs postgres
docker-compose logs redis

# Test connectivity
curl http://localhost:4000/health
```

---

## 📚 Referensi

- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Jenkins Pipeline Documentation](https://www.jenkins.io/doc/book/pipeline/)
- [Go Testing](https://golang.org/pkg/testing/)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)

---

## ✅ Checklist Deployment

- [ ] Credentials configured di Jenkins
- [ ] SSH keys setup di deployment servers
- [ ] Docker registry accessible
- [ ] Database backups configured
- [ ] Health check endpoints working
- [ ] Load balancer updated (if applicable)
- [ ] Monitoring & alerts configured
- [ ] Logging configured
- [ ] Runbook created untuk incident response

---

**Last Updated:** 2024
**Version:** 1.0.0
