// Package accounts manages account records, credentials, and verification tokens.
package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrAccountSuspended     = errors.New("account is suspended")
	ErrDuplicateEmail       = errors.New("email already registered")
	ErrInvalidPassword      = errors.New("invalid password")
	ErrTokenNotFound        = errors.New("token not found")
	ErrTokenExpired         = errors.New("token expired")
	ErrTokenConsumed        = errors.New("token already consumed")
	ErrTokenPurposeMismatch = errors.New("token purpose mismatch")
	ErrVersionConflict      = errors.New("version conflict")
)

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
	PasswordHash  string    `json:"-"`
	EmailVerified bool      `json:"emailVerified"`
	Status        string    `json:"status"` // "active", "suspended"
	Roles         []string  `json:"roles"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
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

// Store defines persistence operations for accounts and tokens.
type Store interface {
	CreateAccount(ctx context.Context, email, password string) (*Account, string, error)
	GetAccountByEmail(ctx context.Context, email string) (*Account, error)
	GetAccountByID(ctx context.Context, id string) (*Account, error)
	VerifyEmailWithToken(ctx context.Context, rawToken string) (*Account, error)
	CreateVerificationToken(ctx context.Context, email string, ttl time.Duration) (string, error)
	CreatePasswordResetToken(ctx context.Context, email string, ttl time.Duration) (string, error)
	ResetPasswordWithToken(ctx context.Context, rawToken, newPassword string) (*Account, error)
	SetAccountStatus(ctx context.Context, id, status string, expectedVersion int) (*Account, error)
	AddRole(ctx context.Context, id, role string) (*Account, error)
	RevokeRole(ctx context.Context, id, role string, expectedVersion int) (*Account, error)
	CountAccounts(ctx context.Context) int
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
func (s *MemoryStore) CreateAccount(ctx context.Context, email, password string) (*Account, string, error) {
	normEmail := NormalizeEmail(email)
	if normEmail == "" {
		return nil, "", errors.New("email cannot be empty")
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
		PasswordHash:  string(hash),
		EmailVerified: false,
		Status:        "active",
		Roles:         []string{"student"},
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.accounts[acc.ID] = acc
	s.byEmail[normEmail] = acc.ID

	// Create verification token
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
		Status:    "sent",
		CreatedAt: now,
	})

	accCopy := *acc
	return &accCopy, rawToken, nil
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
	if acc.Status == "suspended" {
		return nil, ErrAccountSuspended
	}
	if err := bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}
	accCopy := *acc
	return &accCopy, nil
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
	accCopy := *s.accounts[id]
	return &accCopy, nil
}

// GetAccountByID retrieves an account by ID.
func (s *MemoryStore) GetAccountByID(ctx context.Context, id string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}
	accCopy := *acc
	return &accCopy, nil
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
		// Idempotent return: already consumed
		id := s.byEmail[st.Target]
		if acc, ok := s.accounts[id]; ok {
			accCopy := *acc
			return &accCopy, nil
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

	accCopy := *acc
	return &accCopy, nil
}

// CreateVerificationToken creates a new token and records an email task.
func (s *MemoryStore) CreateVerificationToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	normEmail := NormalizeEmail(email)
	s.mu.Lock()
	defer s.mu.Unlock()

	id, exists := s.byEmail[normEmail]
	if !exists {
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
		Status:    "sent",
		CreatedAt: now,
	})

	_ = id
	return rawToken, nil
}

// CreatePasswordResetToken generates a single-use password reset token.
func (s *MemoryStore) CreatePasswordResetToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	normEmail := NormalizeEmail(email)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[normEmail]; !exists {
		// Return empty token without error to avoid email enumeration.
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
		Status:    "sent",
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

	accCopy := *acc
	return &accCopy, nil
}

// SetAccountStatus updates status with optimistic version check.
func (s *MemoryStore) SetAccountStatus(ctx context.Context, id, status string, expectedVersion int) (*Account, error) {
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

	accCopy := *acc
	return &accCopy, nil
}

// AddRole appends a role to the account if not already present.
func (s *MemoryStore) AddRole(ctx context.Context, id, role string) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, exists := s.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}

	for _, r := range acc.Roles {
		if r == role {
			accCopy := *acc
			return &accCopy, nil
		}
	}

	acc.Roles = append(acc.Roles, role)
	acc.Version++
	acc.UpdatedAt = time.Now()

	accCopy := *acc
	return &accCopy, nil
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

	accCopy := *acc
	return &accCopy, nil
}

// CountAccounts returns the total number of registered accounts.
func (s *MemoryStore) CountAccounts(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.accounts)
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
