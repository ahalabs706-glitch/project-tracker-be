# ProjectFlow Backend - Docker Compose & Jenkins CI/CD

Quick reference guide untuk Docker Compose dan Jenkins CI/CD setup backend ProjectFlow.

---

## 🚀 Quick Start

### 1. Local Development dengan Docker Compose

```bash
# Clone dan setup
git clone <repository>
cd project-tracker-be

# Copy environment template
cp .env.example .env

# Start services (backend only)
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f api

# Stop services
docker-compose down

# Stop & remove volumes
docker-compose down -v
```

**Services yang running:**
- 🗄️ PostgreSQL (port 5432)
- 🔴 Redis (port 6379)
- 🔵 API (port 4000)
- 🔧 Worker (background jobs)
- 📦 DB Setup (migrations)

**Access points:**
- API: http://localhost:4000
- Health Check: http://localhost:4000/health

---

## 📁 File Structure

```
project-tracker-be/
├── docker-compose.yml              # Backend only (dev/testing)
├── docker-compose.prod.yml         # Production compose
├── Jenkinsfile.ci                  # CI Pipeline
├── Jenkinsfile.cd                  # CD Pipeline
├── JENKINS_CICD_SETUP.md           # Full setup guide
├── JENKINS_CONFIG.md               # Configuration templates
└── .env.example                    # Environment template
```

---

## 🔄 Git Workflow

### Feature Development

```bash
# 1. Create branch
git checkout -b feature/my-feature

# 2. Make changes & commit
git add .
git commit -m "Add feature description"
git push origin feature/my-feature

# 3. Create PR ke develop
# 4. After merge to develop → CI Pipeline runs → Auto deploy to DEV
```

### Release to Production

```bash
# 1. Merge to main (must be merge commit)
git checkout main
git pull origin main
git merge --no-ff develop
git push origin main

# 2. CI Pipeline runs automatically
# 3. CD Pipeline waits for manual approval in Jenkins
# 4. Review & approve deployment in Jenkins UI
```

---

## 🔐 Jenkins Setup (5 Steps)

### Step 1: Create Credentials

In Jenkins → Manage Credentials → Create:

1. **API_DOCKER_DEV** (Secret text)
   ```
   docker-username/projectflow-api-dev
   ```

2. **API_DOCKER_PROD** (Secret text)
   ```
   docker-username/projectflow-api
   ```

3. **DOCKER_DEV_USERNAME** (Secret text)
   ```
   your-docker-username
   ```

4. **DOCKER_DEV_PASSWORD** (Secret text)
   ```
   your-docker-token
   ```

5. **DOCKER_PROD_USERNAME** (Secret text)
6. **DOCKER_PROD_PASSWORD** (Secret text)

7. **SSH_PRIVATE_KEY** (SSH Key)
   ```bash
   cat ~/.ssh/id_rsa | base64 | tr -d '\n'
   ```

8. **DEPLOYMENT_SERVER_DEV** (Secret text)
   ```
   deployer@dev-server.com
   ```

9. **DEPLOYMENT_SERVER_PROD** (Secret text)
   ```
   deployer@prod-server.com
   ```

### Step 2: Create CI Job

1. New Item → Multibranch Pipeline
2. Name: `projectflow-backend-ci`
3. Branch Sources: GitHub → Your repository
4. Build Configuration: Script Path = `project-tracker-be/Jenkinsfile.ci`
5. Save

### Step 3: Create CD Job

1. New Item → Multibranch Pipeline
2. Name: `projectflow-backend-cd`
3. Branch Sources: GitHub → Your repository
4. Build Configuration: Script Path = `project-tracker-be/Jenkinsfile.cd`
5. Save

### Step 4: Setup GitHub Webhook

1. Repository Settings → Webhooks → Add webhook
2. Payload URL: `https://jenkins.example.com/github-webhook/`
3. Content type: `application/json`
4. Events: Push events, PR events
5. Save

### Step 5: Test Pipeline

```bash
# Push to develop branch
git push origin develop

# Check Jenkins UI:
# 1. CI Pipeline runs (testing, linting, security scan)
# 2. CD Pipeline runs (build, push, auto-deploy to dev)
# 3. Dev environment updated

# Push to main branch
git push origin main

# Check Jenkins UI:
# 1. CI Pipeline runs
# 2. CD Pipeline runs (build, push, wait for approval)
# 3. Click "Proceed" on approval dialog
# 4. Prod environment updated with backup
```

---

## 📊 Pipeline Status

### CI Pipeline (Jenkinsfile.ci)

**What it does:**
- ✅ Code analysis & linting
- ✅ Security scanning (gosec)
- ✅ Unit tests with coverage
- ✅ Docker image build
- ✅ Image vulnerability scan
- ✅ Integration tests
- ✅ Prepare metadata

**Duration:** ~10-15 minutes

**Triggers:** On push/PR to `develop` or `main`

**Output:** Docker image ready + metadata file

