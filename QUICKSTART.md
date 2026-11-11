# Quick Reference Card

## 🚀 Getting Started (Choose Your Path)

### For Developers - Local Setup (20 min)
```bash
cd project-tracker-be
docker-compose up -d
docker-compose ps
curl http://localhost:4000/health
```
👉 **Read:** README_DOCKER_JENKINS.md → Quick Start

### For DevOps - Full Setup (3-4 hours)
1. Read: SOLUTION_SUMMARY.md (overview)
2. Read: JENKINS_CICD_SETUP.md (complete guide)
3. Setup: JENKINS_CONFIG.md (templates)
4. Deploy: DEPLOYMENT_CHECKLIST.md
👉 **Read:** INDEX.md → For DevOps Engineers

### For SRE - Safe Deployments (30 min review)
👉 **Read:** DEPLOYMENT_CHECKLIST.md

### For Jenkins Admin - Configuration
👉 **Read:** JENKINS_CONFIG.md

---

## 📁 Files Overview

| File | Purpose | Size |
|------|---------|------|
| **docker-compose.yml** | Backend services | 5 KB |
| **Jenkinsfile.ci** | CI Pipeline (10 stages) | 14 KB |
| **Jenkinsfile.cd** | CD Pipeline (11 stages) | 22 KB |
| **INDEX.md** | Documentation index | 11 KB |
| **README_DOCKER_JENKINS.md** | ⭐ Quick start guide | 9 KB |
| **JENKINS_CICD_SETUP.md** | ⭐ Complete setup | 13 KB |
| **JENKINS_CONFIG.md** | Configuration templates | 10 KB |
| **DEPLOYMENT_CHECKLIST.md** | Deployment runbook | 11 KB |
| **GITHUB_BRANCH_PROTECTION.md** | GitHub config | 10 KB |
| **SOLUTION_SUMMARY.md** | Project overview | 11 KB |

---

## 🎯 Quick Commands

### Local Testing
```bash
docker-compose up -d          # Start
docker-compose ps             # Status
curl localhost:4000/health    # Health check
docker-compose logs -f api    # View logs
docker-compose down -v        # Stop & clean
```

### Git Workflow
```bash
# Feature branch
git checkout -b feature/name
git push origin feature/name
# Create PR → get approval → merge to develop

# Release to prod
git checkout main
git merge --no-ff develop
git tag v1.0.0
git push origin main --tags
# CI runs → CD waits → approve in Jenkins
```

### Jenkins
```
Jenkins URL: https://jenkins.example.com
Pipeline 1:  projectflow-backend-ci
Pipeline 2:  projectflow-backend-cd
```

---

## ✅ Checklist: Get Production Ready

- [ ] Docker Compose tested locally
- [ ] Jenkins server running
- [ ] 9 credentials configured in Jenkins
- [ ] CI job created (multibranch)
- [ ] CD job created (multibranch)
- [ ] GitHub webhook configured
- [ ] Branch protection rules set
- [ ] SSH keys deployed to servers
- [ ] Test: push to develop (should auto-deploy)
- [ ] Test: push to main (should wait for approval)

---

## 🆘 Common Issues

| Issue | Solution |
|-------|----------|
| API won't start | `docker-compose logs api` |
| Database error | `docker-compose exec postgres pg_isready` |
| Jenkins not triggering | Check webhook in GitHub settings |
| Deployment fails | Check DEPLOYMENT_CHECKLIST.md → Incident Response |
| Can't merge to main | Ensure: 2 approvals, merge commit, status checks pass |

---

## 📞 Need Help?

| Question | Answer |
|----------|--------|
| How to setup? | READ: README_DOCKER_JENKINS.md (Quick Start) |
| How Jenkins works? | READ: JENKINS_CICD_SETUP.md |
| How to deploy safely? | READ: DEPLOYMENT_CHECKLIST.md |
| How to configure? | READ: JENKINS_CONFIG.md |
| Full documentation? | READ: INDEX.md |

