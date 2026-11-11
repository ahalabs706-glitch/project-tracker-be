# GitHub Branch Protection Configuration

Konfigurasi untuk setup GitHub branch protection rules agar mendukung CI/CD workflow yang aman.

---

## 📋 Branch Protection Rules

### For `develop` Branch

Navigate to: Repository Settings → Branches → Branch protection rules

#### Rule: `develop`

**Basic Settings:**
- ✅ Require a pull request before merging
  - ✅ Require approvals: 1
  - ✅ Dismiss stale pull request approvals when new commits are pushed
  - ✅ Require review from code owners

- ✅ Require status checks to pass before merging
  - ✅ Require branches to be up to date before merging
  - Status checks required:
    - `continuous-integration/jenkins/branch`
    - `continuous-integration/jenkins/pr-merge`

- ✅ Require branches to be up to date before merging

- ✅ Require deployment to succeed before merging
  - Required deployments:
    - `dev` (after PR merge)

**Advanced Settings:**
- [ ] Include administrators
- ✅ Restrict who can push to matching branches

**Allowed to push:**
- GitHub Apps: Jenkins CI/CD
- Users: (leave empty for admin only)
- Teams: (leave empty)

**Conversation Settings:**
- ✅ Require conversations to be resolved before merging
- ✅ Allow auto-merge
- ✅ Allow deletion of head branch

---

### For `main` Branch

Navigate to: Repository Settings → Branches → Branch protection rules

#### Rule: `main`

**Basic Settings:**
- ✅ Require a pull request before merging
  - ✅ Require approvals: 2
  - ✅ Dismiss stale pull request approvals when new commits are pushed
  - ✅ Require review from code owners
  - ✅ Require approval of the most recent push

- ✅ Require status checks to pass before merging
  - ✅ Require branches to be up to date before merging
  - Status checks required:
    - `continuous-integration/jenkins/branch`
    - `continuous-integration/jenkins/pr-merge`

- ✅ Require branches to be up to date before merging

- ✅ Require deployment to succeed before merging
  - Required deployments:
    - `production` (manual approval in Jenkins)

- ✅ Require a merge commit
  - (This enforces merge commits instead of squash)

- ✅ Require dismissal of stale reviews

**Advanced Settings:**
- ✅ Include administrators
- ✅ Restrict who can push to matching branches

**Allowed to push:**
- GitHub Apps: None (no auto-push to main)
- Users: Only release managers
- Teams: Only devops team

**Conversation Settings:**
- ✅ Require conversations to be resolved before merging
- ❌ Allow auto-merge (disable for prod safety)
- ❌ Allow deletion of head branch (disable for prod safety)

**Webhook Configuration:**
- ✅ Enable GitHub webhook to Jenkins

---

## 🔧 GitHub Webhook Setup

### Webhook URL

**URL:** `https://jenkins.example.com/github-webhook/`

### Events to Send

Select:
- ✅ Push events
- ✅ Pull requests
- ✅ Pull request reviews
- ✅ Branch or tag creation
- ✅ Status
- ✅ Check runs
- ✅ Check suites

### Configuration

- **Content type:** application/json
- **SSL verification:** ✅ Enable SSL verification
- **Active:** ✅ Yes

### Test Webhook

```bash
# After setup, test with:
curl -X POST https://jenkins.example.com/github-webhook/ \
  -H 'X-GitHub-Event: push' \
  -H 'X-GitHub-Delivery: test-delivery' \
  -H 'Content-Type: application/json' \
  -d '{"ref":"refs/heads/develop","repository":{"name":"project-tracker"}}'
```

---

## 🤖 GitHub Actions Integration (Optional)

### Setup Status Check

If using GitHub Actions instead of Jenkins:

```yaml
# .github/workflows/ci.yml
name: CI Pipeline
on:
  push:
    branches: [develop, main]
  pull_request:
    branches: [develop, main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version-file: go.mod
      
      - name: Run tests
        run: go test ./...
      
      - name: Run linting
        run: go vet ./...
```

---

## 📝 CODEOWNERS File

Create `.github/CODEOWNERS` for code review requirements:

```
# Default owners
* @devops-team @tech-lead

# Backend specific
/project-tracker-be/ @backend-team @devops-team
/project-tracker-be/internal/auth/ @auth-specialist @backend-team

# Infrastructure
/.github/workflows/ @devops-team
/terraform/ @devops-team
/kubernetes/ @devops-team
/docker-compose.yml @devops-team
/Jenkinsfile* @devops-team
```

---

## 🔐 Protected Branch Settings Summary

| Setting | develop | main |
|---------|---------|------|
| Pull Request Required | ✅ Yes | ✅ Yes |
| Require Approvals | 1 | 2 |
| Dismiss Stale | ✅ Yes | ✅ Yes |
| Status Checks | Jenkins CI | Jenkins CI + CD |
| Up to Date | ✅ Yes | ✅ Yes |
| Merge Commit | ❌ Any | ✅ Merge only |
| Admin Override | ❌ No | ✅ Yes |
| Delete Branch | ✅ Yes | ❌ No |
| Auto-merge | ✅ Yes | ❌ No |

