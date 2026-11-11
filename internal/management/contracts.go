package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"projectflow-backend/internal/auth"
)

var contractValuePattern = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,2})?$`)

var (
	contractStatuses = map[string]struct{}{
		"DRAFT": {}, "UNDER_REVIEW": {}, "ACTIVE": {}, "EXPIRING_SOON": {},
		"RENEWAL_IN_PROGRESS": {}, "EXPIRED": {}, "TERMINATED": {}, "COMPLETED": {},
	}
	renewalTypes = map[string]struct{}{
		"NONE": {}, "MANUAL": {}, "AUTOMATIC": {},
	}
)

type Contract struct {
	ID                string             `json:"id"`
	ContractNumber    string             `json:"contract_number"`
	Name              string             `json:"name"`
	ContractType      *string            `json:"contract_type"`
	VendorName        string             `json:"vendor_name"`
	Owner             ContractOwner      `json:"owner"`
	Department        ContractDepartment `json:"department"`
	Project           *ContractProject   `json:"project"`
	StartDate         string             `json:"start_date"`
	EffectiveDate     string             `json:"effective_date"`
	EndDate           string             `json:"end_date"`
	ContractValue     string             `json:"contract_value"`
	Currency          string             `json:"currency"`
	RenewalType       string             `json:"renewal_type"`
	RenewalNoticeDays int                `json:"renewal_notice_days"`
	Status            string             `json:"status"`
	Description       *string            `json:"description"`
	DaysRemaining     int                `json:"days_remaining"`
	ExpirationLevel   string             `json:"expiration_level"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	RenewalHistory    []ContractRenewal  `json:"renewal_history"`
	Documents         []ContractDocument `json:"documents"`
}

type ContractOwner struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
}

type ContractDepartment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ContractProject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectCode string `json:"project_code"`
}

type ContractDocument struct {
	ID               string    `json:"id"`
	AttachmentID     string    `json:"attachment_id"`
	OriginalFilename string    `json:"original_filename"`
	ContentType      string    `json:"content_type"`
	SizeBytes        int64     `json:"size_bytes"`
	DocumentType     string    `json:"document_type"`
	Version          int       `json:"version"`
	UploadedBy       string    `json:"uploaded_by"`
	CreatedAt        time.Time `json:"created_at"`
}

type ContractRenewal struct {
	ID                 string    `json:"id"`
	OriginalContractID string    `json:"original_contract_id"`
	RenewedContractID  string    `json:"renewed_contract_id"`
	ContractNumber     string    `json:"contract_number"`
	ContractName       string    `json:"contract_name"`
	RenewalDate        string    `json:"renewal_date"`
	Status             string    `json:"status"`
	Notes              *string   `json:"notes"`
	CreatedBy          string    `json:"created_by"`
	CreatedAt          time.Time `json:"created_at"`
}

type ContractInput struct {
	ContractNumber    string  `json:"contract_number"`
	Name              string  `json:"name"`
	ContractType      *string `json:"contract_type"`
	VendorName        string  `json:"vendor_name"`
	OwnerID           string  `json:"owner_id"`
	DepartmentID      string  `json:"department_id"`
	ProjectID         *string `json:"project_id"`
	StartDate         string  `json:"start_date"`
	EffectiveDate     string  `json:"effective_date"`
	EndDate           string  `json:"end_date"`
	ContractValue     string  `json:"contract_value"`
	Currency          string  `json:"currency"`
	RenewalType       string  `json:"renewal_type"`
	RenewalNoticeDays *int    `json:"renewal_notice_days"`
	Status            string  `json:"status"`
	Description       *string `json:"description"`
}

type ContractRenewalInput struct {
	NewContractNumber string  `json:"new_contract_number"`
	NewStartDate      string  `json:"new_start_date"`
	NewEndDate        string  `json:"new_end_date"`
	NewContractValue  string  `json:"new_contract_value"`
	Currency          string  `json:"currency"`
	Notes             *string `json:"notes"`
}

type ContractFilter struct {
	Page           int
	PerPage        int
	Search         string
	Status         string
	DepartmentID   string
	OwnerID        string
	ProjectID      string
	ExpirationFrom string
	ExpirationTo   string
	SortBy         string
	SortOrder      string
	ExpiringWithin int
	ExpiringOnly   bool
}

