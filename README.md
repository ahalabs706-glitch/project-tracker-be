# project-tracker-be
# ProjectFlow
## Project & Contract Management System

A centralized enterprise platform for managing projects, contracts, timelines, budgets, risks, and project performance.

---

## 1. Overview

ProjectFlow is an enterprise Project & Contract Management System designed to improve project governance, operational visibility, and financial control.

The application addresses three major business challenges:

1. **Contract Expiration Management** – Contracts are frequently overlooked before their expiration dates, resulting in missed renewal opportunities, operational disruption, and potential business risks.
2. **Project Timeline Management** – Project deadlines are frequently missed due to insufficient monitoring of milestones, task dependencies, and project progress.
3. **Budget Control** – Project expenditures are difficult to monitor against approved budgets, creating risks of uncontrolled spending and budget overruns.

ProjectFlow provides a centralized platform to monitor these activities, identify potential problems early, and support management decision-making.

### Main Objectives

- Prevent contract expiration from being overlooked.
- Improve project timeline visibility and accountability.
- Monitor project budgets against actual expenditures.
- Identify project risks and issues before they escalate.
- Provide management with reliable project portfolio information.
- Establish a standardized project monitoring and reporting process.

---

## 2. Technology Stack

The application is developed using separate frontend and backend repositories.

### Frontend

| Technology | Purpose |
|---|---|
| Vite | Development and build tooling |
| React | UI framework |
| TypeScript | Type safety |
| Tailwind CSS | Styling |
| shadcn/ui | UI components |
| React Router | Routing |
| TanStack Query | Server state management |
| Zustand | Client state management |
| Axios | HTTP client |
| React Hook Form | Form management |
| Zod | Form validation |
| Recharts | Data visualization |
| Lucide React | Icons |
| date-fns | Date utilities |
| Gantt Chart Library | Timeline visualization |

### Backend

| Technology | Purpose |
|---|---|
| Go | Backend programming language |
| Gin | HTTP framework |
| GORM | ORM |
| PostgreSQL | Relational database |
| golang-migrate | Database migrations |
| JWT | Authentication |
| Redis | Cache and background job support |
| Asynq | Background job processing |
| Uber Zap | Structured logging |
| Swagger / OpenAPI | API documentation |
| Testify | Unit and integration testing |

### Infrastructure

- Docker
- Docker Compose
- Nginx
- PostgreSQL
- Redis
- S3-compatible object storage
- GitHub
- Jenkins (CI/CD)
- AWS (deployment environment)

---

## 3. System Architecture

ProjectFlow uses a decoupled frontend and backend architecture.

```text
                  USERS
                    |
                    v
          +-------------------+
          |  Vite + React     |
          |  Frontend         |
          +-------------------+
                    |
                    | REST API / JSON
                    v
          +-------------------+
          |  Go Backend       |
          |  Gin Framework    |
          +-------------------+
                    |
          +---------+----------+
          |         |          |
          v         v          v
      PostgreSQL  Redis     S3 Storage
                   |
                   v
              Asynq Workers
                   |
                   v
           Background Jobs
           - Contract Alerts
           - Timeline Alerts
           - Budget Alerts
           - Email Notifications
```

### Repository Structure

Maintain separate repositories:

**Frontend**

`projectflow-frontend`

**Backend**

`projectflow-backend`

Both repositories must follow the same API contract and data definitions.

The backend is the single source of truth for business logic, authorization, calculations, and data validation.

---

# 4. Development Roadmap

Development is divided into three major stages:

1. MVP – Minimum Viable Product
2. Middle Stage – Operational Enhancement
3. Long-Term – Enterprise & Advanced Analytics

The development approach should prioritize business-critical problems before introducing advanced features.

---

# STAGE 1: MVP
## Core Project & Contract Monitoring

**Objective:** Build a functional application that solves the three primary business problems: contract expiration, project timeline monitoring, and budget tracking.

The MVP must be usable by actual project teams and management.

### 4.1 Authentication & User Management

Features:

- User login and logout.
- JWT authentication.
- Password hashing.
- User management.
- Role-based access control (RBAC).
- User activation and deactivation.

Initial roles:

| Role | Responsibility |
|---|---|
| Super Admin | Full system access |
| Admin | System and user management |
| Executive | Management dashboard and reports |
| Project Manager | Project management |
| Project Member | Task and progress updates |
| Finance | Budget monitoring |
| Contract Manager | Contract management |
| Viewer | Read-only access |

Global role permissions are managed by `SUPER_ADMIN` from Access Management → Role permissions. Project owners or managers with `projects.update` can configure task and milestone permissions per project role from the project team panel. `SUPER_ADMIN` permissions are locked; all permission changes are audited.

### 4.2 Organization Management

Features:

- Department management.
- Business unit management.
- User-to-department assignment.
- Project ownership.
- Department-based data filtering.

### 4.3 Dashboard

Create a centralized dashboard showing:

**Project Overview**

- Total Projects.
- Active Projects.
- Completed Projects.
- Delayed Projects.
- Projects At Risk.

**Contract Overview**

- Total Active Contracts.
- Contracts Expiring Soon.
- Contracts Expired.
- Contracts Requiring Renewal.

**Budget Overview**

- Total Approved Budget.
- Total Actual Expenditure.
- Total Committed Cost.
- Budget Utilization.
- Budget Overrun.

**Monitoring & Alerts**

- Upcoming contract expirations.
- Overdue tasks.
- Delayed milestones.
- Budget threshold warnings.
- Critical project alerts.

### 4.4 Project Management

Features:

- Create and update projects.
- Project code generation.
- Project description.
- Project manager assignment.
- Department assignment.
- Project start date.
- Target completion date.
- Approved budget.
- Project status.
- Project progress.
- Project team assignment.

Project statuses:

- Draft
- Proposed
- Planning
- Approved
- In Progress
- On Hold
- Completed
- Cancelled

### 4.5 Contract Management

This is one of the highest-priority modules.

Contract information:

- Contract Number.
- Contract Name.
- Contract Type.
- Vendor / Partner.
- Contract Owner.
- Responsible Department.
- Start Date.
- Effective Date.
- End Date.
- Contract Value.
- Renewal Type.
- Renewal Notice Period.
- Related Project.
- Contract Documents.
- Contract Status.

Contract statuses:

- Draft
- Under Review
- Active
- Expiring Soon
- Renewal in Progress
- Expired
- Terminated
- Completed

**Contract Expiration Monitoring**

Implement automatic monitoring:

| Reminder | Action |
|---|---|
| H-90 | Early warning |
| H-60 | Reminder |
| H-30 | Renewal preparation alert |
| H-14 | High priority alert |
| H-7 | Critical alert |
| Expired | Expiration notification |

Features:

- Automatic expiration detection.
- Contract owner notification.
- Renewal tracking.
- Contract document upload.
- Contract history.
- Contract search and filtering.
- Contract-to-project relationship.

Reminder schedules must be configurable.

A renewal must create a new contract version or renewal record without deleting historical contract information.

### 4.6 Timeline & Task Management

Features:

- Project milestones.
- Task creation.
- Multi-user task assignment with shared status/progress and per-assignee My Tasks visibility.
- Task priority.
- Planned start date.
- Planned due date.
- Actual start date.
- Actual completion date.
- Task progress.
- Task status.
- Overdue detection.

Task statuses:

- Backlog
- To Do
- In Progress
- In Review
- Blocked
- Done
- Cancelled

Milestone monitoring:

- Planned date.
- Actual date.
- Completion status.
- Responsible person.
- Delay duration.

Provide a basic timeline visualization and milestone list.

**Important business rules:**

- Detect overdue tasks automatically.
- Notify responsible users about overdue tasks.
- Notify Project Managers about delayed milestones.
- Maintain a record of important schedule changes.

### 4.7 Budget Management

Implement basic project financial monitoring.

Budget information:

- Initial Budget.
- Approved Budget.
- Budget Category.
- Committed Cost.
- Actual Cost.
- Remaining Budget.
- Budget Utilization.
- Budget Variance.

Budget categories:

- Personnel
- Infrastructure
- Software / License
- Procurement
- Vendor / Consultant
- Operational
- Other

**Budget Monitoring Rules**

| Utilization | Status |
|---|---|
| Below 80% | Normal |
| 80%–89.99% | Warning |
| 90%–100% | Critical |
| Above 100% | Over Budget |

Formulas:

Budget Utilization:

`Actual Cost / Approved Budget * 100`

Budget Variance:

`Approved Budget - Forecast Cost`

Use NUMERIC data types in PostgreSQL for financial calculations.

### 4.8 Notification System

MVP notifications:

- Contract expiration.
- Overdue task.
- Delayed milestone.
- Budget threshold.
- Project status changes.

Channels:

- In-app notifications.
- Email notifications.

Implement background processing to avoid blocking API requests.

### 4.9 Audit Trail

Track important activities:

- Project creation and updates.
- Contract creation and updates.
- Task updates.
- Budget changes.
- User activities.
- Status changes.

Audit records should include:

- User ID.
- Action.
- Entity type.
- Entity ID.
- Previous value.
- New value.
- Timestamp.

### 4.10 MVP Deliverables

- Authentication and RBAC.
- Organization management.
- Dashboard.
- Project management.
- Contract management.
- Contract expiration alerts.
- Basic task and milestone management.
- Basic timeline monitoring.
- Budget tracking.
- In-app and email notifications.
- Basic audit trail.
- REST API documentation.
- Docker deployment.
- Basic automated tests.

**MVP Success Criteria**

The MVP is considered successful when:

1. Every active contract has an assigned owner and end date.
2. The system can identify contracts approaching expiration.
3. Project Managers can monitor tasks and milestones.
4. Overdue tasks and delayed milestones are visible.
5. Approved budget and actual expenditure can be compared.
6. Management can see project and contract status from one dashboard.
7. Users receive alerts for critical business events.

---

# STAGE 2: MIDDLE
## Project Governance & Operational Enhancement

**Objective:** Improve project governance, accountability, financial control, and operational efficiency.

This stage transforms the MVP from a monitoring application into a comprehensive project management platform.

### 5.1 Advanced Project Management

Features:

- Project phases.
- Work Breakdown Structure (WBS).
- Subtasks.
- Task dependencies.
- Critical path identification.
- Project baseline.
- Baseline versus actual comparison.
- Advanced Gantt chart.
- Project completion forecasting.
- Project portfolio filtering.

### 5.2 Advanced Contract Lifecycle Management

Features:

- Contract version management.
- Renewal workflow.
- Contract amendment tracking.
- Contract approval.
- Contract document versioning.
- Vendor management.
- Contract obligation tracking.
- Contract deliverables monitoring.
- Contract-to-project and contract-to-budget mapping.

### 5.3 Advanced Budget Management

Features:

- Budget allocation by category.
- Budget revision workflow.
- Budget approval.
- Planned versus actual cost.
- Committed cost tracking.
- Forecast cost.
- Monthly expenditure tracking.
- Budget history.
- Financial transaction records.

Implement financial forecasting:

- Estimate at Completion (EAC).
- Estimate to Complete (ETC).
- Cost Performance Index (CPI).
- Cost Variance (CV).

### 5.4 Risk Management

Implement a comprehensive Risk Register.

Fields:

- Risk Title.
- Risk Description.
- Risk Category.
- Probability.
- Impact.
- Risk Score.
- Risk Owner.
- Mitigation Plan.
- Contingency Plan.
- Target Resolution Date.
- Risk Status.

Features:

- Risk matrix.
- Risk scoring.
- Risk ownership.
- Mitigation tracking.
- Risk escalation.
- Risk history.

### 5.5 Issue Management

Features:

- Issue registration.
- Issue severity.
- Issue priority.
- Assigned person.
- Resolution target.
- Issue status.
- Resolution notes.
- Escalation history.