---

## 🚀 Git Workflow with Branch Protection

### Feature Development

```bash
# 1. Create feature branch from develop
git checkout develop
git pull origin develop
git checkout -b feature/new-feature

# 2. Make changes and commit
git add .
git commit -m "feat: description"
git push origin feature/new-feature

# 3. Create Pull Request
# Go to GitHub → Create PR to develop
# - Fill description
# - Request reviewers
# - Wait for approval

# 4. Check required status
# - Jenkins CI must pass
# - 1 approval required
# - Conversations resolved

# 5. Merge to develop
# Click "Squash and merge" or "Create a merge commit"
```

**After merge to develop:**
- ✅ Jenkins CI runs automatically
- ✅ Jenkins CD auto-deploys to dev
- ✅ Dev environment updated

### Release to Production

```bash
# 1. Create release branch from develop
git checkout develop
git pull origin develop
git checkout -b release/v1.0.0

# 2. Version bump and changelog
git add .
git commit -m "chore: bump to v1.0.0"

# 3. Merge to main
git checkout main
git pull origin main
git merge --no-ff release/v1.0.0

# 4. Create tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin main --tags

# 5. Create Pull Request
# Go to GitHub → Create PR from release/v1.0.0 to main
# - Requires 2 approvals
# - Must be merge commit
# - All status checks pass
# - Conversations resolved

# 6. Merge to main
# Jenkins CI & CD run
# Wait for approval in Jenkins UI
```

**After merge to main:**
- ✅ Jenkins CI runs automatically
- ✅ Jenkins CD builds image & waits for approval
- ⏸️ Manual approval required in Jenkins
- ✅ Jenkins CD deploys to production with backup

---

## ⚠️ Important Rules

### Do NOT

- ❌ Force push to `develop` or `main`
- ❌ Commit directly to `develop` or `main`
- ❌ Skip code review
- ❌ Bypass status checks
- ❌ Delete `develop` or `main` branch

### Always

- ✅ Create feature branch from `develop`
- ✅ Make PR before merging
- ✅ Wait for approval
- ✅ Ensure status checks pass
- ✅ Use descriptive commit messages
- ✅ Keep branches up to date

---

## 🔄 Example: Full Workflow

### Step 1: Feature Development

```bash
git checkout -b feature/user-auth
# Make changes
git push origin feature/user-auth
```

**Status:** 🔵 Awaiting PR

### Step 2: Create Pull Request

Go to GitHub → Create PR to `develop`

**Status:** 🟡 Jenkins CI running

### Step 3: Review & Approval

- Code review by team member
- Jenkins CI passes
- 1 approval received

**Status:** 🟢 Ready to merge

### Step 4: Merge to Develop

Click "Squash and merge" or "Create a merge commit"

**Status:** 🟢 Merged
- Jenkins CI runs
- Jenkins CD auto-deploys to dev
- Dev environment updated

### Step 5: Create Release PR

```bash
git checkout release/v1.0.0
git checkout main
git pull origin main
```

Create PR: `release/v1.0.0` → `main`

**Status:** 🟡 Awaiting review

### Step 6: Production Approval

- 2 code reviews required
- Jenkins CI must pass
- Status checks verified
- Conversations resolved

### Step 7: Merge to Main

Click "Create merge commit" (must be merge)

**Status:** 🟢 Merged
- Jenkins CI runs
- Jenkins CD runs & waits for approval
- DevOps approves in Jenkins
- Production deployed with backup

---

## 🆘 Troubleshooting Branch Protection

### Issue: Cannot merge PR

**Causes:**
- [ ] Status checks failed → Fix code, re-run
- [ ] Lacks approvals → Request review
- [ ] Branch out of date → Update branch
- [ ] Conversations unresolved → Resolve comments

**Solution:**
```bash
# Sync with develop
git fetch origin
git merge origin/develop
git push origin feature/name

# Wait for status checks to pass
# Then resolve approvals
```

### Issue: Cannot push to main

**Cause:** Main branch is protected

**Solution:**
- Always create PR first
- Never push directly to main
- Use GitHub web interface to merge

### Issue: Webhook not working

```bash
# Check webhook status in GitHub
Repository Settings → Webhooks → Check delivery

# Redeploy webhook
curl -X POST https://jenkins.example.com/github-webhook/ \
  -H 'Content-Type: application/json' \
  -d '{"action":"test"}'

# Check Jenkins webhook endpoint
https://jenkins.example.com/github-webhook/
```

---

## 📚 Documentation

- [GitHub Branch Protection](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches)
- [GitHub Webhooks](https://docs.github.com/en/developers/webhooks-and-events/webhooks)
- [CODEOWNERS](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)

---

**Version:** 1.0.0  
**Last Updated:** 2024