type cleanContractInput struct {
	ContractNumber    string
	Name              string
	ContractType      any
	VendorName        string
	OwnerID           string
	DepartmentID      string
	ProjectID         any
	StartDate         string
	EffectiveDate     string
	EndDate           string
	ContractValue     string
	Currency          string
	RenewalType       string
	RenewalNoticeDays int
	Status            string
	Description       any
}

const contractSelectColumns = `
	c.id::text, c.contract_number, c.name, c.contract_type, c.vendor_name,
	c.owner_id::text, u.full_name, c.department_id::text, d.name,
	c.project_id::text, p.name, p.project_code,
	TO_CHAR(c.start_date, 'YYYY-MM-DD'), TO_CHAR(c.effective_date, 'YYYY-MM-DD'),
	TO_CHAR(c.end_date, 'YYYY-MM-DD'), c.contract_value::text, c.currency,
	c.renewal_type, c.renewal_notice_days, c.status, c.description, c.created_at, c.updated_at`

func (s *Service) ListContracts(ctx context.Context, principal auth.Principal, filter ContractFilter) ([]Contract, PageMeta, error) {
	where, args, err := contractWhere(filter, principal)
	if err != nil {
		return nil, PageMeta{}, err
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM contracts c
		JOIN users u ON u.id=c.owner_id
		JOIN departments d ON d.id=c.department_id
		LEFT JOIN projects p ON p.id=c.project_id
		`+where, args...).Scan(&total); err != nil {
		return nil, PageMeta{}, fmt.Errorf("count contracts: %w", err)
	}
	orderColumns := map[string]string{
		"created_at": "c.created_at", "contract_number": "c.contract_number",
		"name": "c.name", "end_date": "c.end_date", "contract_value": "c.contract_value",
		"status": "c.status",
	}
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "end_date"
	}
	sortOrder := strings.ToUpper(filter.SortOrder)
	if sortOrder == "" {
		sortOrder = "ASC"
	}
	if _, ok := orderColumns[sortBy]; !ok || (sortOrder != "ASC" && sortOrder != "DESC") {
		return nil, PageMeta{}, fmt.Errorf("%w: unsupported contract sort", ErrInvalid)
	}
	page, perPage := filter.Page, filter.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		return nil, PageMeta{}, fmt.Errorf("%w: per_page cannot exceed 100", ErrInvalid)
	}
	queryArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	query := fmt.Sprintf(`
		SELECT %s FROM contracts c
		JOIN users u ON u.id=c.owner_id
		JOIN departments d ON d.id=c.department_id
		LEFT JOIN projects p ON p.id=c.project_id
		%s ORDER BY %s %s, c.id
		LIMIT $%d OFFSET $%d`,
		contractSelectColumns, where, orderColumns[sortBy], sortOrder, len(args)+1, len(args)+2)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, PageMeta{}, fmt.Errorf("list contracts: %w", err)
	}
	defer rows.Close()
	contracts := make([]Contract, 0)
	for rows.Next() {
		contract, err := scanContract(rows, s.businessDate())
		if err != nil {
			return nil, PageMeta{}, err
		}
		contracts = append(contracts, contract)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, fmt.Errorf("iterate contracts: %w", err)
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	return contracts, PageMeta{Page: page, PerPage: perPage, Total: total, TotalPages: totalPages}, nil
}

func (s *Service) Contract(ctx context.Context, id string, principal auth.Principal) (Contract, error) {
	contract, err := s.contractByID(ctx, id)
	if err != nil {
		return Contract{}, err
	}
	if !canReadContract(contract, principal) {
		return Contract{}, ErrNotFound
	}
	contract.RenewalHistory, err = s.contractRenewals(ctx, id)
	if err != nil {
		return Contract{}, err
	}
	contract.Documents, err = s.contractDocuments(ctx, id)
	if err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func (s *Service) CreateContract(ctx context.Context, input ContractInput, principal auth.Principal) (Contract, error) {
	clean, err := validateContractInput(input)
	if err != nil {
		return Contract{}, err
	}
	if !canManageContract(principal, clean.OwnerID, clean.DepartmentID) {
		return Contract{}, ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Contract{}, fmt.Errorf("begin create contract: %w", err)
	}
	defer tx.Rollback()
	if err := validateContractRelations(ctx, tx, clean); err != nil {
		return Contract{}, err
	}
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO contracts (
			contract_number, name, contract_type, vendor_name, owner_id, department_id,
			project_id, start_date, effective_date, end_date, contract_value, currency,
			renewal_type, renewal_notice_days, status, description
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id::text`,
		clean.ContractNumber, clean.Name, clean.ContractType, clean.VendorName, clean.OwnerID, clean.DepartmentID,
		clean.ProjectID, clean.StartDate, clean.EffectiveDate, clean.EndDate, clean.ContractValue, clean.Currency,
		clean.RenewalType, clean.RenewalNoticeDays, clean.Status, clean.Description,
	).Scan(&id)
	if err != nil {
		return Contract{}, translateWriteError("create contract", err)
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "CONTRACT_CREATED", "CONTRACT", id, nil, contractAuditValue(clean)); err != nil {
		return Contract{}, err
	}
	if err := tx.Commit(); err != nil {
		return Contract{}, fmt.Errorf("commit create contract: %w", err)
	}
	return s.Contract(ctx, id, principal)
}