### 5.6 Change Request Management

Support formal change requests for:

- Scope changes.
- Timeline changes.
- Budget changes.
- Contract changes.

Workflow:

1. User submits change request.
2. Project Manager reviews.
3. Impact assessment is recorded.
4. Authorized approver evaluates.
5. Request is approved or rejected.
6. Approved changes update project records.
7. Previous baselines and revisions remain available.

### 5.7 Approval Workflow

Implement configurable approval processes.

Approval categories:

- Project approval.
- Budget approval.
- Budget revision.
- Timeline change.
- Contract renewal.
- Contract amendment.
- Project closure.

Features:

- Multi-level approval.
- Approval delegation.
- Approval comments.
- Rejection reason.
- Approval history.
- Pending approval notifications.

### 5.8 Advanced Reporting

Reports:

- Project Portfolio Report.
- Project Health Report.
- Timeline Performance Report.
- Contract Expiration Report.
- Contract Renewal Report.
- Budget Performance Report.
- Risk Register Report.
- Issue Tracking Report.
- Task Performance Report.

Export:

- CSV.
- Excel.
- PDF.

### 5.9 Resource Management

Features:

- Resource allocation.
- Team workload.
- Member availability.
- Project assignment.
- Resource utilization.
- Workload overview.

### 5.10 Middle Stage Deliverables

- Advanced Gantt chart.
- Project baseline management.
- WBS and task dependencies.
- Advanced contract lifecycle.
- Vendor management.
- Budget revision and approval.
- Risk management.
- Issue management.
- Change request management.
- Multi-level approval.
- Advanced reports.
- Resource management.

**Middle Stage Success Criteria**

1. Project schedule changes are controlled through baselines.
2. Budget revisions have approval history.
3. Project risks and issues have assigned owners.
4. Contract renewals have traceable workflows.
5. Project changes are documented and approved.
6. Management can compare planned versus actual performance.
7. Project Managers can identify resource allocation problems.

---

# STAGE 3: LONG-TERM
## Enterprise Intelligence & Predictive Project Management

**Objective:** Transform ProjectFlow into an enterprise-level project governance and decision-support platform.

This stage focuses on automation, predictive analytics, integrations, and advanced management capabilities.

### 6.1 Advanced Project Performance Analytics

Implement Earned Value Management (EVM).

Metrics:

- Planned Value (PV).
- Earned Value (EV).
- Actual Cost (AC).
- Schedule Variance (SV).
- Cost Variance (CV).
- Schedule Performance Index (SPI).
- Cost Performance Index (CPI).
- Estimate at Completion (EAC).
- Estimate to Complete (ETC).

Provide visual performance indicators and historical trends.

### 6.2 Predictive Project Analytics

Develop analytics to identify:

- Projects likely to miss deadlines.
- Projects likely to exceed approved budgets.
- Contracts at risk of missing renewal deadlines.
- Milestones with high delay probability.
- Projects with increasing operational risks.

Use historical project data and measurable indicators.

Predictive features should be introduced only after sufficient historical data has been collected and validated.

### 6.3 AI-Powered Project Assistant

Potential capabilities:

- Generate project status summaries.
- Summarize project activity.
- Identify potential project risks.
- Explain budget variance.
- Summarize contract information.
- Generate management reports.
- Recommend mitigation actions.
- Answer natural-language questions about project data.

AI-generated insights must reference supporting data and should not automatically approve financial or contractual decisions.

### 6.4 Advanced Contract Intelligence

Features:

- Contract document text extraction.
- OCR for scanned documents.
- AI-assisted contract summarization.
- Extract important contract dates.
- Identify renewal clauses.
- Identify contractual obligations.
- Detect missing contract information.
- Contract compliance monitoring.

All extracted information must be verified by an authorized user.

### 6.5 Enterprise Integration

Potential integrations:

- Microsoft 365.
- Microsoft Outlook.
- Microsoft Teams.
- SharePoint.
- Microsoft Power BI.
- SAP ERP.
- Financial systems.
- HR systems.
- Single Sign-On (SSO) using Microsoft Entra ID.

