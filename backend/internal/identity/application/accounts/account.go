package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	accountsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/accounts"
	"golang.org/x/crypto/bcrypt"
)

// Store defines persistence operations for accounts, one-time tokens, and mail tasks.
type Store interface {
	CreateAccount(ctx context.Context, email, password, nickname string) (*accountsdomain.Account, string, error)
	EnsureBootstrapAdmin(ctx context.Context, email, password string) (*accountsdomain.Account, error)
	GetAccountByEmail(ctx context.Context, email string) (*accountsdomain.Account, error)
	GetAccountByID(ctx context.Context, id string) (*accountsdomain.Account, error)
	ListAccounts(ctx context.Context, filter accountsdomain.ListAccountsFilter) ([]*accountsdomain.Account, int, error)
	UpdateNickname(ctx context.Context, id, nickname string) (*accountsdomain.Account, error)
	VerifyEmailWithToken(ctx context.Context, rawToken string) (*accountsdomain.Account, error)
	CreateVerificationToken(ctx context.Context, email string, ttl time.Duration) (string, error)
	CreatePasswordResetToken(ctx context.Context, email string, ttl time.Duration) (string, error)
	ResetPasswordWithToken(ctx context.Context, rawToken, newPassword string) (*accountsdomain.Account, error)
	SetAccountStatus(ctx context.Context, id, status string, expectedVersion int) (*accountsdomain.Account, error)
	AddRole(ctx context.Context, id, role string) (*accountsdomain.Account, error)
	RevokeRole(ctx context.Context, id, role string, expectedVersion int) (*accountsdomain.Account, error)
	CountAccounts(ctx context.Context) int
	RecordMailTask(ctx context.Context, task accountsdomain.MailTask) error
	UpdateMailTaskStatus(ctx context.Context, recipient string, purpose accountsdomain.TokenPurpose, rawToken, status string) error
	GetRecentMailTasks(ctx context.Context, recipient string) []accountsdomain.MailTask
	Authenticate(ctx context.Context, email, password string) (*accountsdomain.Account, error)
}

// MemoryStore is an in-memory, thread-safe implementation of Store.
type MemoryStore struct {
	mu        sync.RWMutex
	accounts  map[string]*accountsdomain.Account     // id -> Account
	byEmail   map[string]string                      // email -> id
	tokens    map[string]*accountsdomain.StoredToken // tokenHash -> StoredToken
	mailTasks []accountsdomain.MailTask
}

// NewMemoryStore initializes a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		accounts:  make(map[string]*accountsdomain.Account),
		byEmail:   make(map[string]string),
		tokens:    make(map[string]*accountsdomain.StoredToken),
		mailTasks: make([]accountsdomain.MailTask, 0),
	}
}

func CloneAccount(acc *accountsdomain.Account) *accountsdomain.Account {
	if acc == nil {
		return nil
	}
	cp := *acc
	cp.Roles = append([]string(nil), acc.Roles...)
	return &cp
}

func NormalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func GenerateRandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func NewID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, GenerateRandomToken(8))
}