### CD Pipeline (Jenkinsfile.cd)

**What it does:**
- ✅ Load build metadata
- ✅ Build Docker image
- ✅ Scan for vulnerabilities
- ✅ Push to registry
- ✅ Deploy to server (auto for dev, manual for prod)
- ✅ Health checks
- ✅ Backup database (prod only)

**Duration:** ~15-20 minutes

**Triggers:** After CI success

**Output:** Running application on dev/prod

---

## 🔍 Monitoring Deployments

### View Pipeline Logs

```bash
# Option 1: Jenkins UI
# Go to job → Build number → Console Output

# Option 2: SSH to deployment server
ssh deployer@deployment-server
cd /opt/projectflow/backend
docker-compose logs -f api
```

### Health Checks

```bash
# Manual health check
curl http://localhost:4000/health

# Check all services
docker-compose ps

# View service logs
docker-compose logs api
docker-compose logs postgres
docker-compose logs redis
```

### Database Backup Location

```bash
# Dev server backups
/backups/projectflow/

# List backups
ls -lh /backups/projectflow/

# Restore from backup
gunzip -c /backups/projectflow/db_backup_*.sql.gz | \
  docker-compose exec -T postgres psql -U projectflow projectflow
```

---

## 🚨 Troubleshooting

### Pipeline stuck in approval?

```bash
# Check Jenkins logs
docker logs -f jenkins

# Go to build page and click "Proceed"
# Or click "Abort" to cancel
```

### Docker image build fails?

```bash
# Check Dockerfile
docker build -t test:latest .

# Check build context
ls -la project-tracker-be/

# Clear Docker cache
docker system prune -a
```

### API won't start?

```bash
# Check logs
docker-compose logs api

# Check environment variables
docker-compose config

# Verify database is healthy
docker-compose logs postgres

# Test database connection
docker-compose exec postgres pg_isready -U projectflow
```

### Deployment server unreachable?

```bash
# Test SSH connection
ssh -i ~/.ssh/deploy_key deployer@deployment-server "echo OK"

# Check SSH key permissions
ls -la ~/.ssh/deploy_key  # Should be 600

# Verify server IP/hostname
ping deployment-server
```

---

## 📋 Environment Variables

### Development `.env`

```bash
APP_ENV=development
APP_PORT=4000
DATABASE_URL=postgres://projectflow:projectflow_dev@postgres:5432/projectflow?sslmode=disable
REDIS_PASSWORD=local-redis-development-only
JWT_ACCESS_SECRET=local-development-only-change-this-secret
WEB_ORIGIN=http://127.0.0.1:5175
```

### Production (set in Jenkins/deployment server)

```bash
APP_ENV=production
DATABASE_URL=postgres://user:pass@prod-db:5432/projectflow
REDIS_PASSWORD=<secure-password>
JWT_ACCESS_SECRET=<secure-secret>
WEB_ORIGIN=https://app.example.com
SMTP_HOST=smtp.example.com
```

---

## 🔐 Security Checklist

- [ ] Docker Hub credentials secured in Jenkins
- [ ] SSH keys configured with restricted permissions
- [ ] Database backups automated & tested
- [ ] HTTPS enabled for deployment servers
- [ ] Firewall rules configured
- [ ] Log rotation setup
- [ ] Monitoring & alerting configured
- [ ] Incident response runbook created

---

## 📚 Full Documentation

- **[JENKINS_CICD_SETUP.md](./JENKINS_CICD_SETUP.md)** - Complete setup guide
- **[JENKINS_CONFIG.md](./JENKINS_CONFIG.md)** - Configuration templates
- **[docker-compose.yml](./docker-compose.yml)** - Backend compose config
- **[Jenkinsfile.ci](./Jenkinsfile.ci)** - CI pipeline
- **[Jenkinsfile.cd](./Jenkinsfile.cd)** - CD pipeline

---

## 💡 Tips

1. **Always test locally first**
   ```bash
   docker-compose up -d
   # Test locally before pushing
   docker-compose down
   ```

2. **Use descriptive commit messages**
   ```bash
   git commit -m "feat: Add user authentication"  # Better
   git commit -m "update"                         # Avoid this
   ```

3. **Monitor deployments**
   - Keep Jenkins tab open during deployment
   - Check logs before marking as complete
   - Verify health checks pass

4. **Database safety**
   - Backups run automatically before prod deploy
   - Always test restore procedure
   - Keep 7+ days of backups

5. **Branch protection**
   - Require PR review before merging to develop
   - Require PR review before merging to main
   - Main branch requires "merge commit" type

---

## 🆘 Need Help?

1. Check logs: `docker-compose logs <service>`
2. Read full guide: `JENKINS_CICD_SETUP.md`
3. Review configuration: `JENKINS_CONFIG.md`
4. Check pipeline output in Jenkins UI
5. SSH to deployment server for more info

---

**Version:** 1.0.0  
**Last Updated:** 2024
