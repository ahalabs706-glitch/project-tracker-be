# Jenkins CI/CD Configuration Template

Gunakan konfigurasi ini untuk setup Jobs di Jenkins.

## 1. Job Configuration untuk CI Pipeline

### Name: projectflow-backend-ci

```yaml
# Multibranch Pipeline Configuration

General:
  Display Name: ProjectFlow Backend CI
  Description: |
    Continuous Integration Pipeline untuk backend ProjectFlow
    - Code checkout & analysis
    - Linting & formatting check
    - Security scanning (gosec)
    - Unit tests dengan coverage
    - Docker image build & scan

Branch Sources:
  - GitHub
    - Repository HTTPS URL: https://github.com/username/repository.git
    - Credentials: GitHub credentials
    - Include branches: develop, main
    - Exclude branches: master

Build Configuration:
  - Mode: by Jenkinsfile
  - Script Path: project-tracker-be/Jenkinsfile.ci

Scan Repository Triggers:
  - Interval: H H * * *  # Daily scan
  - Webhook: GitHub Push events

Pipeline Model Definition:
  - Custom: Enable extended settings
  - Timeout: 30 minutes
  - Abort previous builds: enabled

Notification:
  - Notify on build failure
  - Notify on build fix
  - GitHub commit status: enabled
```

### Trigger Configuration

```groovy
// Jenkinsfile trigger example
properties([
    pipelineTriggers([
        githubPush(),
        pollSCM('H/5 * * * *')  // Poll every 5 minutes
    ]),
    parameters([
        booleanParam(
            name: 'FORCE_BUILD',
            defaultValue: false,
            description: 'Force build tanpa conditional checks'
        )
    ])
])
```

---

## 2. Job Configuration untuk CD Pipeline

### Name: projectflow-backend-cd

```yaml
# Multibranch Pipeline Configuration

General:
  Display Name: ProjectFlow Backend CD
  Description: |
    Continuous Deployment Pipeline untuk backend ProjectFlow
    - Build & scan Docker image
    - Push ke registry
    - Deploy ke dev (otomatis)
    - Deploy ke prod (manual approval)

Branch Sources:
  - GitHub
    - Repository HTTPS URL: https://github.com/username/repository.git
    - Credentials: GitHub credentials
    - Include branches: develop, main
    - Exclude branches: master

Build Configuration:
  - Mode: by Jenkinsfile
  - Script Path: project-tracker-be/Jenkinsfile.cd

Scan Repository Triggers:
  - Interval: H H * * *

Pipeline Model Definition:
  - Timeout: 45 minutes
  - Abort previous builds: enabled

Notification:
  - Slack integration: enabled
  - Email on failure: enabled
  - GitHub deployment status: enabled
```

---

## 3. Jenkins Credentials Configuration

### Menggunakan Jenkins CLI atau UI

#### Docker Registry Credentials

```bash
# Via Jenkins CLI
jenkins-cli create-credentials-by-xml system::system::jenkins \
  '(global)' < docker-registry-creds.xml
```

```xml
<!-- docker-registry-creds.xml -->
<com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl>
  <scope>GLOBAL</scope>
  <id>DOCKER_REGISTRY_CREDENTIALS</id>
  <description>Docker Hub Registry Credentials</description>
  <username>docker-username</username>
  <password>docker-token-or-password</password>
</com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl>
```

#### SSH Private Key

```bash
# Via Jenkins CLI
jenkins-cli create-credentials-by-xml system::system::jenkins \
  '(global)' < ssh-key-creds.xml
```

```xml
<!-- ssh-key-creds.xml -->
<com.cloudbees.jenkins.plugins.kubernetes.credentials.impl.BasicSSHUserPrivateKey>
  <scope>GLOBAL</scope>
  <id>SSH_PRIVATE_KEY</id>
  <description>SSH Key for Deployment</description>
  <username>deployer</username>
  <passphrase></passphrase>
  <privateKeySource class="com.cloudbees.jenkins.plugins.kubernetes.credentials.impl.BasicSSHUserPrivateKey$DirectEntryPrivateKeySource">
    <privateKey>-----BEGIN RSA PRIVATE KEY-----
...private key content...
-----END RSA PRIVATE KEY-----</privateKey>
  </privateKeySource>
</com.cloudbees.jenkins.plugins.kubernetes.credentials.impl.BasicSSHUserPrivateKey>
```