// CreateAccount registers a new account with a hashed password and returns a verification token.
func (s *MemoryStore) CreateAccount(ctx context.Context, email, password, nickname string) (*accountsdomain.Account, string, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	if normEmail == "" {
		return nil, "", errors.New("email cannot be empty")
	}
	if strings.TrimSpace(nickname) != "" {
		if _, err := accountsdomain.ValidateNickname(nickname); err != nil {
			return nil, "", err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[normEmail]; exists {
		return nil, "", accountsdomain.ErrDuplicateEmail
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	now := time.Now()
	acc := &accountsdomain.Account{
		ID:            NewID("usr"),
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

	s.accounts[acc.ID] = acc
	s.byEmail[normEmail] = acc.ID

	rawToken := GenerateRandomToken(16)
	tHash := HashToken(rawToken)
	s.tokens[tHash] = &accountsdomain.StoredToken{
		TokenHash: tHash,
		Purpose:   accountsdomain.PurposeEmailVerification,
		Target:    normEmail,
		ExpiresAt: now.Add(24 * time.Hour),
		Consumed:  false,
		CreatedAt: now,
	}

	s.mailTasks = append(s.mailTasks, accountsdomain.MailTask{
		ID:        NewID("mail"),
		Recipient: normEmail,
		Subject:   "Verify your email",
		Purpose:   accountsdomain.PurposeEmailVerification,
		Token:     rawToken,
		Status:    "pending",
		CreatedAt: now,
	})

	return CloneAccount(acc), rawToken, nil
}

// EnsureBootstrapAdmin idempotently initializes a verified administrator account.
func (s *MemoryStore) EnsureBootstrapAdmin(ctx context.Context, email, password string) (*accountsdomain.Account, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	if normEmail == "" || len(password) < 8 {
		return nil, errors.New("valid bootstrap admin email and password (min 8 chars) are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if id, exists := s.byEmail[normEmail]; exists {
		acc := s.accounts[id]
		changed := false
		if !acc.EmailVerified {
			acc.EmailVerified = true
			changed = true
		}
		if acc.Status != accountsdomain.StatusActive {
			acc.Status = accountsdomain.StatusActive
			changed = true
		}
		hasAdmin := false
		for _, r := range acc.Roles {
			if r == "admin" {
				hasAdmin = true
				break
			}
		}
		if !hasAdmin {
			acc.Roles = append(acc.Roles, "admin")
			changed = true
		}
		if changed {
			acc.Version++
			acc.UpdatedAt = time.Now()
		}
		return CloneAccount(acc), nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash bootstrap admin password: %w", err)
	}

	now := time.Now()
	acc := &accountsdomain.Account{
		ID:            NewID("usr"),
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
	s.accounts[acc.ID] = acc
	s.byEmail[normEmail] = acc.ID
	return CloneAccount(acc), nil
}

// Authenticate verifies email and password against stored hash.
func (s *MemoryStore) Authenticate(ctx context.Context, email, password string) (*accountsdomain.Account, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.byEmail[normEmail]
	if !exists {
		return nil, accountsdomain.ErrInvalidPassword
	}
	acc := s.accounts[id]
	if acc.Status == accountsdomain.StatusSuspended {
		return nil, accountsdomain.ErrAccountSuspended
	}
	if err := bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(password)); err != nil {
		return nil, accountsdomain.ErrInvalidPassword
	}
	return CloneAccount(acc), nil
}

// GetAccountByEmail looks up an account by normalized email.
func (s *MemoryStore) GetAccountByEmail(ctx context.Context, email string) (*accountsdomain.Account, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.byEmail[normEmail]
	if !exists {
		return nil, accountsdomain.ErrAccountNotFound
	}
	return CloneAccount(s.accounts[id]), nil
}

// GetAccountByID retrieves an account by ID.
func (s *MemoryStore) GetAccountByID(ctx context.Context, id string) (*accountsdomain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, accountsdomain.ErrAccountNotFound
	}
	return CloneAccount(acc), nil
}

// ListAccounts returns filtered accounts sorted by createdAt DESC with pagination.
func (s *MemoryStore) ListAccounts(ctx context.Context, filter accountsdomain.ListAccountsFilter) ([]*accountsdomain.Account, int, error) {
	page, pageSize := NormalizePagination(filter.Page, filter.PageSize)
	statusFilter := strings.TrimSpace(filter.Status)
	roleFilter := strings.TrimSpace(filter.Role)
	queryFilter := strings.ToLower(strings.TrimSpace(filter.Query))

	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]*accountsdomain.Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		if statusFilter != "" && acc.Status != statusFilter {
			continue
		}
		if roleFilter != "" && !HasRole(acc.Roles, roleFilter) {
			continue
		}
		if queryFilter != "" {
			if !strings.Contains(strings.ToLower(acc.Email), queryFilter) &&
				!strings.Contains(strings.ToLower(acc.Nickname), queryFilter) &&
				!strings.Contains(strings.ToLower(acc.ID), queryFilter) {
				continue
			}
		}
		matched = append(matched, CloneAccount(acc))
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID > matched[j].ID
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := len(matched)
	start := (page - 1) * pageSize
	if start >= total {
		return []*accountsdomain.Account{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

func HasRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

// UpdateNickname updates an account's display nickname and increments its version.
func (s *MemoryStore) UpdateNickname(ctx context.Context, id, nickname string) (*accountsdomain.Account, error) {
	trimmed, err := accountsdomain.ValidateNickname(nickname)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, accountsdomain.ErrAccountNotFound
	}
	if acc.Status == accountsdomain.StatusSuspended {
		return nil, accountsdomain.ErrAccountSuspended
	}
	acc.Nickname = trimmed
	acc.Version++
	acc.UpdatedAt = time.Now()
	return CloneAccount(acc), nil
}

// VerifyEmailWithToken validates the single-use token and sets email_verified = true.
func (s *MemoryStore) VerifyEmailWithToken(ctx context.Context, rawToken string) (*accountsdomain.Account, error) {
	tHash := HashToken(rawToken)
	s.mu.Lock()
	defer s.mu.Unlock()

	st, exists := s.tokens[tHash]
	if !exists {
		return nil, accountsdomain.ErrTokenNotFound
	}
	if st.Purpose != accountsdomain.PurposeEmailVerification {
		return nil, accountsdomain.ErrTokenPurposeMismatch
	}
	if time.Now().After(st.ExpiresAt) {
		return nil, accountsdomain.ErrTokenExpired
	}
	if st.Consumed {
		id := s.byEmail[st.Target]
		if acc, ok := s.accounts[id]; ok {
			return CloneAccount(acc), nil
		}
		return nil, accountsdomain.ErrTokenConsumed
	}

	st.Consumed = true
	id, exists := s.byEmail[st.Target]
	if !exists {
		return nil, accountsdomain.ErrAccountNotFound
	}

	acc := s.accounts[id]
	acc.EmailVerified = true
	acc.Version++
	acc.UpdatedAt = time.Now()

	return CloneAccount(acc), nil
}

// CreateVerificationToken creates a new token and records an email task.
func (s *MemoryStore) CreateVerificationToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[normEmail]; !exists {
		return "", accountsdomain.ErrAccountNotFound
	}

	now := time.Now()
	rawToken := GenerateRandomToken(16)
	tHash := HashToken(rawToken)
	s.tokens[tHash] = &accountsdomain.StoredToken{
		TokenHash: tHash,
		Purpose:   accountsdomain.PurposeEmailVerification,
		Target:    normEmail,
		ExpiresAt: now.Add(ttl),
		Consumed:  false,
		CreatedAt: now,
	}

	s.mailTasks = append(s.mailTasks, accountsdomain.MailTask{
		ID:        NewID("mail"),
		Recipient: normEmail,
		Subject:   "Verify your email",
		Purpose:   accountsdomain.PurposeEmailVerification,
		Token:     rawToken,
		Status:    "pending",
		CreatedAt: now,
	})

	return rawToken, nil
}

// CreatePasswordResetToken generates a single-use password reset token.
func (s *MemoryStore) CreatePasswordResetToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	normEmail := accountsdomain.NormalizeEmail(email)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[normEmail]; !exists {
		return "", nil
	}

	now := time.Now()
	rawToken := GenerateRandomToken(16)
	tHash := HashToken(rawToken)
	s.tokens[tHash] = &accountsdomain.StoredToken{
		TokenHash: tHash,
		Purpose:   accountsdomain.PurposePasswordReset,
		Target:    normEmail,
		ExpiresAt: now.Add(ttl),
		Consumed:  false,
		CreatedAt: now,
	}

	s.mailTasks = append(s.mailTasks, accountsdomain.MailTask{
		ID:        NewID("mail"),
		Recipient: normEmail,
		Subject:   "Reset your password",
		Purpose:   accountsdomain.PurposePasswordReset,
		Token:     rawToken,
		Status:    "pending",
		CreatedAt: now,
	})

	return rawToken, nil
}

// ResetPasswordWithToken updates the account's password hash and marks token consumed.
func (s *MemoryStore) ResetPasswordWithToken(ctx context.Context, rawToken, newPassword string) (*accountsdomain.Account, error) {
	tHash := HashToken(rawToken)
	s.mu.Lock()
	defer s.mu.Unlock()

	st, exists := s.tokens[tHash]
	if !exists {
		return nil, accountsdomain.ErrTokenNotFound
	}
	if st.Purpose != accountsdomain.PurposePasswordReset {
		return nil, accountsdomain.ErrTokenPurposeMismatch
	}
	if time.Now().After(st.ExpiresAt) {
		return nil, accountsdomain.ErrTokenExpired
	}
	if st.Consumed {
		return nil, accountsdomain.ErrTokenConsumed
	}

	id, exists := s.byEmail[st.Target]
	if !exists {
		return nil, accountsdomain.ErrAccountNotFound
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	st.Consumed = true
	acc := s.accounts[id]
	acc.PasswordHash = string(hash)
	acc.Version++
	acc.UpdatedAt = time.Now()

	return CloneAccount(acc), nil
}

// SetAccountStatus updates status with optimistic version check.
func (s *MemoryStore) SetAccountStatus(ctx context.Context, id, status string, expectedVersion int) (*accountsdomain.Account, error) {
	if err := accountsdomain.ValidateStatus(status); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, accountsdomain.ErrAccountNotFound
	}
	if acc.Version != expectedVersion {
		return nil, accountsdomain.ErrVersionConflict
	}

	acc.Status = status
	acc.Version++
	acc.UpdatedAt = time.Now()

	return CloneAccount(acc), nil
}

// AddRole appends a role to the account if not already present.
func (s *MemoryStore) AddRole(ctx context.Context, id, role string) (*accountsdomain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, accountsdomain.ErrAccountNotFound
	}

	if HasRole(acc.Roles, role) {
		return CloneAccount(acc), nil
	}

	acc.Roles = append(acc.Roles, role)
	acc.Version++
	acc.UpdatedAt = time.Now()

	return CloneAccount(acc), nil
}

// RevokeRole removes a role from the account with optimistic version check.
func (s *MemoryStore) RevokeRole(ctx context.Context, id, role string, expectedVersion int) (*accountsdomain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, accountsdomain.ErrAccountNotFound
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
	acc.UpdatedAt = time.Now()

	return CloneAccount(acc), nil
}

// CountAccounts returns the total number of registered accounts.
func (s *MemoryStore) CountAccounts(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.accounts)
}

// RecordMailTask appends an outbound mail task record.
func (s *MemoryStore) RecordMailTask(ctx context.Context, task accountsdomain.MailTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = NewID("mail")
	}
	task.Recipient = accountsdomain.NormalizeEmail(task.Recipient)
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	s.mailTasks = append(s.mailTasks, task)
	return nil
}

// UpdateMailTaskStatus updates the status of the latest matching mail task.
func (s *MemoryStore) UpdateMailTaskStatus(ctx context.Context, recipient string, purpose accountsdomain.TokenPurpose, rawToken, status string) error {
	norm := accountsdomain.NormalizeEmail(recipient)
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.mailTasks) - 1; i >= 0; i-- {
		t := &s.mailTasks[i]
		if t.Recipient == norm && t.Purpose == purpose && t.Token == rawToken {
			t.Status = status
			return nil
		}
	}
	return nil
}

// GetRecentMailTasks returns mail tasks sent to the given recipient.
func (s *MemoryStore) GetRecentMailTasks(ctx context.Context, recipient string) []accountsdomain.MailTask {
	norm := accountsdomain.NormalizeEmail(recipient)
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []accountsdomain.MailTask
	for _, task := range s.mailTasks {
		if recipient == "" || task.Recipient == norm {
			result = append(result, task)
		}
	}
	return result
}