func (s *Service) UpdateContract(ctx context.Context, id string, input ContractInput, principal auth.Principal) (Contract, error) {
	current, err := s.contractByID(ctx, id)
	if err != nil {
		return Contract{}, err
	}
	if !canManageContract(principal, current.Owner.ID, current.Department.ID) {
		return Contract{}, ErrForbidden
	}
	if strings.TrimSpace(input.Status) == "" {
		input.Status = current.Status
	}
	if input.RenewalNoticeDays == nil {
		noticeDays := current.RenewalNoticeDays
		input.RenewalNoticeDays = &noticeDays
	}
	if strings.TrimSpace(input.RenewalType) == "" {
		input.RenewalType = current.RenewalType
	}
	clean, err := validateContractInput(input)
	if err != nil {
		return Contract{}, err
	}
	if !canManageContract(principal, clean.OwnerID, clean.DepartmentID) {
		return Contract{}, ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Contract{}, fmt.Errorf("begin update contract: %w", err)
	}
	defer tx.Rollback()
	var lockedID, lockedOwnerID, lockedDepartmentID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, owner_id::text, department_id::text
		FROM contracts WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID, &lockedOwnerID, &lockedDepartmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Contract{}, ErrNotFound
		}
		return Contract{}, fmt.Errorf("lock contract for update: %w", err)
	}
	if !canManageContract(principal, lockedOwnerID, lockedDepartmentID) {
		return Contract{}, ErrForbidden
	}
	if err := validateContractRelations(ctx, tx, clean); err != nil {
		return Contract{}, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE contracts SET contract_number=$1, name=$2, contract_type=$3, vendor_name=$4,
			owner_id=$5, department_id=$6, project_id=$7, start_date=$8, effective_date=$9,
			end_date=$10, contract_value=$11, currency=$12, renewal_type=$13,
			renewal_notice_days=$14, status=$15, description=$16, updated_at=NOW()
		WHERE id=$17`,
		clean.ContractNumber, clean.Name, clean.ContractType, clean.VendorName, clean.OwnerID,
		clean.DepartmentID, clean.ProjectID, clean.StartDate, clean.EffectiveDate, clean.EndDate,
		clean.ContractValue, clean.Currency, clean.RenewalType, clean.RenewalNoticeDays,
		clean.Status, clean.Description, id)
	if err != nil {
		return Contract{}, translateWriteError("update contract", err)
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "CONTRACT_UPDATED", "CONTRACT", id, current, contractAuditValue(clean)); err != nil {
		return Contract{}, err
	}
	if err := tx.Commit(); err != nil {
		return Contract{}, fmt.Errorf("commit update contract: %w", err)
	}
	return s.Contract(ctx, id, principal)
}

func (s *Service) ExpiringContracts(ctx context.Context, principal auth.Principal, filter ContractFilter) ([]Contract, PageMeta, error) {
	filter.ExpiringOnly = true
	today := s.businessDate()
	filter.ExpirationFrom = today.Format("2006-01-02")
	filter.ExpirationTo = today.AddDate(0, 0, filter.ExpiringWithin).Format("2006-01-02")
	return s.ListContracts(ctx, principal, filter)
}

func (s *Service) ContractRenewals(ctx context.Context, id string, principal auth.Principal) ([]ContractRenewal, error) {
	contract, err := s.contractByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canReadContract(contract, principal) {
		return nil, ErrNotFound
	}
	return s.contractRenewals(ctx, id)
}

func (s *Service) RenewContract(ctx context.Context, id string, input ContractRenewalInput, principal auth.Principal) (Contract, error) {
	contract, err := s.contractByID(ctx, id)
	if err != nil {
		return Contract{}, err
	}
	if !canManageContract(principal, contract.Owner.ID, contract.Department.ID) {
		return Contract{}, ErrForbidden
	}
	if !canRenewContract(contract.RenewalType, contract.Status) {
		return Contract{}, fmt.Errorf("%w: this contract cannot be renewed", ErrInvalid)
	}
	newNumber := strings.TrimSpace(input.NewContractNumber)
	if newNumber == "" || utf8.RuneCountInString(newNumber) > 100 {
		return Contract{}, fmt.Errorf("%w: new_contract_number must contain 1 to 100 characters", ErrInvalid)
	}
	startDate, err := parseProjectDate(input.NewStartDate, "new_start_date")
	if err != nil {
		return Contract{}, err
	}
	endDate, err := parseProjectDate(input.NewEndDate, "new_end_date")
	if err != nil {
		return Contract{}, err
	}
	if endDate.Before(startDate) {
		return Contract{}, fmt.Errorf("%w: new_end_date must not precede new_start_date", ErrInvalid)
	}
	value := strings.TrimSpace(input.NewContractValue)
	if !contractValuePattern.MatchString(value) {
		return Contract{}, fmt.Errorf("%w: new_contract_value must be a non-negative decimal with at most two fractional digits", ErrInvalid)
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(currency) {
		return Contract{}, fmt.Errorf("%w: currency must be a three-letter code", ErrInvalid)
	}
	if input.Notes != nil && utf8.RuneCountInString(*input.Notes) > 5000 {
		return Contract{}, fmt.Errorf("%w: renewal notes are too long", ErrInvalid)
	}
	var notes any
	if input.Notes != nil {
		trimmed := strings.TrimSpace(*input.Notes)
		if trimmed != "" {
			notes = trimmed
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Contract{}, fmt.Errorf("begin contract renewal: %w", err)
	}
	defer tx.Rollback()
	var originalStatus, renewalType, lockedOwnerID, lockedDepartmentID string
	if err := tx.QueryRowContext(ctx, `
		SELECT status, renewal_type, owner_id::text, department_id::text
		FROM contracts WHERE id=$1 FOR UPDATE`, id).Scan(&originalStatus, &renewalType, &lockedOwnerID, &lockedDepartmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Contract{}, ErrNotFound
		}
		return Contract{}, fmt.Errorf("lock original contract for renewal: %w", err)
	}
	if !canManageContract(principal, lockedOwnerID, lockedDepartmentID) {
		return Contract{}, ErrForbidden
	}
	if !canRenewContract(renewalType, originalStatus) {
		return Contract{}, fmt.Errorf("%w: this contract cannot be renewed", ErrInvalid)
	}
	var renewedID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO contracts (
			project_id, contract_number, name, contract_type, vendor_name, owner_id, department_id,
			start_date, effective_date, end_date, contract_value, currency, renewal_type,
			renewal_notice_days, status, description
		) SELECT project_id, $2, name, contract_type, vendor_name, owner_id, department_id,
			$3, $3, $4, $5, $6, renewal_type, renewal_notice_days, 'ACTIVE', description
		FROM contracts WHERE id=$1
		RETURNING id::text`, id, newNumber, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), value, currency,
	).Scan(&renewedID)
	if err != nil {
		return Contract{}, translateWriteError("create renewed contract", err)
	}
	renewalDate := s.businessDate().Format("2006-01-02")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_renewals (
			original_contract_id, renewed_contract_id, renewal_date, status, notes, created_by
		) VALUES ($1,$2,$3,'COMPLETED',$4,$5)`, id, renewedID, renewalDate, notes, principal.User.ID); err != nil {
		return Contract{}, translateWriteError("record contract renewal", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contracts SET status='COMPLETED', updated_at=NOW() WHERE id=$1`, id); err != nil {
		return Contract{}, fmt.Errorf("complete original contract after renewal: %w", err)
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "CONTRACT_RENEWED", "CONTRACT", id,
		map[string]string{"status": originalStatus},
		map[string]any{"status": "COMPLETED", "renewed_contract_id": renewedID, "renewal_date": renewalDate}); err != nil {
		return Contract{}, err
	}
	if err := recordAudit(ctx, tx, principal.User.ID, "CONTRACT_RENEWAL_CREATED", "CONTRACT", renewedID, nil,
		map[string]any{"contract_number": newNumber, "start_date": startDate.Format("2006-01-02"),
			"end_date": endDate.Format("2006-01-02"), "contract_value": value, "currency": currency}); err != nil {
		return Contract{}, err
	}
	if err := tx.Commit(); err != nil {
		return Contract{}, fmt.Errorf("commit contract renewal: %w", err)
	}
	return s.Contract(ctx, renewedID, principal)
}

