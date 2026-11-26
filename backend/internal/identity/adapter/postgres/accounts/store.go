package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
	accountsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/accounts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// PostgresStore implements Store backed by PostgreSQL via pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a PostgreSQL-backed account store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

var _ accountsapp.Store = (*PostgresStore)(nil)

func scanAccount(row pgx.Row) (*accountsdomain.Account, error) {
	var acc accountsdomain.Account
	err := row.Scan(
		&acc.ID,
		&acc.Email,
		&acc.Nickname,
		&acc.PasswordHash,
		&acc.EmailVerified,
		&acc.Status,
		&acc.Roles,
		&acc.Version,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if acc.Nickname == "" {
		acc.Nickname = accountsdomain.DefaultNickname(acc.Email, "")
	}
	if acc.Roles == nil {
		acc.Roles = []string{}
	}
	return &acc, nil
}

// CreateAccount registers a new account, verification token, and pending mail task atomically.
func (s *PostgresStore) CreateAccount(ctx context.Context, email, password, nickname string) (*accountsdomain.Account, string, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	if normEmail == "" {
		return nil, "", errors.New("email cannot be empty")
	}
	if strings.TrimSpace(nickname) != "" {
		if _, err := accountsdomain.ValidateNickname(nickname); err != nil {
			return nil, "", err
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	acc := &accountsdomain.Account{
		ID:            accountsapp.NewID("usr"),
		Email:         normEmail,
		Nickname:      accountsdomain.DefaultNickname(normEmail, nickname),
		PasswordHash:  string(hash),
		EmailVerified: false,
		Status:        accountsdomain.StatusActive,
		Roles:         []string{"student"},
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO accounts (id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, acc.ID, acc.Email, acc.Nickname, acc.PasswordHash, acc.EmailVerified, acc.Status, acc.Roles, acc.Version, acc.CreatedAt, acc.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, "", accountsdomain.ErrDuplicateEmail
		}
		return nil, "", fmt.Errorf("insert account: %w", err)
	}

	rawToken := accountsapp.GenerateRandomToken(16)
	tHash := accountsapp.HashToken(rawToken)
	_, err = tx.Exec(ctx, `
		INSERT INTO one_time_tokens (token_hash, purpose, target, expires_at, consumed, created_at)
		VALUES ($1, $2, $3, $4, FALSE, $5)
	`, tHash, string(accountsdomain.PurposeEmailVerification), normEmail, now.Add(24*time.Hour), now)
	if err != nil {
		return nil, "", fmt.Errorf("insert verification token: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO mail_tasks (id, recipient, subject, purpose, token, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, accountsapp.NewID("mail"), normEmail, "Verify your email", string(accountsdomain.PurposeEmailVerification), rawToken, "pending", now)
	if err != nil {
		return nil, "", fmt.Errorf("insert mail task: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("commit create account: %w", err)
	}
	return accountsapp.CloneAccount(acc), rawToken, nil
}

// EnsureBootstrapAdmin idempotently creates or upgrades a verified admin account.
func (s *PostgresStore) EnsureBootstrapAdmin(ctx context.Context, email, password string) (*accountsdomain.Account, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	if normEmail == "" || len(password) < 8 {
		return nil, errors.New("valid bootstrap admin email and password (min 8 chars) are required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin bootstrap admin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		SELECT id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
		FROM accounts
		WHERE email = $1
		FOR UPDATE
	`, normEmail)

	acc, err := scanAccount(row)
	if err == nil {
		changed := false
		if !acc.EmailVerified {
			acc.EmailVerified = true
			changed = true
		}
		if acc.Status != accountsdomain.StatusActive {
			acc.Status = accountsdomain.StatusActive
			changed = true
		}
		if !accountsapp.HasRole(acc.Roles, "admin") {
			acc.Roles = append(acc.Roles, "admin")
			changed = true
		}
		if changed {
			acc.Version++
			acc.UpdatedAt = time.Now().UTC()
			_, err = tx.Exec(ctx, `
				UPDATE accounts
				SET email_verified = $1, status = $2, roles = $3, version = $4, updated_at = $5
				WHERE id = $6
			`, acc.EmailVerified, acc.Status, acc.Roles, acc.Version, acc.UpdatedAt, acc.ID)
			if err != nil {
				return nil, fmt.Errorf("update bootstrap admin: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit bootstrap admin: %w", err)
		}
		return accountsapp.CloneAccount(acc), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("query bootstrap admin: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash bootstrap admin password: %w", err)
	}

	now := time.Now().UTC()
	newAcc := &accountsdomain.Account{
		ID:            accountsapp.NewID("usr"),
		Email:         normEmail,
		Nickname:      accountsdomain.DefaultNickname(normEmail, ""),
		PasswordHash:  string(hash),
		EmailVerified: true,
		Status:        accountsdomain.StatusActive,
		Roles:         []string{"student", "admin"},
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO accounts (id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, newAcc.ID, newAcc.Email, newAcc.Nickname, newAcc.PasswordHash, newAcc.EmailVerified, newAcc.Status, newAcc.Roles, newAcc.Version, newAcc.CreatedAt, newAcc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert bootstrap admin: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit bootstrap admin insert: %w", err)
	}
	return accountsapp.CloneAccount(newAcc), nil
}

// Authenticate verifies email and password against stored hash.
func (s *PostgresStore) Authenticate(ctx context.Context, email, password string) (*accountsdomain.Account, error) {
	acc, err := s.GetAccountByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, accountsdomain.ErrAccountNotFound) {
			return nil, accountsdomain.ErrInvalidPassword
		}
		return nil, err
	}
	if acc.Status == accountsdomain.StatusSuspended {
		return nil, accountsdomain.ErrAccountSuspended
	}
	if err := bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(password)); err != nil {
		return nil, accountsdomain.ErrInvalidPassword
	}
	return acc, nil
}

// GetAccountByEmail looks up an account by normalized email.
func (s *PostgresStore) GetAccountByEmail(ctx context.Context, email string) (*accountsdomain.Account, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
		FROM accounts
		WHERE email = $1
	`, normEmail)
	acc, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accountsdomain.ErrAccountNotFound
		}
		return nil, err
	}
	return acc, nil
}

// GetAccountByID retrieves an account by ID.
func (s *PostgresStore) GetAccountByID(ctx context.Context, id string) (*accountsdomain.Account, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
		FROM accounts
		WHERE id = $1
	`, id)
	acc, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accountsdomain.ErrAccountNotFound
		}
		return nil, err
	}
	return acc, nil
}

// ListAccounts queries filtered accounts ordered by created_at DESC with pagination.
func (s *PostgresStore) ListAccounts(ctx context.Context, filter accountsdomain.ListAccountsFilter) ([]*accountsdomain.Account, int, error) {
	page, pageSize := accountsapp.NormalizePagination(filter.Page, filter.PageSize)

	var clauses []string
	var args []any
	argIdx := 1

	if status := strings.TrimSpace(filter.Status); status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if role := strings.TrimSpace(filter.Role); role != "" {
		clauses = append(clauses, fmt.Sprintf("$%d = ANY(roles)", argIdx))
		args = append(args, role)
		argIdx++
	}
	if q := strings.ToLower(strings.TrimSpace(filter.Query)); q != "" {
		clauses = append(clauses, fmt.Sprintf("(LOWER(email) LIKE $%d OR LOWER(nickname) LIKE $%d OR LOWER(id) LIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+q+"%")
		argIdx++
	}

	whereSQL := ""
	if len(clauses) > 0 {
		whereSQL = " WHERE " + strings.Join(clauses, " AND ")
	}

	var total int
	countSQL := "SELECT COUNT(*) FROM accounts" + whereSQL
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count accounts: %w", err)
	}

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
		FROM accounts%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)
	listArgs := append(append([]any(nil), args...), pageSize, offset)

	rows, err := s.pool.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	items := make([]*accountsdomain.Account, 0)
	for rows.Next() {
		acc, err := scanAccount(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan account: %w", err)
		}
		items = append(items, acc)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate accounts: %w", err)
	}
	return items, total, nil
}

// UpdateNickname updates an account's display nickname and increments version.
func (s *PostgresStore) UpdateNickname(ctx context.Context, id, nickname string) (*accountsdomain.Account, error) {
	trimmed, err := accountsdomain.ValidateNickname(nickname)
	if err != nil {
		return nil, err
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE accounts
		SET nickname = $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND status <> $3
		RETURNING id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
	`, trimmed, id, accountsdomain.StatusSuspended)
	acc, err := scanAccount(row)
	if err == nil {
		return acc, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	existing, lookupErr := s.GetAccountByID(ctx, id)
	if lookupErr != nil {
		return nil, lookupErr
	}
	if existing.Status == accountsdomain.StatusSuspended {
		return nil, accountsdomain.ErrAccountSuspended
	}
	return nil, accountsdomain.ErrAccountNotFound
}

// VerifyEmailWithToken atomically validates and consumes the verification token.
func (s *PostgresStore) VerifyEmailWithToken(ctx context.Context, rawToken string) (*accountsdomain.Account, error) {
	tHash := accountsapp.HashToken(rawToken)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin verify tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var purpose, target string
	var expiresAt time.Time
	var consumed bool
	err = tx.QueryRow(ctx, `
		SELECT purpose, target, expires_at, consumed
		FROM one_time_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, tHash).Scan(&purpose, &target, &expiresAt, &consumed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accountsdomain.ErrTokenNotFound
		}
		return nil, err
	}
	if accountsdomain.TokenPurpose(purpose) != accountsdomain.PurposeEmailVerification {
		return nil, accountsdomain.ErrTokenPurposeMismatch
	}
	if time.Now().After(expiresAt) {
		return nil, accountsdomain.ErrTokenExpired
	}
	if consumed {
		row := tx.QueryRow(ctx, `
			SELECT id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
			FROM accounts
			WHERE email = $1
		`, target)
		acc, scanErr := scanAccount(row)
		if scanErr != nil {
			return nil, accountsdomain.ErrTokenConsumed
		}
		_ = tx.Commit(ctx)
		return acc, nil
	}

	if _, err := tx.Exec(ctx, `UPDATE one_time_tokens SET consumed = TRUE WHERE token_hash = $1`, tHash); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE accounts
		SET email_verified = TRUE, version = version + 1, updated_at = NOW()
		WHERE email = $1
		RETURNING id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
	`, target)
	acc, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accountsdomain.ErrAccountNotFound
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return acc, nil
}

// CreateVerificationToken inserts a new verification token and pending mail task.
func (s *PostgresStore) CreateVerificationToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	if _, err := s.GetAccountByEmail(ctx, normEmail); err != nil {
		return "", err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	rawToken := accountsapp.GenerateRandomToken(16)
	tHash := accountsapp.HashToken(rawToken)
	if _, err := tx.Exec(ctx, `
		INSERT INTO one_time_tokens (token_hash, purpose, target, expires_at, consumed, created_at)
		VALUES ($1, $2, $3, $4, FALSE, $5)
	`, tHash, string(accountsdomain.PurposeEmailVerification), normEmail, now.Add(ttl), now); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO mail_tasks (id, recipient, subject, purpose, token, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, accountsapp.NewID("mail"), normEmail, "Verify your email", string(accountsdomain.PurposeEmailVerification), rawToken, "pending", now); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return rawToken, nil
}

// CreatePasswordResetToken creates a single-use password reset token if the email exists.
func (s *PostgresStore) CreatePasswordResetToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	if _, err := s.GetAccountByEmail(ctx, normEmail); err != nil {
		if errors.Is(err, accountsdomain.ErrAccountNotFound) {
			return "", nil
		}
		return "", err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	rawToken := accountsapp.GenerateRandomToken(16)
	tHash := accountsapp.HashToken(rawToken)
	if _, err := tx.Exec(ctx, `
		INSERT INTO one_time_tokens (token_hash, purpose, target, expires_at, consumed, created_at)
		VALUES ($1, $2, $3, $4, FALSE, $5)
	`, tHash, string(accountsdomain.PurposePasswordReset), normEmail, now.Add(ttl), now); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO mail_tasks (id, recipient, subject, purpose, token, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, accountsapp.NewID("mail"), normEmail, "Reset your password", string(accountsdomain.PurposePasswordReset), rawToken, "pending", now); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return rawToken, nil
}

// ResetPasswordWithToken atomically consumes the reset token and updates the password hash.
func (s *PostgresStore) ResetPasswordWithToken(ctx context.Context, rawToken, newPassword string) (*accountsdomain.Account, error) {
	tHash := accountsapp.HashToken(rawToken)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var purpose, target string
	var expiresAt time.Time
	var consumed bool
	err = tx.QueryRow(ctx, `
		SELECT purpose, target, expires_at, consumed
		FROM one_time_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, tHash).Scan(&purpose, &target, &expiresAt, &consumed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accountsdomain.ErrTokenNotFound
		}
		return nil, err
	}
	if accountsdomain.TokenPurpose(purpose) != accountsdomain.PurposePasswordReset {
		return nil, accountsdomain.ErrTokenPurposeMismatch
	}
	if time.Now().After(expiresAt) {
		return nil, accountsdomain.ErrTokenExpired
	}
	if consumed {
		return nil, accountsdomain.ErrTokenConsumed
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE one_time_tokens SET consumed = TRUE WHERE token_hash = $1`, tHash); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE accounts
		SET password_hash = $1, version = version + 1, updated_at = NOW()
		WHERE email = $2
		RETURNING id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
	`, string(hash), target)
	acc, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accountsdomain.ErrAccountNotFound
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return acc, nil
}

// SetAccountStatus updates account status with optimistic concurrency check.
func (s *PostgresStore) SetAccountStatus(ctx context.Context, id, status string, expectedVersion int) (*accountsdomain.Account, error) {
	if err := accountsdomain.ValidateStatus(status); err != nil {
		return nil, err
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE accounts
		SET status = $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3
		RETURNING id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
	`, status, id, expectedVersion)
	acc, err := scanAccount(row)
	if err == nil {
		return acc, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if _, lookupErr := s.GetAccountByID(ctx, id); lookupErr != nil {
		return nil, lookupErr
	}
	return nil, accountsdomain.ErrVersionConflict
}

// AddRole appends a role to an account if not already present.
func (s *PostgresStore) AddRole(ctx context.Context, id, role string) (*accountsdomain.Account, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		SELECT id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`, id)
	acc, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accountsdomain.ErrAccountNotFound
		}
		return nil, err
	}
	if accountsapp.HasRole(acc.Roles, role) {
		_ = tx.Commit(ctx)
		return acc, nil
	}

	acc.Roles = append(acc.Roles, role)
	acc.Version++
	acc.UpdatedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE accounts
		SET roles = $1, version = $2, updated_at = $3
		WHERE id = $4
	`, acc.Roles, acc.Version, acc.UpdatedAt, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return acc, nil
}

// RevokeRole removes a role with optimistic concurrency check.
func (s *PostgresStore) RevokeRole(ctx context.Context, id, role string, expectedVersion int) (*accountsdomain.Account, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		SELECT id, email, nickname, password_hash, email_verified, status, roles, version, created_at, updated_at
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`, id)
	acc, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, accountsdomain.ErrAccountNotFound
		}
		return nil, err
	}
	if acc.Version != expectedVersion {
		return nil, accountsdomain.ErrVersionConflict
	}

	newRoles := make([]string, 0, len(acc.Roles))
	for _, r := range acc.Roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	acc.Roles = newRoles
	acc.Version++
	acc.UpdatedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE accounts
		SET roles = $1, version = $2, updated_at = $3
		WHERE id = $4
	`, acc.Roles, acc.Version, acc.UpdatedAt, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return acc, nil
}

// CountAccounts returns total accounts in PostgreSQL.
func (s *PostgresStore) CountAccounts(ctx context.Context) int {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		return 0
	}
	return count
}

// RecordMailTask persists an outbound mail task record.
func (s *PostgresStore) RecordMailTask(ctx context.Context, task accountsdomain.MailTask) error {
	if task.ID == "" {
		task.ID = accountsapp.NewID("mail")
	}
	task.Recipient = accountsdomain.NormalizeEmail(task.Recipient)
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO mail_tasks (id, recipient, subject, purpose, token, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, task.ID, task.Recipient, task.Subject, string(task.Purpose), task.Token, task.Status, task.CreatedAt)
	return err
}

// UpdateMailTaskStatus updates the status of the latest matching mail task.
func (s *PostgresStore) UpdateMailTaskStatus(ctx context.Context, recipient string, purpose accountsdomain.TokenPurpose, rawToken, status string) error {
	norm := accountsdomain.NormalizeEmail(recipient)
	_, err := s.pool.Exec(ctx, `
		UPDATE mail_tasks
		SET status = $1
		WHERE id = (
			SELECT id FROM mail_tasks
			WHERE recipient = $2 AND purpose = $3 AND token = $4
			ORDER BY created_at DESC
			LIMIT 1
		)
	`, status, norm, string(purpose), rawToken)
	return err
}

// GetRecentMailTasks retrieves recent mail tasks for a recipient (or all if empty).
func (s *PostgresStore) GetRecentMailTasks(ctx context.Context, recipient string) []accountsdomain.MailTask {
	norm := accountsdomain.NormalizeEmail(recipient)
	var rows pgx.Rows
	var err error
	if norm == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, recipient, subject, purpose, COALESCE(token, ''), status, created_at
			FROM mail_tasks
			ORDER BY created_at ASC
			LIMIT 100
		`)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, recipient, subject, purpose, COALESCE(token, ''), status, created_at
			FROM mail_tasks
			WHERE recipient = $1
			ORDER BY created_at ASC
			LIMIT 100
		`, norm)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []accountsdomain.MailTask
	for rows.Next() {
		var t accountsdomain.MailTask
		var purpose string
		if scanErr := rows.Scan(&t.ID, &t.Recipient, &t.Subject, &purpose, &t.Token, &t.Status, &t.CreatedAt); scanErr == nil {
			t.Purpose = accountsdomain.TokenPurpose(purpose)
			result = append(result, t)
		}
	}
	return result
}