Integration priorities should be based on actual organizational requirements.

### 6.6 Advanced Workflow Automation

Features:

- Configurable workflow engine.
- Conditional notification rules.
- Automatic escalation.
- Approval delegation.
- Recurring project activities.
- Automatic report distribution.
- Automated contract renewal reminders.
- Event-driven integration.

### 6.7 Enterprise Portfolio Management

Features:

- Multi-department portfolio management.
- Cross-project dependencies.
- Strategic objective alignment.
- Portfolio budget allocation.
- Portfolio performance comparison.
- Executive scorecards.
- Organizational KPI tracking.

### 6.8 Advanced Security & Governance

Features:

- Enterprise SSO.
- Advanced permission policies.
- Data access segregation.
- Enhanced audit monitoring.
- Data retention policies.
- Backup and disaster recovery.
- Security monitoring.
- API observability.

### 6.9 Long-Term Deliverables

- Earned Value Management.
- Predictive analytics.
- AI project assistant.
- Contract intelligence.
- Enterprise integrations.
- Workflow automation.
- Advanced portfolio management.
- Enterprise SSO.
- Advanced governance and security.

**Long-Term Success Criteria**

1. Management can identify project performance trends.
2. The system can provide evidence-based early warnings.
3. Contract documents can be analyzed and important information extracted.
4. Repetitive project monitoring activities are automated.
5. Project portfolio data can support strategic decisions.
6. ProjectFlow can integrate with organizational enterprise systems.

---

# 7. Development Priority Matrix

| Module | MVP | Middle | Long-Term |
|---|:---:|:---:|:---:|
| Authentication & RBAC | Yes | Enhance | SSO |
| Organization | Yes | Enhance | Enterprise |
| Dashboard | Basic | Advanced | Predictive |
| Project Management | Basic | Advanced | Portfolio |
| Contract Management | Basic | Lifecycle | Intelligence |
| Timeline | Basic | Gantt / WBS | Predictive |
| Task Management | Basic | Dependencies | Automation |
| Budget Management | Basic | Forecasting | EVM |
| Risk Management | Basic | Advanced | Predictive |
| Issue Management | Basic | Advanced | Automation |
| Change Request | - | Yes | Advanced |
| Approval Workflow | Basic | Multi-level | Workflow Engine |
| Notification | Basic | Configurable | Event-driven |
| Reporting | Basic | Advanced | AI-assisted |
| Resource Management | - | Yes | Optimization |
| AI Assistant | - | - | Yes |
| Enterprise Integration | - | Selected | Advanced |

---

# 8. Database Design

Use PostgreSQL as the primary relational database.

Use UUID as primary keys and maintain appropriate indexes, foreign keys, constraints, and database migrations.

### Core Entities – MVP

- organizations
- departments
- users
- roles
- permissions
- user_roles
- role_permissions
- projects
- project_members
- contracts
- contract_renewals
- contract_documents
- milestones
- tasks
- budget_categories
- project_budgets
- budget_transactions
- notifications
- notification_logs
- audit_logs
- attachments

### Additional Entities – Middle Stage

- project_phases
- task_dependencies
- project_baselines
- budget_revisions
- risks
- risk_mitigations
- issues
- change_requests
- approvals
- vendors
- resources
- resource_allocations
- contract_obligations

### Additional Entities – Long-Term

- project_performance_metrics
- earned_value_snapshots
- prediction_results
- ai_insights
- workflow_definitions
- workflow_instances
- integration_connections
- integration_logs
- document_extractions

### Database Design Principles

- Use NUMERIC for financial values.
- Use TIMESTAMPTZ for timestamps.
- Use DATE for business dates.
- Use JSONB only for flexible metadata.
- Maintain historical records for important business changes.
- Use soft deletion only where appropriate.
- Apply database constraints to protect data integrity.
- Use version-controlled migrations.
- Never rely exclusively on frontend validation.

---

# 9. API Standards

Backend API base path:

`/api/v1`

Example endpoints:

### Authentication

- POST /auth/login
- POST /auth/refresh
- POST /auth/logout
- GET /auth/me

### Dashboard

- GET /dashboard/overview
- GET /dashboard/project-health
- GET /dashboard/contract-expiration
- GET /dashboard/budget-overview

### Projects

- GET /projects
- POST /projects
- GET /projects/:id
- PUT /projects/:id
- GET /projects/:id/overview
- GET /projects/:id/milestones

### Contracts

- GET /contracts
- POST /contracts
- GET /contracts/:id
- PUT /contracts/:id
- GET /contracts/expiring
- GET /contracts/:id/renewals

### Tasks

- GET /projects/:id/tasks
- POST /projects/:id/tasks
- PUT /tasks/:id
- GET /tasks/overdue

### Budget

- GET /projects/:id/budget
- POST /projects/:id/budget/transactions
- GET /projects/:id/budget/summary

### Risks & Issues

- GET /projects/:id/risks
- POST /projects/:id/risks
- GET /projects/:id/issues
- POST /projects/:id/issues

### Notifications

- GET /notifications
- PATCH /notifications/:id/read

All endpoints must follow consistent REST conventions and provide pagination, filtering, validation, authorization, and standardized error responses.

Maintain a shared OpenAPI specification as the frontend-backend integration contract.

---

# 10. Non-Functional Requirements

### Security

- OWASP API Security best practices.
- Secure password hashing.
- JWT access and refresh token management.
- Role-based authorization.
- Input validation.
- Rate limiting.
- CORS configuration.
- Secure file uploads.
- Environment-based secrets.
- Audit logging.

### Performance

- Database indexing.
- Pagination.
- Efficient SQL queries.
- Connection pooling.
- Caching where necessary.
- Background processing for scheduled jobs.
- Avoid N+1 query problems.

### Reliability

- Health check endpoint.
- Readiness endpoint.
- Graceful shutdown.
- Background job retries.
- Database backup strategy.
- Error monitoring.
- Structured logging.

### Testing

- Unit tests.
- Integration tests.
- API tests.
- Authentication tests.
- Authorization tests.
- Business rule tests.
- Financial calculation tests.
- Contract expiration tests.
- Timeline calculation tests.

---

# 11. Development Guidelines

Both repositories must be developed independently but follow a shared technical contract.

### Backend Development Rules

- Follow Clean Architecture.
- Separate handler, service, repository, and domain layers.
- Implement business rules in the backend.
- Maintain database migrations.
- Document every API endpoint.
- Write automated tests.
- Do not hardcode secrets.

### Frontend Development Rules

- Follow feature-based architecture.
- Use reusable UI components.
- Use TanStack Query for server state.
- Use Zustand for client-side state.
- Use Zod for form validation.
- Use centralized API clients.
- Implement loading, error, and empty states.
- Avoid hardcoded production data.
- Follow the backend API contract.

### Shared Development Rules

- Use consistent naming conventions.
- Maintain API documentation.
- Use semantic Git commits.
- Review code before merging.
- Maintain environment-specific configurations.
- Ensure backward compatibility for API changes where practical.

---

# 12. DevOps & Deployment Strategy

Recommended deployment architecture:

```text
                Internet
                   |
                   v
                NGINX
                   |
             +-----+-----+
             |           |
             v           v
          Frontend    Go Backend
          (Vite)      (Gin API)
                         |
                  +------+------+
                  |             |
                  v             v
              PostgreSQL      Redis
                                |
                                v
                           Asynq Worker
```

### Deployment Components

- Dockerfile for frontend.
- Dockerfile for backend.
- Docker Compose for local and initial server deployment.
- Nginx reverse proxy.
- PostgreSQL persistent volume.
- Redis.
- Background worker.
- S3-compatible object storage.

### CI/CD Pipeline

Recommended pipeline:

1. Source checkout.
2. Dependency installation.
3. Linting.
4. Unit testing.
5. Build frontend and backend.
6. Security scanning.
7. Docker image build.
8. Docker image push.
9. Deployment to server.
10. Health check and deployment verification.