func contractAuditValue(clean cleanContractInput) map[string]any {
	return map[string]any{
		"contract_number": clean.ContractNumber, "name": clean.Name,
		"contract_type": clean.ContractType, "vendor_name": clean.VendorName,
		"owner_id": clean.OwnerID, "department_id": clean.DepartmentID, "project_id": clean.ProjectID,
		"start_date": clean.StartDate, "effective_date": clean.EffectiveDate, "end_date": clean.EndDate,
		"contract_value": clean.ContractValue, "currency": clean.Currency,
		"renewal_type": clean.RenewalType, "renewal_notice_days": clean.RenewalNoticeDays,
		"status": clean.Status, "description": clean.Description,
	}
}

func (s *Service) contractByID(ctx context.Context, id string) (Contract, error) {
	contract, err := scanContract(s.db.QueryRowContext(ctx, `
		SELECT `+contractSelectColumns+`
		FROM contracts c
		JOIN users u ON u.id=c.owner_id
		JOIN departments d ON d.id=c.department_id
		LEFT JOIN projects p ON p.id=c.project_id
		WHERE c.id=$1`, id), s.businessDate())
	if errors.Is(err, sql.ErrNoRows) {
		return Contract{}, ErrNotFound
	}
	return contract, err
}

func (s *Service) contractRenewals(ctx context.Context, id string) ([]ContractRenewal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cr.id::text, cr.original_contract_id::text, cr.renewed_contract_id::text,
			c.contract_number, c.name, TO_CHAR(cr.renewal_date,'YYYY-MM-DD'),
			cr.status, cr.notes, cr.created_by::text, cr.created_at
		FROM contract_renewals cr
		JOIN contracts c ON c.id=cr.renewed_contract_id
		WHERE cr.original_contract_id=$1 OR cr.renewed_contract_id=$1
		ORDER BY cr.renewal_date, cr.created_at`, id)
	if err != nil {
		return nil, fmt.Errorf("list contract renewal history: %w", err)
	}
	defer rows.Close()
	history := make([]ContractRenewal, 0)
	for rows.Next() {
		var renewal ContractRenewal
		var notes sql.NullString
		if err := rows.Scan(
			&renewal.ID, &renewal.OriginalContractID, &renewal.RenewedContractID,
			&renewal.ContractNumber, &renewal.ContractName, &renewal.RenewalDate,
			&renewal.Status, &notes, &renewal.CreatedBy, &renewal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contract renewal history: %w", err)
		}
		if notes.Valid {
			renewal.Notes = &notes.String
		}
		history = append(history, renewal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract renewal history: %w", err)
	}
	return history, nil
}

func (s *Service) contractDocuments(ctx context.Context, id string) ([]ContractDocument, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cd.id::text, a.id::text, a.original_filename, a.content_type, a.size_bytes,
			cd.document_type, cd.version, cd.uploaded_by::text, cd.created_at
		FROM contract_documents cd
		JOIN attachments a ON a.id=cd.attachment_id
		WHERE cd.contract_id=$1
		ORDER BY cd.version DESC, cd.created_at DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("list contract documents: %w", err)
	}
	defer rows.Close()
	documents := make([]ContractDocument, 0)
	for rows.Next() {
		var document ContractDocument
		if err := rows.Scan(
			&document.ID, &document.AttachmentID, &document.OriginalFilename, &document.ContentType,
			&document.SizeBytes, &document.DocumentType, &document.Version, &document.UploadedBy, &document.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contract document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract documents: %w", err)
	}
	return documents, nil
}

func (s *Service) businessDate() time.Time {
	return time.Now().In(s.timezone)
}

func contractWhere(filter ContractFilter, principal auth.Principal) (string, []any, error) {
	clauses := make([]string, 0, 9)
	args := make([]any, 0, 10)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		clauses = append(clauses, fmt.Sprintf(
			"(c.contract_number ILIKE $%d OR c.name ILIKE $%d OR c.vendor_name ILIKE $%d)", len(args), len(args), len(args)))
	}
	if filter.Status != "" {
		status := strings.ToUpper(strings.TrimSpace(filter.Status))
		if _, ok := contractStatuses[status]; !ok {
			return "", nil, fmt.Errorf("%w: unsupported contract status filter", ErrInvalid)
		}
		add("c.status = $%d", status)
	}
	for _, field := range []struct {
		name   string
		value  string
		column string
	}{
		{"department_id", filter.DepartmentID, "c.department_id"},
		{"owner_id", filter.OwnerID, "c.owner_id"},
		{"project_id", filter.ProjectID, "c.project_id"},
	} {
		if field.value == "" {
			continue
		}
		id, err := normalizeUUID(field.value, field.name)
		if err != nil {
			return "", nil, err
		}
		add(field.column+" = $%d", id)
	}
	if filter.ExpirationFrom != "" {
		date, err := parseProjectDate(filter.ExpirationFrom, "expiration_from")
		if err != nil {
			return "", nil, err
		}
		add("c.end_date >= $%d", date.Format("2006-01-02"))
	}
	if filter.ExpirationTo != "" {
		date, err := parseProjectDate(filter.ExpirationTo, "expiration_to")
		if err != nil {
			return "", nil, err
		}
		add("c.end_date <= $%d", date.Format("2006-01-02"))
	}
	if filter.ExpirationFrom != "" && filter.ExpirationTo != "" && filter.ExpirationTo < filter.ExpirationFrom {
		return "", nil, fmt.Errorf("%w: expiration_to must not precede expiration_from", ErrInvalid)
	}
	if filter.ExpiringOnly {
		clauses = append(clauses, "c.status IN ('ACTIVE','EXPIRING_SOON','RENEWAL_IN_PROGRESS')")
	}
	if !principal.HasRole("ADMIN") && !principal.HasRole("SUPER_ADMIN") && !principal.HasRole("EXECUTIVE") {
		args = append(args, principal.User.ID)
		position := len(args)
		if principal.User.DepartmentID == nil {
			clauses = append(clauses, fmt.Sprintf("c.owner_id = $%d", position))
		} else {
			args = append(args, *principal.User.DepartmentID)
			departmentPosition := len(args)
			clauses = append(clauses, fmt.Sprintf("(c.department_id = $%d OR c.owner_id = $%d)", departmentPosition, position))
		}
	}
	if len(clauses) == 0 {
		return "", args, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

func scanContract(row interface{ Scan(...any) error }, today time.Time) (Contract, error) {
	var contract Contract
	var contractType, projectID, projectName, projectCode, description sql.NullString
	err := row.Scan(
		&contract.ID, &contract.ContractNumber, &contract.Name, &contractType, &contract.VendorName,
		&contract.Owner.ID, &contract.Owner.FullName, &contract.Department.ID, &contract.Department.Name,
		&projectID, &projectName, &projectCode,
		&contract.StartDate, &contract.EffectiveDate, &contract.EndDate, &contract.ContractValue,
		&contract.Currency, &contract.RenewalType, &contract.RenewalNoticeDays, &contract.Status,
		&description, &contract.CreatedAt, &contract.UpdatedAt,
	)
	if err != nil {
		return Contract{}, fmt.Errorf("scan contract: %w", err)
	}
	if contractType.Valid {
		contract.ContractType = &contractType.String
	}
	if description.Valid {
		contract.Description = &description.String
	}
	if projectID.Valid {
		contract.Project = &ContractProject{ID: projectID.String, Name: projectName.String, ProjectCode: projectCode.String}
	}
	contract.DaysRemaining, err = daysUntilEnd(contract.EndDate, today)
	if err != nil {
		return Contract{}, err
	}
	contract.ExpirationLevel = expirationLevel(contract.DaysRemaining)
	return contract, nil
}

func validateContractInput(input ContractInput) (cleanContractInput, error) {
	number := strings.TrimSpace(input.ContractNumber)
	name := strings.TrimSpace(input.Name)
	vendor := strings.TrimSpace(input.VendorName)
	if number == "" || utf8.RuneCountInString(number) > 100 ||
		name == "" || utf8.RuneCountInString(name) > 255 ||
		vendor == "" || utf8.RuneCountInString(vendor) > 255 {
		return cleanContractInput{}, fmt.Errorf("%w: contract number, name, and vendor are required and within their length limits", ErrInvalid)
	}
	departmentID, err := normalizeUUID(input.DepartmentID, "department_id")
	if err != nil {
		return cleanContractInput{}, err
	}
	ownerID, err := normalizeUUID(input.OwnerID, "owner_id")
	if err != nil {
		return cleanContractInput{}, err
	}
	var projectID any
	if input.ProjectID != nil && strings.TrimSpace(*input.ProjectID) != "" {
		id, err := normalizeUUID(*input.ProjectID, "project_id")
		if err != nil {
			return cleanContractInput{}, err
		}
		projectID = id
	}
	start, err := parseProjectDate(input.StartDate, "start_date")
	if err != nil {
		return cleanContractInput{}, err
	}
	effective, err := parseProjectDate(input.EffectiveDate, "effective_date")
	if err != nil {
		return cleanContractInput{}, err
	}
	end, err := parseProjectDate(input.EndDate, "end_date")
	if err != nil {
		return cleanContractInput{}, err
	}
	if start.After(effective) || effective.After(end) {
		return cleanContractInput{}, fmt.Errorf("%w: contract dates must satisfy start_date <= effective_date <= end_date", ErrInvalid)
	}
	value := strings.TrimSpace(input.ContractValue)
	if !contractValuePattern.MatchString(value) {
		return cleanContractInput{}, fmt.Errorf("%w: contract_value must be a non-negative decimal with at most two fractional digits", ErrInvalid)
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if !currencyPattern.MatchString(currency) {
		return cleanContractInput{}, fmt.Errorf("%w: currency must be a three-letter code", ErrInvalid)
	}
	renewalType := strings.ToUpper(strings.TrimSpace(input.RenewalType))
	if renewalType == "" {
		renewalType = "MANUAL"
	}
	if _, ok := renewalTypes[renewalType]; !ok {
		return cleanContractInput{}, fmt.Errorf("%w: unsupported renewal type", ErrInvalid)
	}
	noticeDays := 90
	if input.RenewalNoticeDays != nil {
		noticeDays = *input.RenewalNoticeDays
	}
	if noticeDays < 0 || noticeDays > 3650 {
		return cleanContractInput{}, fmt.Errorf("%w: renewal_notice_days must be between 0 and 3650", ErrInvalid)
	}
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = "ACTIVE"
	}
	if _, ok := contractStatuses[status]; !ok {
		return cleanContractInput{}, fmt.Errorf("%w: unsupported contract status", ErrInvalid)
	}
	if input.ContractType != nil && utf8.RuneCountInString(*input.ContractType) > 100 {
		return cleanContractInput{}, fmt.Errorf("%w: contract_type is too long", ErrInvalid)
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 10000 {
		return cleanContractInput{}, fmt.Errorf("%w: contract description is too long", ErrInvalid)
	}
	var contractType, description any
	if input.ContractType != nil && strings.TrimSpace(*input.ContractType) != "" {
		contractType = strings.TrimSpace(*input.ContractType)
	}
	if input.Description != nil && strings.TrimSpace(*input.Description) != "" {
		description = strings.TrimSpace(*input.Description)
	}
	return cleanContractInput{
		ContractNumber: number, Name: name, ContractType: contractType, VendorName: vendor,
		OwnerID: ownerID, DepartmentID: departmentID, ProjectID: projectID,
		StartDate: start.Format("2006-01-02"), EffectiveDate: effective.Format("2006-01-02"),
		EndDate: end.Format("2006-01-02"), ContractValue: value, Currency: currency,
		RenewalType: renewalType, RenewalNoticeDays: noticeDays, Status: status,
		Description: description,
	}, nil
}

func validateContractRelations(ctx context.Context, tx *sql.Tx, contract cleanContractInput) error {
	var active bool
	var departmentID sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT is_active, department_id::text FROM users WHERE id=$1`, contract.OwnerID).
		Scan(&active, &departmentID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: contract owner does not exist", ErrInvalid)
	}
	if err != nil {
		return fmt.Errorf("validate contract owner: %w", err)
	}
	if !active || !departmentID.Valid || departmentID.String != contract.DepartmentID {
		return fmt.Errorf("%w: contract owner must be active and belong to the contract department", ErrInvalid)
	}
	if contract.ProjectID != nil {
		var projectDepartment string
		var archivedAt sql.NullTime
		err := tx.QueryRowContext(ctx, `
			SELECT department_id::text, archived_at FROM projects WHERE id=$1`, contract.ProjectID).
			Scan(&projectDepartment, &archivedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: related project does not exist", ErrInvalid)
		}
		if err != nil {
			return fmt.Errorf("validate related project: %w", err)
		}
		if projectDepartment != contract.DepartmentID || archivedAt.Valid {
			return fmt.Errorf("%w: related project must be active and in the contract department", ErrInvalid)
		}
	}
	return nil
}