#### Secret Text Credentials

```bash
# Via Jenkins CLI untuk multiple secret credentials
for secret_id in API_DOCKER_DEV API_DOCKER_PROD DOCKER_DEV_USERNAME DOCKER_DEV_PASSWORD DOCKER_PROD_USERNAME DOCKER_PROD_PASSWORD DEPLOYMENT_SERVER_DEV DEPLOYMENT_SERVER_PROD; do
  jenkins-cli create-credentials-by-xml system::system::jenkins \
    '(global)' < secret-text-${secret_id}.xml
done
```

```xml
<!-- secret-text-API_DOCKER_DEV.xml -->
<org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl>
  <scope>GLOBAL</scope>
  <id>API_DOCKER_DEV</id>
  <description>Docker Repository Name for Development</description>
  <secret>docker-username/projectflow-api-dev</secret>
</org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl>
```

```xml
<!-- secret-text-DEPLOYMENT_SERVER_DEV.xml -->
<org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl>
  <scope>GLOBAL</scope>
  <id>DEPLOYMENT_SERVER_DEV</id>
  <description>Development Deployment Server Address</description>
  <secret>deployer@dev-server.example.com</secret>
</org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl>
```

---

## 4. System Configuration

### Jenkins System Configuration

```groovy
// Jenkins System Configuration (via Groovy Console atau Configuration as Code)

import jenkins.model.Jenkins
import com.cloudbees.plugins.credentials.SystemCredentialsProvider
import com.cloudbees.plugins.credentials.domains.Domain

// Configure Docker credentials
def store = SystemCredentialsProvider.getInstance()
def domain = Domain.global()

// Add credentials programmatically
// ...
```

### Pipeline Global Variables

```groovy
// Define in Jenkins System Configuration > Global Pipeline Libraries

library identifier: 'projectflow-backend-pipeline@main',
        retriever: modernSCM(
            github(
                id: 'github-projectflow',
                credentialsId: 'github-credentials',
                repositoryUrl: 'https://github.com/username/projectflow-pipeline-lib.git',
                traits: [gitHubBranchDiscovery(strategyId: 1)]
            )
        )
```

---

## 5. Jenkins Webhook Configuration

### GitHub Webhook Setup

1. Repository Settings → Webhooks → Add webhook
2. Payload URL: `https://jenkins.example.com/github-webhook/`
3. Content type: `application/json`
4. Events:
   - Push events
   - Pull request events
   - Branch or tag creation

```bash
# Test webhook
curl -X POST https://jenkins.example.com/github-webhook/ \
  -H 'Content-Type: application/json' \
  -d @webhook-payload.json
```

---

## 6. Environment Variables untuk Deployment

### Development Server Configuration

```bash
# /opt/projectflow/backend/.env.dev
APP_ENV=development
DOCKER_IMAGE=docker-username/projectflow-api-dev:latest
POSTGRES_PASSWORD=dev-password-123
REDIS_PASSWORD=dev-redis-password
```

### Production Server Configuration

```bash
# /opt/projectflow/backend/.env.prod
APP_ENV=production
DOCKER_IMAGE=docker-username/projectflow-api:latest
POSTGRES_PASSWORD=prod-password-secure-123
REDIS_PASSWORD=prod-redis-password-secure
WEB_ORIGIN=https://app.example.com
```

---

## 7. Monitoring & Notifications

### Slack Integration

Configure Jenkins Slack plugin untuk notifications:

```groovy
// In pipeline post section
post {
    always {
        slackSend(
            channel: '#deployments',
            color: currentBuild.result == 'SUCCESS' ? 'good' : 'danger',
            message: """
            ${currentBuild.fullDisplayName}
            Environment: ${env.DEPLOY_ENV}
            Branch: ${env.BRANCH_NAME}
            Status: ${currentBuild.result}
            """.stripIndent()
        )
    }
}
```

### Email Configuration

```groovy
emailext(
    subject: "Build #${env.BUILD_NUMBER} - ${currentBuild.result}",
    body: """
    Project: ${env.JOB_NAME}
    Build Number: ${env.BUILD_NUMBER}
    Build Status: ${currentBuild.result}
    Build URL: ${env.BUILD_URL}
    """.stripIndent(),
    to: 'devops@example.com',
    recipientProviders: [developers(), requestor()]
)
```

---

## 8. Backup & Restore Strategy

### Database Backup Script

```bash
#!/bin/bash
# /opt/projectflow/backend/scripts/backup-db.sh

set -e

BACKUP_DIR="/backups/projectflow"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DB_NAME="projectflow"

mkdir -p ${BACKUP_DIR}

# Create backup
docker-compose exec -T postgres \
  pg_dump -U projectflow ${DB_NAME} | \
  gzip > ${BACKUP_DIR}/db_backup_${TIMESTAMP}.sql.gz

# Keep only last 7 days
find ${BACKUP_DIR} -name "db_backup_*.sql.gz" -mtime +7 -delete

echo "Backup completed: ${BACKUP_DIR}/db_backup_${TIMESTAMP}.sql.gz"
```

### Database Restore Script

```bash
#!/bin/bash
# /opt/projectflow/backend/scripts/restore-db.sh

set -e

BACKUP_FILE=$1

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup-file>"
    exit 1
fi

# Restore from backup
gunzip -c ${BACKUP_FILE} | \
  docker-compose exec -T postgres \
  psql -U projectflow projectflow

echo "Restore completed from: ${BACKUP_FILE}"
```

---

## 9. Health Check Scripts

### API Health Check

```bash
#!/bin/bash
# /opt/projectflow/backend/scripts/health-check.sh

set -e

HEALTH_ENDPOINT="http://localhost:4000/health"
TIMEOUT=30

echo "Checking API health..."

for i in $(seq 1 $TIMEOUT); do
    if curl -f -s $HEALTH_ENDPOINT > /dev/null; then
        echo "✓ API is healthy"
        exit 0
    fi
    echo "Attempt $i/$TIMEOUT..."
    sleep 1
done

echo "✗ API health check failed after $TIMEOUT seconds"
exit 1
```

---

## 10. Setup Automation Script

Setup script untuk quick Jenkins configuration:

```bash
#!/bin/bash
# scripts/setup-jenkins.sh

set -e

JENKINS_URL="${JENKINS_URL:-http://localhost:8080}"
JENKINS_USER="${JENKINS_USER:-admin}"
JENKINS_TOKEN="${JENKINS_TOKEN:-}"

echo "Creating Jenkins jobs..."

# Create CI Job
java -jar jenkins-cli.jar -s $JENKINS_URL \
  create-job projectflow-backend-ci \
  < ci-job-config.xml

# Create CD Job
java -jar jenkins-cli.jar -s $JENKINS_URL \
  create-job projectflow-backend-cd \
  < cd-job-config.xml

echo "Jobs created successfully!"
echo "Configure credentials in Jenkins UI:"
echo "1. API_DOCKER_DEV"
echo "2. API_DOCKER_PROD"
echo "3. DOCKER_DEV_USERNAME"
echo "4. DOCKER_DEV_PASSWORD"
echo "5. DOCKER_PROD_USERNAME"
echo "6. DOCKER_PROD_PASSWORD"
echo "7. DEPLOYMENT_SERVER_DEV"
echo "8. DEPLOYMENT_SERVER_PROD"
echo "9. SSH_PRIVATE_KEY"
```

---

**Version**: 1.0.0
**Last Updated**: 2024
