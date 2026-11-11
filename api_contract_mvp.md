# PROJECTFLOW
# SHARED API CONTRACT

**Document Type:** API Specification  
**Product:** ProjectFlow  
**Version:** 1.0  
**Scope:** MVP  
**API Style:** REST / JSON  
**API Version:** v1  
**Backend:** Go + Gin  
**Frontend:** Vite + React + TypeScript  
**Database:** PostgreSQL

---

## 1. API General Standards

### 1.1 Base URL

Development:

```text
http://localhost:4000/api/v1
```

Production example:

```text
https://api.example.com/api/v1
```

Environment variable:

Backend:

```env
APP_PORT=4000
APP_ENV=development
APP_TIMEZONE=Asia/Jakarta
DATABASE_URL=postgresql://user:password@localhost:5432/projectflow
JWT_ACCESS_SECRET=change-me
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=7d
WEB_ORIGIN=http://localhost:5173
```

Frontend:

```env
VITE_API_BASE_URL=http://localhost:4000/api/v1
```

Never commit production secrets.

### 1.2 HTTP Methods

| Method | Purpose |
|---|---|
| GET | Retrieve resources |
| POST | Create resources or perform actions |
| PUT | Replace/update a resource |
| PATCH | Partial update |
| DELETE | Delete or deactivate a resource where permitted |

### 1.3 Content Type

Default:

```http
Content-Type: application/json
Accept: application/json
```

File uploads:

```http
Content-Type: multipart/form-data
```

### 1.4 Authentication

Protected endpoints use:

```http
Authorization: Bearer <access_token>
```

Refresh token is managed through a Secure, HttpOnly cookie.

Browser requests that require refresh/logout cookies must use:

```typescript
withCredentials: true
```

CORS must allow the configured frontend origin and credentials.
Refresh and logout requests must also send the configured `Origin`; requests with a missing or untrusted origin are rejected. The refresh cookie uses the `/api/v1/auth` path, `SameSite=Lax`, and `Secure` in production. Refresh tokens are opaque, stored only as hashes, rotated on refresh, and revoked on logout or replay.

The backend requires `JWT_ACCESS_SECRET` to be at least 32 characters in production. Configure access and refresh token lifetimes with `JWT_ACCESS_TTL` and `JWT_REFRESH_TTL`; refresh lifetime must exceed access lifetime.

---

## 2. Standard Response Format

### 2.1 Success Response

Single resource:

```json
{
  "success": true,
  "message": "Project retrieved successfully",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "project_code": "2026-AHA-RKAP-001",
    "name": "Project Management System"
  }
}
```

Collection response:

```json
{
  "success": true,
  "message": "Projects retrieved successfully",
  "data": [],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

### 2.2 Error Response

All API errors must use the same format.

```json
{
  "success": false,
  "message": "Validation failed",
  "error": {
    "code": "VALIDATION_ERROR",
    "details": [
      {
        "field": "name",
        "message": "Project name is required"
      }
    ]
  }
}
```

### 2.3 HTTP Status Codes

| Code | Meaning |
|---|---|
| 200 | Successful request |
| 201 | Resource created |
| 204 | Successful request without response body |
| 400 | Bad request |
| 401 | Unauthenticated |
| 403 | Forbidden |
| 404 | Resource not found |
| 409 | Conflict |
| 422 | Validation error |
| 429 | Too many requests |
| 500 | Internal server error |
| 503 | Service unavailable |

### 2.4 Standard Error Codes

- VALIDATION_ERROR
- UNAUTHORIZED
- FORBIDDEN
- RESOURCE_NOT_FOUND
- RESOURCE_CONFLICT
- INVALID_CREDENTIALS
- ACCOUNT_INACTIVE
- TOKEN_EXPIRED
- INVALID_TOKEN
- BUSINESS_RULE_VIOLATION
- INSUFFICIENT_PERMISSION
- INTERNAL_SERVER_ERROR

Never expose database errors, stack traces, or secrets in API responses.

---

## 3. Pagination, Sorting & Filtering

### 3.1 Pagination

Example:

```http
GET /projects?page=1&per_page=20
```

Rules:

- Default page: 1.
- Default per_page: 20.
- Maximum per_page: 100.

### 3.2 Sorting

Example:

```http
GET /projects?sort_by=created_at&sort_order=desc
```

Supported:

- asc
- desc

Only explicitly allowed sorting fields may be used.

### 3.3 Search

Example:

```http
GET /projects?search=management
```

### 3.4 Filtering

Example:

```http
GET /projects?status=IN_PROGRESS&department_id=<uuid>
```

Dates use ISO 8601:

```text
2026-11-09
```

Timestamps use RFC 3339:

```text
2026-11-09T10:30:00+07:00
```

---

# 4. Authentication API

## 4.1 Login

```http
POST /auth/login
```

Request:

```json
{
  "email": "admin@example.com",
  "password": "SecurePassword123"
}
```

Response:

```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "access_token": "jwt-access-token",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "full_name": "Admin User",
      "email": "admin@example.com",
      "roles": ["ADMIN"],
      "permissions": [
        "projects.read",
        "projects.create",
        "contracts.read"
      ]
    }
  }
}
```

Response must also set the refresh token cookie.

## 4.2 Refresh Token

```http
POST /auth/refresh
```

Cookie:

```http
Cookie: refresh_token=<opaque-refresh-token>
```

Response:

```json
{
  "success": true,
  "message": "Token refreshed",
  "data": {
    "access_token": "new-access-token",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

The refresh token must be rotated and the previous token invalidated.

## 4.3 Logout

```http
POST /auth/logout
```

Requires authentication and refresh cookie.

Response:

```json
{
  "success": true,
  "message": "Logout successful",
  "data": null
}
```

Revoke the refresh token and clear the cookie.

## 4.4 Get Current User

```http
GET /auth/me
```

Response:

```json
{
  "success": true,
  "message": "Profile retrieved",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "full_name": "Admin User",
    "email": "admin@example.com",
    "department": {
      "id": "650e8400-e29b-41d4-a716-446655440000",
      "name": "IT Department"
    },
    "roles": ["ADMIN"],
    "permissions": [
      "projects.read",
      "projects.create",
      "contracts.read"
    ]
  }
}
```

---

# 5. User Management API

Permissions:

- users.read
- users.create
- users.update
- users.deactivate

### Endpoints

```http
GET    /users
POST   /users
GET    /users/:id
PUT    /users/:id
PATCH  /users/:id/status
GET    /roles
GET    /role-permissions
PUT    /roles/:id/permissions
```

`GET /users` supports `page`, `per_page` (maximum 100), `search`, `department_id`, `is_active`, `sort_by` (`created_at`, `full_name`, or `email`), and `sort_order` (`asc` or `desc`). `PUT /users/:id` accepts the create-user fields; `password` is optional, and changing it revokes the user's active refresh tokens. Password hashes are never returned.

`GET /roles` requires `users.read` and returns role IDs, codes, and names for user assignment. Administrator roles are included only for `SUPER_ADMIN` callers; the server independently validates every role assignment.

`GET /role-permissions` and `PUT /roles/:id/permissions` are restricted to `SUPER_ADMIN`. They configure global permission grants by system role; the `SUPER_ADMIN` role is locked to preserve a recovery administrator. Permission changes are audited and take effect on the next authenticated API request.

Create user:

```http
POST /users
```

Request:

```json
{
  "full_name": "John Doe",
  "email": "john@example.com",
  "password": "SecurePassword123",
  "department_id": "650e8400-e29b-41d4-a716-446655440000",
  "role_ids": [
    "750e8400-e29b-41d4-a716-446655440000"
  ]
}
```

Response:

```json
{
  "success": true,
  "message": "User created",
  "data": {
    "id": "850e8400-e29b-41d4-a716-446655440000",
    "full_name": "John Doe",
    "email": "john@example.com",
    "is_active": true
  }
}
```

Never return password hashes.

---

# 6. Department API

Permissions:

- departments.read
- departments.create
- departments.update

### Endpoints

```http
GET    /departments
POST   /departments
GET    /departments/:id
PUT    /departments/:id
PATCH  /departments/:id/status
```

Create department:

```json
{
  "code": "IT",
  "name": "Information Technology",
  "description": "IT Department"
}
```

---

# 7. Project Management API

Permissions:

- projects.read
- projects.create
- projects.update
- projects.archive

### 7.1 List Projects

```http
GET /projects
```

Query parameters:

```text
page
per_page
search
status
department_id
manager_id
priority
sort_by
sort_order
include_archived
```

`sort_by` accepts `created_at`, `name`, `project_code`, `start_date`, `target_end_date`, or `status`. Pagination uses `page` and `per_page` (maximum 100). The response includes pagination metadata.

### 7.2 Create Project

```http
POST /projects
```

Request:

```json
{
  "name": "ProjectFlow Implementation",
  "description": "Internal project management implementation",
  "category": "RKAP",
  "department_id": "650e8400-e29b-41d4-a716-446655440000",
  "owner_id": "550e8400-e29b-41d4-a716-446655440000",
  "manager_id": "750e8400-e29b-41d4-a716-446655440000",
  "start_date": "2026-11-01",
  "target_end_date": "2027-03-31",
  "approved_budget": "500000000.00",
  "currency": "IDR",
  "priority": "HIGH",
  "status": "DRAFT"
}
```

Backend generates project_code.

Generated codes use `YEAR-OWNER-CATEGORY-RUNNING_NUMBER`. `YEAR` comes from `start_date`; `OWNER` uses the project's owner initials (or up to three letters for a single-word name); `CATEGORY` uses the normalized category, defaulting to `GENERAL`. The sequence is independent for each year/owner/category combination and is allocated transactionally, so concurrent creates do not reuse codes.

Response:

```json
{
  "success": true,
  "message": "Project created successfully",
  "data": {
    "id": "850e8400-e29b-41d4-a716-446655440000",
    "project_code": "2026-AHA-RKAP-001",
    "name": "ProjectFlow Implementation",
    "status": "DRAFT",
    "approved_budget": "500000000.00",
    "currency": "IDR",
    "progress": 0
  }
}
```

Financial values are represented as decimal strings to avoid JavaScript floating-point precision problems.

### 7.3 Get Project Detail

```http
GET /projects/:id
```

### 7.4 Update Project

`PUT /projects/:id` updates the project fields shown in the request example. `description`, `category`, `approved_budget`, and `currency` may be omitted to retain their current values. Project codes are immutable after creation.

### 7.5 Update Project Status

```http
PATCH /projects/:id/status
```

Request (the reason is optional and is recorded with the status change):

```json
{
  "status": "ON_HOLD",
  "reason": "Pending management decision"
}
```

### 7.6 Update Project Progress

```http
PATCH /projects/:id/progress
```

Request (`progress` must be between 0 and 100):

```json
{
  "progress": 42.5
}
```

### 7.7 Project Members

```http
GET    /projects/:id/members
POST   /projects/:id/members
DELETE /projects/:id/members/:user_id
GET    /projects/:id/permissions
GET    /projects/:id/role-permissions
PUT    /projects/:id/role-permissions
```

Add member:

```json
{
  "user_id": "750e8400-e29b-41d4-a716-446655440000",
  "project_role": "MEMBER"
}
```

Members must be active users in the project's department. `project_role` accepts `MEMBER`, `LEAD`, or `CONTRIBUTOR` and defaults to `MEMBER`.

Project owners or managers with `projects.update` can configure task and milestone permissions for each project role. The default grants are: `MEMBER` can read/update/assign tasks; `LEAD` can read/create/update/assign tasks and read/create/update milestones; `CONTRIBUTOR` can read/update tasks and read milestones. Saving a role's permission set replaces its defaults for that project. Project role settings override global grants for these project-scoped permissions when a user is a project member. System administration, finance, and other global permissions remain controlled by the global role matrix.

`GET /projects/:id/permissions` returns the current user's effective project-scoped task and milestone permissions.

### 7.8 Archive Project

```http
POST /projects/:id/archive
```

Archiving must preserve historical records.

Archived projects are hidden from normal lists but remain readable by authorized users; pass `include_archived=true` to include them in list results. Archive sets an archive timestamp rather than deleting the project.

Project reads are limited to the caller's department, projects they own/manage or belong to, and global read roles. Only an administrator or the project's owner/manager in the project department may update, manage members, change status/progress, or archive it. Non-administrator project creation and assignments are restricted to the caller's department, and the caller must be the project's owner or manager.

---

# 8. Contract Management API

Permissions:

- contracts.read
- contracts.create
- contracts.update
- contracts.renew
- contracts.documents.upload

### 8.1 List Contracts

```http
GET /contracts
```

Query parameters:

```text
page
per_page
search
status
department_id
owner_id
project_id
expiration_from
expiration_to
sort_by
sort_order
```

### 8.2 Create Contract

```http
POST /contracts
```

Request:

```json
{
  "contract_number": "CTR-2026-001",
  "name": "IT Infrastructure Maintenance",
  "contract_type": "SERVICE",
  "vendor_name": "ABC Technology",
  "owner_id": "550e8400-e29b-41d4-a716-446655440000",
  "department_id": "650e8400-e29b-41d4-a716-446655440000",
  "project_id": "850e8400-e29b-41d4-a716-446655440000",
  "start_date": "2026-01-01",
  "effective_date": "2026-01-01",
  "end_date": "2026-12-31",
  "contract_value": "250000000.00",
  "currency": "IDR",
  "renewal_type": "MANUAL",
  "renewal_notice_days": 90,
  "description": "Infrastructure maintenance contract"
}
```

Response:

```json
{
  "success": true,
  "message": "Contract created successfully",
  "data": {
    "id": "950e8400-e29b-41d4-a716-446655440000",
    "contract_number": "CTR-2026-001",
    "name": "IT Infrastructure Maintenance",
    "end_date": "2026-12-31",
    "days_remaining": 52,
    "expiration_level": "WARNING",
    "status": "ACTIVE"
  }
}
```

The example `days_remaining` and expiration level are illustrative; actual values must be calculated by the backend at request time.

`renewal_type` accepts `NONE`, `MANUAL`, or `AUTOMATIC`. The default is `MANUAL`; omitted `renewal_notice_days` defaults to 90. `status` is optional and defaults to `ACTIVE`. Contract values are non-negative decimal strings with at most two fractional digits. The owner must be an active user in the selected department; a related project, when provided, must be active and in that department.

Expiration levels are calculated against the current date in `APP_TIMEZONE`: `NORMAL` (>90 days), `EARLY_WARNING` (60–90), `WARNING` (30–59), `HIGH_PRIORITY` (14–29), `CRITICAL` (1–13), and `EXPIRED` (0 or less). These are computed response fields; contract records are not rewritten by reads.

### 8.3 Get Contract Detail

```http
GET /contracts/:id
```

Response must include:

- Contract information.
- Owner.
- Department.
- Related project.
- Expiration information.
- Renewal history.
- Documents.

### 8.4 Update Contract

```http
PUT /contracts/:id
```

Request uses the editable contract fields defined in the create request. When `status`, `renewal_type`, or `renewal_notice_days` are omitted, their existing values are retained.

### 8.5 Expiring Contracts

```http
GET /contracts/expiring
```

Query:

```text
within_days=90
page=1
per_page=20
```

Response:

```json
{
  "success": true,
  "message": "Expiring contracts retrieved",
  "data": [
    {
      "id": "950e8400-e29b-41d4-a716-446655440000",
      "contract_number": "CTR-2026-001",
      "name": "IT Infrastructure Maintenance",
      "vendor_name": "ABC Technology",
      "end_date": "2026-12-31",
      "days_remaining": 52,
      "expiration_level": "WARNING",
      "owner": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "full_name": "Admin User"
      }
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

### 8.6 Contract Renewal

```http
GET  /contracts/:id/renewals
POST /contracts/:id/renewals
```

Request:

```json
{
  "new_contract_number": "CTR-2027-001",
  "new_start_date": "2027-01-01",
  "new_end_date": "2027-12-31",
  "new_contract_value": "275000000.00",
  "currency": "IDR",
  "notes": "Annual contract renewal"
}
```

Renewal must preserve the original contract.

Renewal creates a new contract using the original contract's vendor, owner, department, project, and renewal settings; it sets the new effective date to `new_start_date`. The prior contract is retained with `COMPLETED` status and a linked renewal-history record. Only active, expiring-soon, renewal-in-progress, or expired contracts with renewal enabled can be renewed; completed, terminated, and draft contracts cannot. The selected owner/department access rules still apply. `GET /contracts/:id` includes the owner, department, optional project, computed expiration fields, renewal history, and document metadata.

Contract lists support pagination and sorting. `GET /contracts/expiring` returns active, renewal-in-progress, or expiring-soon contracts whose end date falls from today through `within_days` (default 90; maximum 3650), using `APP_TIMEZONE`. Detail access is limited to the caller's department or the caller as owner; administrators and executives have global read access. Writes require an administrator, the contract owner, or a contract manager in the contract's department.

### 8.7 Contract Documents

```http
GET    /contracts/:id/documents
POST   /contracts/:id/documents
DELETE /contracts/:id/documents/:document_id
```

Upload uses multipart/form-data:

```text
file: <binary>
document_type: SIGNED_CONTRACT
```

Document types:

- DRAFT_CONTRACT
- SIGNED_CONTRACT
- ADDENDUM
- SUPPORTING_DOCUMENT
- OTHER

File size and MIME type restrictions must be enforced by the backend.

---

# 9. Task Management API

Permissions:

- tasks.read
- tasks.create
- tasks.update
- tasks.assign

### 9.1 List Project Tasks

```http
GET /projects/:id/tasks
```

Query:

```text
page
per_page
status
priority
assigned_to
overdue
```

### 9.2 My Tasks

```http
GET /tasks/my
```

Query:

```text
status
priority
overdue
page
per_page
```

### 9.3 Create Task

```http
POST /projects/:id/tasks
```

Request:

```json
{
  "title": "Prepare technical documentation",
  "description": "Prepare project technical documents",
  "assigned_to": "750e8400-e29b-41d4-a716-446655440000",
  "priority": "HIGH",
  "status": "TODO",
  "planned_start_date": "2026-11-10",
  "planned_due_date": "2026-11-20",
  "progress": 0
}
```

### 9.4 Update Task

```http
PUT /tasks/:id
```

`GET /tasks/:id` returns task detail. `PUT /tasks/:id` accepts the create fields and updates only the fields supplied. Use an empty string to clear `parent_task_id`, `assigned_to`, `planned_start_date`, or `description`; omitted or `null` values are preserved.

### 9.5 Update Task Status

```http
PATCH /tasks/:id/status
```

Request:

```json
{
  "status": "IN_PROGRESS"
}
```

### 9.6 Update Task Progress

```http
PATCH /tasks/:id/progress
```

Request:

```json
{
  "progress": 50
}
```

### 9.7 Overdue Tasks

```http
GET /tasks/overdue
```

The backend determines overdue status based on the business date, task status, and due date.
Overdue tasks exclude `DONE` and `CANCELLED` tasks; a task due on the current business date is not overdue. `GET /tasks/:id` returns the task detail, including `is_overdue`. The current assignee may update status or progress; task assignment and schedule edits require project management access.
Task completion sets progress to 100 and records the actual start/completion dates on the first applicable transition. Reopening a task does not erase those actual dates. Changes to planned start/due dates are recorded in schedule history.

---

# 10. Milestone API

Permissions:

- milestones.read
- milestones.create
- milestones.update

### Endpoints

```http
GET    /projects/:id/milestones
POST   /projects/:id/milestones
GET    /milestones/:id
PUT    /milestones/:id
PATCH  /milestones/:id/status
GET    /milestones/upcoming
GET    /milestones/delayed
```

Create milestone:

```json
{
  "name": "Requirements Approval",
  "description": "Approval of the project requirements",
  "owner_id": "750e8400-e29b-41d4-a716-446655440000",
  "planned_date": "2026-12-01"
}
```

`PUT /milestones/:id` updates only the fields supplied. Use an empty string to clear `owner_id` or `description`; omitted or `null` values are preserved.

Update status:

```json
{
  "status": "COMPLETED",
  "actual_date": "2026-11-30"
}
```

Milestones planned before the current business date are returned as `DELAYED` unless completed. `GET /milestones/upcoming` accepts `within_days` (default 30, maximum 3650) and includes milestones planned from today through the end of that window. Completing a milestone records `actual_date` from the request or the current business date. Changes to `planned_date` are recorded in schedule history.

---

# 11. Budget API

Permissions:

- budgets.read
- budgets.manage
- budget_transactions.read
- budget_transactions.create

### 11.1 Get Project Budget Summary

```http
GET /projects/:id/budget
```

Response:

```json
{
  "success": true,
  "message": "Budget summary retrieved",
  "data": {
    "project_id": "850e8400-e29b-41d4-a716-446655440000",
    "currency": "IDR",
    "approved_budget": "500000000.00",
    "planned_budget": "480000000.00",
    "committed_cost": "75000000.00",
    "actual_cost": "200000000.00",
    "forecast_cost": "275000000.00",
    "remaining_budget": "225000000.00",
    "budget_variance": "225000000.00",
    "utilization_percentage": 40.00,
    "status": "NORMAL"
  }
}
```

### 11.2 Budget Categories

```http
GET  /budget-categories
GET  /projects/:id/budget/allocations
POST /projects/:id/budget/allocations
PUT  /projects/:id/budget/allocations/:budget_id
```

The standard active categories are PERSONNEL, INFRASTRUCTURE, SOFTWARE_LICENSE, PROCUREMENT, VENDOR_CONSULTANT, OPERATIONAL, and OTHER. Allocation currency must match the project currency; the sum of approved category allocations cannot exceed the project's approved budget.

Allocation request:

```json
{
  "category_id": "a50e8400-e29b-41d4-a716-446655440000",
  "planned_amount": "100000000.00",
  "approved_amount": "90000000.00",
  "currency": "IDR"
}
```

### 11.3 List Transactions

```http
GET /projects/:id/budget/transactions
```

Query:

```text
page
per_page
category_id
transaction_type
status
date_from
date_to
```

### 11.4 Create Transaction

```http
POST /projects/:id/budget/transactions
```

Commitment request:

```json
{
  "budget_id": "b50e8400-e29b-41d4-a716-446655440000",
  "transaction_number": "TRX-2026-001",
  "transaction_type": "COMMITMENT",
  "amount": "75000000.00",
  "transaction_date": "2026-11-09",
  "description": "Vendor procurement commitment",
  "reference_number": "PO-2026-001"
}
```

Expense request:

```json
{
  "budget_id": "b50e8400-e29b-41d4-a716-446655440000",
  "transaction_number": "TRX-2026-002",
  "transaction_type": "EXPENSE",
  "amount": "25000000.00",
  "transaction_date": "2026-11-10",
  "description": "Infrastructure payment",
  "reference_number": "INV-2026-001"
}
```

The initial creation status is DRAFT. An authorized poster may set `status` to POSTED on creation; posting validates linked commitment balances. New transactions cannot be created as CANCELLED. Transaction amounts must be positive decimal strings with at most two fractional digits, and transaction records cannot be edited or deleted.

### 11.5 Transaction State

```http
POST /budget/transactions/:id/post
POST /budget/transactions/:id/cancel
```

Only DRAFT transactions can be posted or cancelled. A POSTED transaction cannot be silently cancelled or edited; use the correction endpoint to atomically retain it as CANCELLED and create an audited POSTED replacement with a required reason:

```http
POST /budget/transactions/:id/correct
```

The correction body contains `transaction_number`, `amount`, `transaction_date`, `reason`, and optional `description`/`reference_number`. Each transaction can be corrected once; linked commitment expenses are checked against the remaining posted commitment amount.

### 11.6 Budget Transactions Response

```json
{
  "success": true,
  "message": "Transactions retrieved",
  "data": [
    {
      "id": "c50e8400-e29b-41d4-a716-446655440000",
      "transaction_number": "TRX-2026-001",
      "transaction_type": "COMMITMENT",
      "amount": "75000000.00",
      "currency": "IDR",
      "transaction_date": "2026-11-09",
      "status": "POSTED"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

Committed cost is the remaining value of posted commitments after linked POSTED expenses; linked expenditures are not double-counted. Actual cost is all POSTED expenses. Forecast is actual plus remaining commitment, remaining/variance is approved budget minus forecast, and utilization is actual divided by approved budget. Draft and cancelled transactions are excluded. Thresholds use actual utilization: below 80% NORMAL, 80%–<90% WARNING, 90%–100% CRITICAL, and above 100% OVER_BUDGET.

---

# 12. Dashboard API

### 12.1 Overview

```http
GET /dashboard/overview
```

Query:

```text
date_from
date_to
department_id
manager_id
```

Response:

```json
{
  "success": true,
  "message": "Dashboard retrieved",
  "data": {
    "projects": {
      "total": 20,
      "active": 12,
      "completed": 5,
      "delayed": 2,
      "on_hold": 1
    },
    "contracts": {
      "active": 15,
      "expiring_soon": 4,
      "expired": 2
    },
    "budget": [
      {
        "currency": "IDR",
        "approved_budget": "5000000000.00",
        "committed_cost": "1000000000.00",
        "actual_cost": "2000000000.00",
        "forecast_cost": "3000000000.00",
        "utilization_percentage": 40.00
      }
    ]
  }
}
```

Overview, project-status, contract-expiration, budget-overview, critical-alerts, upcoming-milestones, and overdue-tasks require `dashboard.read`. Dashboard endpoints honor the caller's project/contract read scope. Supported filters are `date_from`, `date_to`, `department_id`, `manager_id`, and project `status`; timeline lists additionally support `page` and `per_page`, while upcoming milestones support `within_days` (default 30, maximum 365). Project date filters include projects whose planned range overlaps the requested range. Contract date filters select expiration dates. Budget totals include all posted transactions for the selected projects and are grouped by currency; drafts and cancelled transactions are excluded.

### 12.2 Project Status Distribution

```http
GET /dashboard/project-status
```

Returns `total`, a `statuses` count map, and `delayed` (non-completed, non-cancelled projects past their target end date).

### 12.3 Contract Expiration Overview

```http
GET /dashboard/contract-expiration
```

Returns active, expiring-soon, and expired counts. Expiring-soon uses a 30-day window from the configured business date.

### 12.4 Budget Overview

```http
GET /dashboard/budget-overview
```

Returns one summary per currency. Committed cost is the remaining value of posted commitments after linked posted expenses; actual cost is all posted expenses and forecast is their sum.

### 12.5 Critical Alerts

```http
GET /dashboard/critical-alerts
```

Returns counts for delayed projects, delayed milestones, overdue tasks, expiring contracts, and over-budget projects.

### 12.6 Upcoming Milestones

```http
GET /dashboard/upcoming-milestones
```

Returns non-completed milestones due from the business date through `within_days`, with project identifiers/names and pagination metadata.

### 12.7 Overdue Tasks

```http
GET /dashboard/overdue-tasks
```

Returns tasks past their planned due date unless done or cancelled, with project identifiers/names and pagination metadata.

---

# 13. Notification API

Permissions:

- notifications.read
- notifications.update

### Endpoints

```http
GET    /notifications
GET    /notifications/unread-count
PATCH  /notifications/:id/read
PATCH  /notifications/read-all
```

Example:

```http
GET /notifications?page=1&per_page=20
```

Response:

```json
{
  "success": true,
  "message": "Notifications retrieved",
  "data": [
    {
      "id": "d50e8400-e29b-41d4-a716-446655440000",
      "event_type": "CONTRACT_EXPIRATION",
      "title": "Contract Expiring Soon",
      "message": "Contract CTR-2026-001 expires in 7 days",
      "entity_type": "CONTRACT",
      "entity_id": "950e8400-e29b-41d4-a716-446655440000",
      "is_read": false,
      "created_at": "2026-12-24T09:00:00+07:00"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

Users may only access their own notifications unless an authorized administrative endpoint is explicitly provided.

The Compose stack runs a Redis-backed Asynq worker. Contract expiration, milestone delay, and budget threshold monitoring run daily at 08:00 in `APP_TIMEZONE`; overdue task monitoring runs hourly. Project status changes create notifications in the status-update transaction. Notifications use deterministic event keys, and a unique `(notification_id, channel)` delivery log prevents duplicate channel records. Email delivery is retried by Asynq and logged as `PENDING`, `SENT`, or `FAILED`; configure `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, and optionally `SMTP_USERNAME`/`SMTP_PASSWORD` for STARTTLS delivery. When SMTP is not configured, email attempts are recorded as failed rather than silently discarded.

---

# 14. Audit Trail API

Permission: `audit_logs.read` (granted to administrators and executives by default).

```http
GET /audit-logs?page=1&per_page=20&actor_id=<uuid>&action=PROJECT_STATUS_UPDATED
    &entity_type=PROJECT&entity_id=<uuid>&date_from=2026-01-01&date_to=2026-01-31
```

All filters are optional. `date_from` and `date_to` are inclusive calendar dates in `YYYY-MM-DD` format. Results are ordered newest first and use the standard pagination envelope.

Each audit record contains the actor, action, entity type and ID, previous and new JSON values, and creation timestamp. Sensitive credential values are never included. Audit entries are written in the same transaction as the change and are append-only: application and database operations cannot update, delete, or truncate them. Users referenced by audit history cannot be deleted.

---

# 15. Reporting API

**Status:** Planned; these routes are not implemented or available in the current backend. They are excluded from [`docs/openapi.yaml`](./docs/openapi.yaml) until implementation and authorization behavior are verified.

### Project Report

```http
GET /reports/projects
```

Query:

```text
date_from
date_to
department_id
status
manager_id
```

### Contract Expiration Report

```http
GET /reports/contracts/expiration
```

Query:

```text
date_from
date_to
department_id
owner_id
status
```

### Budget Performance Report

```http
GET /reports/budgets
```

Query:

```text
date_from
date_to
department_id
project_id
```

### Export

```http
GET /reports/projects/export?format=csv
GET /reports/contracts/expiration/export?format=csv
GET /reports/budgets/export?format=csv
```

Exports must respect the same authorization and filtering rules as the corresponding report endpoints.

---

# 16. File Upload API

**Status:** Planned; the current backend does not expose a contract-document upload route. It is excluded from [`docs/openapi.yaml`](./docs/openapi.yaml) until storage configuration, upload limits, and authorization behavior are implemented.

MVP supports contract documents.

Upload:

```http
POST /contracts/:id/documents
```

Request:

```text
multipart/form-data

file: contract.pdf
document_type: SIGNED_CONTRACT
```

Response:

```json
{
  "success": true,
  "message": "Document uploaded",
  "data": {
    "id": "e50e8400-e29b-41d4-a716-446655440000",
    "file_name": "contract.pdf",
    "mime_type": "application/pdf",
    "file_size": 1024000,
    "document_type": "SIGNED_CONTRACT",
    "created_at": "2026-11-09T10:00:00+07:00"
  }
}
```

File metadata is stored in PostgreSQL. File bytes are stored in configured object storage.

Do not expose private storage credentials or permanent public URLs.

---

# 17. TypeScript API Contract

The following types must be shared by frontend developers.

```typescript
export type UUID = string;

export type ProjectStatus =
  | "DRAFT"
  | "PROPOSED"
  | "PLANNING"
  | "APPROVED"
  | "IN_PROGRESS"
  | "ON_HOLD"
  | "COMPLETED"
  | "CANCELLED";

export type ContractStatus =
  | "DRAFT"
  | "UNDER_REVIEW"
  | "ACTIVE"
  | "EXPIRING_SOON"
  | "RENEWAL_IN_PROGRESS"
  | "EXPIRED"
  | "TERMINATED"
  | "COMPLETED";

export type TaskStatus =
  | "BACKLOG"
  | "TODO"
  | "IN_PROGRESS"
  | "IN_REVIEW"
  | "BLOCKED"
  | "DONE"
  | "CANCELLED";

export type MilestoneStatus =
  | "PENDING"
  | "IN_PROGRESS"
  | "COMPLETED"
  | "DELAYED";

export type BudgetStatus =
  | "NORMAL"
  | "WARNING"
  | "CRITICAL"
  | "OVER_BUDGET";

export type DecimalString = string;

export interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
  meta?: PaginationMeta;
}

export interface PaginationMeta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export interface ApiError {
  success: false;
  message: string;
  error: {
    code: string;
    details?: Array<{
      field?: string;
      message: string;
    }>;
  };
}

export interface User {
  id: UUID;
  full_name: string;
  email: string;
  is_active: boolean;
  department?: Department | null;
  roles: string[];
  permissions: string[];
}

export interface Department {
  id: UUID;
  code: string;
  name: string;
  description?: string | null;
  is_active: boolean;
}

export interface Project {
  id: UUID;
  project_code: string;
  name: string;
  description?: string | null;
  department_id: UUID;
  owner_id: UUID;
  manager_id: UUID;
  start_date: string;
  target_end_date: string;
  actual_start_date?: string | null;
  actual_end_date?: string | null;
  approved_budget: DecimalString;
  currency: string;
  priority: string;
  status: ProjectStatus;
  progress: number;
  created_at: string;
  updated_at: string;
}

export interface Contract {
  id: UUID;
  project_id?: UUID | null;
  contract_number: string;
  name: string;
  vendor_name: string;
  owner_id: UUID;
  department_id: UUID;
  start_date: string;
  effective_date: string;
  end_date: string;
  contract_value: DecimalString;
  currency: string;
  renewal_type: string;
  renewal_notice_days: number;
  status: ContractStatus;
  days_remaining?: number;
  expiration_level?: string;
}

export interface Task {
  id: UUID;
  project_id: UUID;
  parent_task_id?: UUID | null;
  title: string;
  description?: string | null;
  assigned_to?: UUID | null;
  priority: string;
  status: TaskStatus;
  planned_start_date?: string | null;
  planned_due_date: string;
  actual_start_date?: string | null;
  actual_completion_date?: string | null;
  progress: number;
}

export interface Milestone {
  id: UUID;
  project_id: UUID;
  owner_id?: UUID | null;
  name: string;
  planned_date: string;
  actual_date?: string | null;
  status: MilestoneStatus;
}

export interface BudgetSummary {
  project_id: UUID;
  currency: string;
  approved_budget: DecimalString;
  planned_budget: DecimalString;
  committed_cost: DecimalString;
  actual_cost: DecimalString;
  forecast_cost: DecimalString;
  remaining_budget: DecimalString;
  budget_variance: DecimalString;
  utilization_percentage: number;
  status: BudgetStatus;
}

export interface BudgetTransaction {
  id: UUID;
  budget_id: UUID;
  category_id: UUID;
  category_code: string;
  category_name: string;
  transaction_number: string;
  transaction_type: "COMMITMENT" | "EXPENSE";
  amount: DecimalString;
  currency: string;
  transaction_date: string;
  description?: string | null;
  reference_number?: string | null;
  status: "DRAFT" | "POSTED" | "CANCELLED";
  original_commitment_id?: UUID | null;
  corrects_transaction_id?: UUID | null;
  correction_reason?: string | null;
  created_by: UUID;
  posted_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface Notification {
  id: UUID;
  event_type: string;
  title: string;
  message: string;
  entity_type?: string | null;
  entity_id?: UUID | null;
  is_read: boolean;
  read_at?: string | null;
  created_at: string;
}

export interface AuditLog {
  id: UUID;
  actor_id: UUID | null;
  actor_name: string | null;
  actor_email: string | null;
  action: string;
  entity_type: string;
  entity_id: UUID | null;
  old_value?: unknown;
  new_value?: unknown;
  ip_address?: string | null;
  created_at: string;
}
```

The actual generated TypeScript client or interfaces must match the published OpenAPI specification.

---

# 18. API Permission Matrix

| Resource | Executive | PM | Member | Finance | Contract Manager | Admin |
|---|---|---|---|---|---|---|
| Dashboard | Read | Read | Read | Read | Read | Full |
| Projects | Read | Manage assigned | Read assigned | Read | Read | Full |
| Tasks | Read | Manage assigned | Update assigned | Read | Read | Full |
| Milestones | Read | Manage assigned | Read assigned | Read | Read | Full |
| Contracts | Read | Read linked | Read linked | Read | Manage | Full |
| Budget | Read | Read | No access by default | Manage | Read if authorized | Full |
| Notifications | Own | Own | Own | Own | Own | Admin |
| Audit logs | No access | No access | No access | No access | No access | Super Admin / Admin / Executive |
| Reports | Read | Assigned scope | Restricted | Financial | Contract scope | Full |
| Users | No access | No access | No access | No access | No access | Full |
| Departments | Read | Read | Read | Read | Read | Full |

The table is a baseline policy. Actual permissions must be configurable through roles and permissions.

All resource access must also be checked against the user's permitted department, project membership, or ownership scope.

---

# 19. API Contract Consistency Rules

These rules are mandatory for both repositories.

1. All IDs use UUID strings.
2. All JSON fields use snake_case.
3. All business dates use `YYYY-MM-DD`.
4. All timestamps use RFC 3339.
5. All financial values use decimal strings.
6. All paginated responses use the standard `meta` structure.
7. All errors use the standard `error.code` structure.
8. Status values use uppercase snake case.
9. Backend is the source of truth for calculated values.
10. Frontend must not assume that hidden UI actions are secure.
11. API changes must update the OpenAPI specification.
12. Breaking API changes require explicit versioning or an agreed migration strategy.

---

# 20. OpenAPI Deliverable

Backend must publish:

```text
docs/
├── openapi.yaml
├── swagger.json
└── api-contract.md
```

Frontend should consume the published specification to validate its API integration.

Recommended future workflow:

1. Backend updates OpenAPI.
2. API specification is reviewed.
3. Frontend updates generated TypeScript types.
4. Integration tests validate endpoint compatibility.
5. Both repositories are released with compatible API versions.

---

# 21. MVP API Acceptance Criteria

The shared contract is accepted when:

- Authentication flows work end-to-end.
- User and department APIs work.
- Project CRUD works.
- Contract CRUD works.
- Contract renewal history is available.
- Contract expiration endpoints return accurate classifications.
- Task and milestone endpoints work.
- Budget transactions are recorded correctly.
- Budget summary calculations are accurate.
- Dashboard endpoints return aggregated data.
- Notifications can be retrieved and updated.
- Reports support filtering.
- File uploads work securely.
- RBAC is enforced.
- Error responses are consistent.
- OpenAPI documentation matches implementation.

---

**End of Shared API Contract**