func canReadContract(contract Contract, principal auth.Principal) bool {
	if principal.HasRole("ADMIN") || principal.HasRole("SUPER_ADMIN") || principal.HasRole("EXECUTIVE") {
		return true
	}
	return principal.User.ID == contract.Owner.ID ||
		(principal.User.DepartmentID != nil && *principal.User.DepartmentID == contract.Department.ID)
}

func canManageContract(principal auth.Principal, ownerID, departmentID string) bool {
	if principal.HasRole("ADMIN") || principal.HasRole("SUPER_ADMIN") {
		return true
	}
	return principal.User.DepartmentID != nil &&
		*principal.User.DepartmentID == departmentID &&
		(principal.User.ID == ownerID || principal.HasRole("CONTRACT_MANAGER"))
}

func canRenewContract(renewalType, status string) bool {
	if renewalType == "NONE" {
		return false
	}
	switch status {
	case "ACTIVE", "EXPIRING_SOON", "RENEWAL_IN_PROGRESS", "EXPIRED":
		return true
	default:
		return false
	}
}

func expirationLevel(daysRemaining int) string {
	switch {
	case daysRemaining <= 0:
		return "EXPIRED"
	case daysRemaining <= 13:
		return "CRITICAL"
	case daysRemaining <= 29:
		return "HIGH_PRIORITY"
	case daysRemaining <= 59:
		return "WARNING"
	case daysRemaining <= 90:
		return "EARLY_WARNING"
	default:
		return "NORMAL"
	}
}

func daysUntilEnd(endDate string, today time.Time) (int, error) {
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return 0, fmt.Errorf("parse contract end date: %w", err)
	}
	todayDate, err := time.Parse("2006-01-02", today.Format("2006-01-02"))
	if err != nil {
		return 0, fmt.Errorf("parse business date: %w", err)
	}
	return int(end.Sub(todayDate).Hours() / 24), nil
}

func parseExpiringWithin(value string) (int, error) {
	within := 90
	if strings.TrimSpace(value) != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("%w: within_days must be an integer", ErrInvalid)
		}
		within = parsed
	}
	if within < 1 || within > 3650 {
		return 0, fmt.Errorf("%w: within_days must be between 1 and 3650", ErrInvalid)
	}
	return within, nil
}