---

## 🔄 Deployment Flow

```
develop branch push
    ↓
CI Pipeline (auto) ✅
├─ Tests, Security Scan
├─ Build Docker Image
└─ Push to Dev Registry
    ↓
CD Pipeline (auto) ✅
└─ Deploy to Dev Server (auto)
    ↓
Dev Updated ✅

---

main branch merge
    ↓
CI Pipeline (auto) ✅
├─ Tests, Security Scan
├─ Build Docker Image
└─ Push to Prod Registry
    ↓
CD Pipeline (manual) ⏸️
└─ Waiting for approval
    ↓
Jenkins: Click "Proceed" ✅
    ↓
CD Pipeline continues ✅
├─ Backup Database
├─ Deploy to Prod Server
└─ Health Checks
    ↓
Prod Updated ✅
```

---

## 📊 Pipeline Duration

| Stage | Duration |
|-------|----------|
| CI Pipeline | 10-15 min |
| CD Pipeline | 15-20 min |
| Total (with approval) | 30-45 min |
| Approval wait time | User dependent |

---

## 🔐 Security Checklist

- ✅ Linting & code quality
- ✅ Security scanning (gosec)
- ✅ Docker image scanning (trivy)
- ✅ Database backups
- ✅ SSH-based deployment
- ✅ Manual approval for production
- ✅ Health checks after deploy
- ✅ No secrets in code

---

## 📚 Documentation Map

```
START HERE
    ↓
   👇
Choose your role:
├─ Developer → README_DOCKER_JENKINS.md
├─ DevOps → JENKINS_CICD_SETUP.md
├─ Jenkins Admin → JENKINS_CONFIG.md
├─ SRE → DEPLOYMENT_CHECKLIST.md
└─ Manager → SOLUTION_SUMMARY.md
```

---

## ⚡ One-Minute Setup

```bash
# 1. Test locally
cd project-tracker-be
docker-compose up -d && sleep 5
curl http://localhost:4000/health && \
docker-compose down -v

# 2. Jenkins credentials (via UI)
# Go to Jenkins → Manage Credentials
# Create 9 credentials (see JENKINS_CICD_SETUP.md)

# 3. Jenkins jobs (via UI)
# Create 2 multibranch pipelines:
# - projectflow-backend-ci (Jenkinsfile.ci)
# - projectflow-backend-cd (Jenkinsfile.cd)

# 4. GitHub webhook
# Repo Settings → Webhooks → Add
# URL: https://jenkins.example.com/github-webhook/
# Events: push, pull requests

# 5. Test!
git push origin develop
# Watch Jenkins: CI → CD → Auto deploy
```

---

## 🎓 Learning Resources

- Docker Docs: https://docs.docker.com/compose/
- Jenkins Docs: https://www.jenkins.io/doc/
- GitHub Docs: https://docs.github.com/
- Go Testing: https://golang.org/pkg/testing/

---

## 📝 Environment Setup

```bash
# Copy template
cp .env.example .env

# Update with your values
POSTGRES_PASSWORD=your-secure-password
REDIS_PASSWORD=your-secure-password
JWT_ACCESS_SECRET=your-secure-secret
WEB_ORIGIN=http://localhost:5173
```

---

## ✨ Features

✅ Backend-only Docker Compose  
✅ Separate CI & CD Pipelines  
✅ Auto-deploy to dev  
✅ Manual approval for prod  
✅ Database backups  
✅ Security scanning  
✅ Health checks  
✅ Comprehensive documentation  

---

## 🎯 Next Steps

1. ✅ Files created (you're here!)
2. → Read README_DOCKER_JENKINS.md
3. → Test locally with docker-compose
4. → Setup Jenkins (20 minutes)
5. → Create test PR to develop
6. → Deploy to production
7. → Monitor & celebrate! 🎉

---

**Version:** 1.0.0  
**Created:** 2024  
**Status:** ✅ Ready to Use