Use separate development, staging, and production environments.

---

# 13. Recommended Development Sequence

| Stage | Focus | Expected Outcome |
|---|---|---|
| MVP - 1 | Foundation, Authentication, RBAC | Secure application foundation |
| MVP - 2 | Organization & Project Management | Project registration and ownership |
| MVP - 3 | Contract Management | Contract expiry monitoring |
| MVP - 4 | Timeline & Task Management | Schedule visibility |
| MVP - 5 | Budget Management | Financial visibility |
| MVP - 6 | Dashboard & Notifications | Centralized monitoring |
| MVP - 7 | Testing & Deployment | Usable MVP |
| Middle - 1 | Advanced Project Management | Gantt, WBS, baselines |
| Middle - 2 | Advanced Contract Lifecycle | Renewal and obligations |
| Middle - 3 | Budget & Approval Workflow | Financial governance |
| Middle - 4 | Risk & Issue Management | Project governance |
| Middle - 5 | Reporting & Resource Management | Operational efficiency |
| Long - 1 | EVM & Predictive Analytics | Performance intelligence |
| Long - 2 | AI & Contract Intelligence | Intelligent assistance |
| Long - 3 | Enterprise Integration | Connected ecosystem |
| Long - 4 | Portfolio & Automation | Enterprise governance |

---

# 14. MVP Definition of Done

The MVP must not be considered complete merely because all screens are implemented.

The following business scenarios must work end-to-end:

### Scenario 1: Contract Expiration

1. Contract Manager registers a contract.
2. Contract owner and end date are assigned.
3. System automatically evaluates expiration dates.
4. Notifications are generated according to reminder rules.
5. Contract Manager records a renewal action.
6. Renewal history is retained.

### Scenario 2: Project Delay

1. Project Manager creates a project.
2. Project milestones and tasks are defined.
3. Team members update task progress.
4. System identifies overdue tasks.
5. Project Manager receives alerts.
6. Management can identify delayed projects from the dashboard.

### Scenario 3: Budget Overrun

1. Project Manager registers the proposed budget.
2. Approved budget is recorded.
3. Finance records project expenditures.
4. System calculates budget utilization.
5. System generates threshold warnings.
6. Management can identify projects exceeding budget.

### Scenario 4: Management Monitoring

1. Executive accesses the dashboard.
2. Executive views project portfolio status.
3. Executive identifies expiring contracts.
4. Executive identifies delayed projects.
5. Executive identifies budget risks.
6. Executive accesses detailed reports.

---

# 15. Project Development Principles

The following principles must guide the entire development process:

1. Business problems take priority over unnecessary features.
2. Build a working MVP before introducing advanced functionality.
3. Keep frontend and backend completely separated.
4. Backend owns business rules and authoritative calculations.
5. Preserve historical financial, contractual, and schedule records.
6. Every important business event must be traceable.
7. Notifications must be actionable, not merely informational.
8. All critical entities must have clear ownership.
9. Management dashboards must display reliable and up-to-date information.
10. Design the architecture for future expansion without overengineering the MVP.

---

## 16. Final Product Vision

ProjectFlow aims to become a centralized enterprise platform for project execution and governance.

The long-term vision is to provide management with a reliable answer to five important questions:

1. **What projects are currently running?**
2. **Which contracts are approaching expiration?**
3. **Which projects are behind schedule?**
4. **Which projects are exceeding or approaching their budget limits?**
5. **What actions should management take before problems become critical?**

ProjectFlow should evolve from a basic monitoring tool into an intelligent project governance platform that improves visibility, accountability, and decision-making across the organization.

---

**Product:** ProjectFlow  
**Category:** Enterprise Project & Contract Management  
**Frontend:** Vite + React + TypeScript  
**Backend:** Go + Gin  
**Database:** PostgreSQL  
**Architecture:** Decoupled Frontend & Backend  
**Development Strategy:** MVP → Middle Stage → Long-Term  
**Deployment:** Docker + Nginx + CI/CD