// Package accounts manages account records, credentials, and verification tokens.
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
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// StatusActive indicates an operational account.
	StatusActive = "active"
	// StatusSuspended indicates an administratively suspended account.
	StatusSuspended = "suspended"
)

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrAccountSuspended     = errors.New("account is suspended")
	ErrDuplicateEmail       = errors.New("email already registered")
	ErrInvalidPassword      = errors.New("invalid password")
	ErrInvalidStatus        = errors.New("invalid account status")
	ErrInvalidNickname      = errors.New("invalid nickname")
	ErrTokenNotFound        = errors.New("token not found")
	ErrTokenExpired         = errors.New("token expired")
	ErrTokenConsumed        = errors.New("token already consumed")
	ErrTokenPurposeMismatch = errors.New("token purpose mismatch")
	ErrVersionConflict      = errors.New("version conflict")
)

// TokenPurpose scopes a one-time token to a single workflow.
type TokenPurpose string

const (
	PurposeEmailVerification TokenPurpose = "email_verification"
	PurposePasswordReset     TokenPurpose = "password_reset"
	PurposeInstructorInvite  TokenPurpose = "instructor_invitation"
)

// Account represents a user identity record.
type Account struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Nickname      string    `json:"nickname"`
	PasswordHash  string    `json:"-"`
	EmailVerified bool      `json:"emailVerified"`
	Status        string    `json:"status"` // "active", "suspended"
	Roles         []string  `json:"roles"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ListAccountsFilter defines optional filters and pagination for admin account listing.
type ListAccountsFilter struct {
	Status   string
	Role     string
	Query    string
	Page     int
	PageSize int
}

// StoredToken tracks one-time tokens for email verification or password reset.
type StoredToken struct {
	TokenHash string
	Purpose   TokenPurpose
	Target    string // email or userId
	ExpiresAt time.Time
	Consumed  bool
	CreatedAt time.Time
}

// MailTask records an outbound email notification.
type MailTask struct {
	ID        string
	Recipient string
	Subject   string
	Purpose   TokenPurpose
	Token     string
	Status    string
	CreatedAt time.Time
}

// Store defines persistence operations for accounts, one-time tokens, and mail tasks.
type Store interface {
	CreateAccount(ctx context.Context, email, password, nickname string) (*Account, string, error)
	EnsureBootstrapAdmin(ctx context.Context, email, password string) (*Account, error)
	GetAccountByEmail(ctx context.Context, email string) (*Account, error)
	GetAccountByID(ctx context.Context, id string) (*Account, error)
	ListAccounts(ctx context.Context, filter ListAccountsFilter) ([]*Account, int, error)
	UpdateNickname(ctx context.Context, id, nickname string) (*Account, error)
	VerifyEmailWithToken(ctx context.Context, rawToken string) (*Account, error)
	CreateVerificationToken(ctx context.Context, email string, ttl time.Duration) (string, error)
	CreatePasswordResetToken(ctx context.Context, email string, ttl time.Duration) (string, error)
	ResetPasswordWithToken(ctx context.Context, rawToken, newPassword string) (*Account, error)
	SetAccountStatus(ctx context.Context, id, status string, expectedVersion int) (*Account, error)
	AddRole(ctx context.Context, id, role string) (*Account, error)
	RevokeRole(ctx context.Context, id, role string, expectedVersion int) (*Account, error)
	CountAccounts(ctx context.Context) int
	RecordMailTask(ctx context.Context, task MailTask) error
	UpdateMailTaskStatus(ctx context.Context, recipient string, purpose TokenPurpose, rawToken, status string) error
	GetRecentMailTasks(ctx context.Context, recipient string) []MailTask
	Authenticate(ctx context.Context, email, password string) (*Account, error)
}

// MemoryStore is an in-memory, thread-safe implementation of Store.
type MemoryStore struct {
	mu        sync.RWMutex
	accounts  map[string]*Account     // id -> Account
	byEmail   map[string]string       // email -> id
	tokens    map[string]*StoredToken // tokenHash -> StoredToken
	mailTasks []MailTask
}

// NewMemoryStore initializes a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		accounts:  make(map[string]*Account),
		byEmail:   make(map[string]string),
		tokens:    make(map[string]*StoredToken),
		mailTasks: make([]MailTask, 0),
	}
}

// NormalizeEmail converts email to lowercase and trims whitespace.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// DefaultNickname returns the trimmed nickname or falls back to the email prefix before "@".
func DefaultNickname(email, nickname string) string {
	trimmed := strings.TrimSpace(nickname)
	if trimmed != "" {
		if utf8.RuneCountInString(trimmed) > 64 {
			return string([]rune(trimmed)[:64])
		}
		return trimmed
	}
	norm := NormalizeEmail(email)
	if prefix, _, ok := strings.Cut(norm, "@"); ok && prefix != "" {
		if utf8.RuneCountInString(prefix) > 64 {
			return string([]rune(prefix)[:64])
		}
		return prefix
	}
	return "user"
}

// ValidateNickname checks that a user-provided nickname is non-empty and within length bounds.
func ValidateNickname(nickname string) (string, error) {
	trimmed := strings.TrimSpace(nickname)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 64 {
		return "", ErrInvalidNickname
	}
	return trimmed, nil
}

// ValidateStatus checks that an account status is an allowed value.
func ValidateStatus(status string) error {
	if status != StatusActive && status != StatusSuspended {
		return ErrInvalidStatus
	}
	return nil
}

func cloneAccount(acc *Account) *Account {
	if acc == nil {
		return nil
	}
	cp := *acc
	cp.Roles = append([]string(nil), acc.Roles...)
	return &cp
}

func normalizePagination(page, pageSize int) (int, int) {
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

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func generateRandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, generateRandomToken(8))
}

// CreateAccount registers a new account with a hashed password and returns a verification token.
func (s *MemoryStore) CreateAccount(ctx context.Context, email, password, nickname string) (*Account, string, error) {
	normEmail := NormalizeEmail(email)
	if normEmail == "" {
		return nil, "", errors.New("email cannot be empty")
	}
	if strings.TrimSpace(nickname) != "" {
		if _, err := ValidateNickname(nickname); err != nil {
			return nil, "", err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[normEmail]; exists {
		return nil, "", ErrDuplicateEmail
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	now := time.Now()
	acc := &Account{
		ID:            generateID("usr"),
		Email:         normEmail,
		Nickname:      DefaultNickname(normEmail, nickname),
		PasswordHash:  string(hash),
		EmailVerified: false,
		Status:        StatusActive,
		Roles:         []string{"student"},
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.accounts[acc.ID] = acc
	s.byEmail[normEmail] = acc.ID

	rawToken := generateRandomToken(16)
	tHash := hashToken(rawToken)
	s.tokens[tHash] = &StoredToken{
		TokenHash: tHash,
		Purpose:   PurposeEmailVerification,
		Target:    normEmail,
		ExpiresAt: now.Add(24 * time.Hour),
		Consumed:  false,
		CreatedAt: now,
	}

	s.mailTasks = append(s.mailTasks, MailTask{
		ID:        generateID("mail"),
		Recipient: normEmail,
		Subject:   "Verify your email",
		Purpose:   PurposeEmailVerification,
		Token:     rawToken,
		Status:    "pending",
		CreatedAt: now,
	})

	return cloneAccount(acc), rawToken, nil
}

// EnsureBootstrapAdmin idempotently initializes a verified administrator account.
func (s *MemoryStore) EnsureBootstrapAdmin(ctx context.Context, email, password string) (*Account, error) {
	normEmail := NormalizeEmail(email)
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
		if acc.Status != StatusActive {
			acc.Status = StatusActive
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
		return cloneAccount(acc), nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash bootstrap admin password: %w", err)
	}

	now := time.Now()
	acc := &Account{
		ID:            generateID("usr"),
		Email:         normEmail,
		Nickname:      DefaultNickname(normEmail, ""),
		PasswordHash:  string(hash),
		EmailVerified: true,
		Status:        StatusActive,
		Roles:         []string{"student", "admin"},
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.accounts[acc.ID] = acc
	s.byEmail[normEmail] = acc.ID
	return cloneAccount(acc), nil
}

// Authenticate verifies email and password against stored hash.
func (s *MemoryStore) Authenticate(ctx context.Context, email, password string) (*Account, error) {
	normEmail := NormalizeEmail(email)
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.byEmail[normEmail]
	if !exists {
		return nil, ErrInvalidPassword
	}
	acc := s.accounts[id]
	if acc.Status == StatusSuspended {
		return nil, ErrAccountSuspended
	}
	if err := bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}
	return cloneAccount(acc), nil
}

// GetAccountByEmail looks up an account by normalized email.
func (s *MemoryStore) GetAccountByEmail(ctx context.Context, email string) (*Account, error) {
	normEmail := NormalizeEmail(email)
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.byEmail[normEmail]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return cloneAccount(s.accounts[id]), nil
}

// GetAccountByID retrieves an account by ID.
func (s *MemoryStore) GetAccountByID(ctx context.Context, id string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return cloneAccount(acc), nil
}

// ListAccounts returns filtered accounts sorted by createdAt DESC with pagination.
func (s *MemoryStore) ListAccounts(ctx context.Context, filter ListAccountsFilter) ([]*Account, int, error) {
	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	statusFilter := strings.TrimSpace(filter.Status)
	roleFilter := strings.TrimSpace(filter.Role)
	queryFilter := strings.ToLower(strings.TrimSpace(filter.Query))

	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]*Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		if statusFilter != "" && acc.Status != statusFilter {
			continue
		}
		if roleFilter != "" && !hasRole(acc.Roles, roleFilter) {
			continue
		}
		if queryFilter != "" {
			if !strings.Contains(strings.ToLower(acc.Email), queryFilter) &&
				!strings.Contains(strings.ToLower(acc.Nickname), queryFilter) &&
				!strings.Contains(strings.ToLower(acc.ID), queryFilter) {
				continue
			}
		}
		matched = append(matched, cloneAccount(acc))
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
		return []*Account{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

func hasRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

// UpdateNickname updates an account's display nickname and increments its version.
func (s *MemoryStore) UpdateNickname(ctx context.Context, id, nickname string) (*Account, error) {
	trimmed, err := ValidateNickname(nickname)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}
	if acc.Status == StatusSuspended {
		return nil, ErrAccountSuspended
	}
	acc.Nickname = trimmed
	acc.Version++
	acc.UpdatedAt = time.Now()
	return cloneAccount(acc), nil
}

// VerifyEmailWithToken validates the single-use token and sets email_verified = true.
func (s *MemoryStore) VerifyEmailWithToken(ctx context.Context, rawToken string) (*Account, error) {
	tHash := hashToken(rawToken)
	s.mu.Lock()
	defer s.mu.Unlock()

	st, exists := s.tokens[tHash]
	if !exists {
		return nil, ErrTokenNotFound
	}
	if st.Purpose != PurposeEmailVerification {
		return nil, ErrTokenPurposeMismatch
	}
	if time.Now().After(st.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if st.Consumed {
		id := s.byEmail[st.Target]
		if acc, ok := s.accounts[id]; ok {
			return cloneAccount(acc), nil
		}
		return nil, ErrTokenConsumed
	}

	st.Consumed = true
	id, exists := s.byEmail[st.Target]
	if !exists {
		return nil, ErrAccountNotFound
	}

	acc := s.accounts[id]
	acc.EmailVerified = true
	acc.Version++
	acc.UpdatedAt = time.Now()

	return cloneAccount(acc), nil
}

// CreateVerificationToken creates a new token and records an email task.
func (s *MemoryStore) CreateVerificationToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	normEmail := NormalizeEmail(email)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[normEmail]; !exists {
		return "", ErrAccountNotFound
	}

	now := time.Now()
	rawToken := generateRandomToken(16)
	tHash := hashToken(rawToken)
	s.tokens[tHash] = &StoredToken{
		TokenHash: tHash,
		Purpose:   PurposeEmailVerification,
		Target:    normEmail,
		ExpiresAt: now.Add(ttl),
		Consumed:  false,
		CreatedAt: now,
	}

	s.mailTasks = append(s.mailTasks, MailTask{
		ID:        generateID("mail"),
		Recipient: normEmail,
		Subject:   "Verify your email",
		Purpose:   PurposeEmailVerification,
		Token:     rawToken,
		Status:    "pending",
		CreatedAt: now,
	})

	return rawToken, nil
}

// CreatePasswordResetToken generates a single-use password reset token.
func (s *MemoryStore) CreatePasswordResetToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	normEmail := NormalizeEmail(email)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[normEmail]; !exists {
		return "", nil
	}

	now := time.Now()
	rawToken := generateRandomToken(16)
	tHash := hashToken(rawToken)
	s.tokens[tHash] = &StoredToken{
		TokenHash: tHash,
		Purpose:   PurposePasswordReset,
		Target:    normEmail,
		ExpiresAt: now.Add(ttl),
		Consumed:  false,
		CreatedAt: now,
	}

	s.mailTasks = append(s.mailTasks, MailTask{
		ID:        generateID("mail"),
		Recipient: normEmail,
		Subject:   "Reset your password",
		Purpose:   PurposePasswordReset,
		Token:     rawToken,
		Status:    "pending",
		CreatedAt: now,
	})

	return rawToken, nil
}

// ResetPasswordWithToken updates the account's password hash and marks token consumed.
func (s *MemoryStore) ResetPasswordWithToken(ctx context.Context, rawToken, newPassword string) (*Account, error) {
	tHash := hashToken(rawToken)
	s.mu.Lock()
	defer s.mu.Unlock()

	st, exists := s.tokens[tHash]
	if !exists {
		return nil, ErrTokenNotFound
	}
	if st.Purpose != PurposePasswordReset {
		return nil, ErrTokenPurposeMismatch
	}
	if time.Now().After(st.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if st.Consumed {
		return nil, ErrTokenConsumed
	}

	id, exists := s.byEmail[st.Target]
	if !exists {
		return nil, ErrAccountNotFound
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

	return cloneAccount(acc), nil
}

// SetAccountStatus updates status with optimistic version check.
func (s *MemoryStore) SetAccountStatus(ctx context.Context, id, status string, expectedVersion int) (*Account, error) {
	if err := ValidateStatus(status); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}
	if acc.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	acc.Status = status
	acc.Version++
	acc.UpdatedAt = time.Now()

	return cloneAccount(acc), nil
}

// AddRole appends a role to the account if not already present.
func (s *MemoryStore) AddRole(ctx context.Context, id, role string) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}

	if hasRole(acc.Roles, role) {
		return cloneAccount(acc), nil
	}

	acc.Roles = append(acc.Roles, role)
	acc.Version++
	acc.UpdatedAt = time.Now()

	return cloneAccount(acc), nil
}

// RevokeRole removes a role from the account with optimistic version check.
func (s *MemoryStore) RevokeRole(ctx context.Context, id, role string, expectedVersion int) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}
	if acc.Version != expectedVersion {
		return nil, ErrVersionConflict
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

	return cloneAccount(acc), nil
}

// CountAccounts returns the total number of registered accounts.
func (s *MemoryStore) CountAccounts(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.accounts)
}

// RecordMailTask appends an outbound mail task record.
func (s *MemoryStore) RecordMailTask(ctx context.Context, task MailTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = generateID("mail")
	}
	task.Recipient = NormalizeEmail(task.Recipient)
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
func (s *MemoryStore) UpdateMailTaskStatus(ctx context.Context, recipient string, purpose TokenPurpose, rawToken, status string) error {
	norm := NormalizeEmail(recipient)
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
func (s *MemoryStore) GetRecentMailTasks(ctx context.Context, recipient string) []MailTask {
	norm := NormalizeEmail(recipient)
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MailTask
	for _, task := range s.mailTasks {
		if recipient == "" || task.Recipient == norm {
			result = append(result, task)
		}
	}
	return result
}
