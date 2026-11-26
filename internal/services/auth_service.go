package services

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const (
	defaultMaxFailedLoginAttempts = 5
	defaultLoginBaseBackoff       = 2 * time.Second
	defaultLoginMaxBackoff        = 60 * time.Second
)

type loginAttemptState struct {
	failures    int
	lockedUntil time.Time
}

// AuthService manages local user bootstrapping, authentication, and in-memory vault unlocking.
type AuthService struct {
	users    ports.UserRepository
	hasher   ports.PasswordHasher
	encrypt  ports.KeyEncryptor
	mu       sync.RWMutex
	active   *domain.UserProfile
	vaultKey []byte

	backoffMu   sync.Mutex
	attempts    map[string]*loginAttemptState
	maxFailures int
	baseBackoff time.Duration
	maxBackoff  time.Duration
	nowFunc     func() time.Time
}

// NewAuthService constructs an AuthService with the required repository and cryptography ports.
func NewAuthService(users ports.UserRepository, hasher ports.PasswordHasher, encrypt ports.KeyEncryptor) *AuthService {
	return &AuthService{
		users:       users,
		hasher:      hasher,
		encrypt:     encrypt,
		attempts:    make(map[string]*loginAttemptState),
		maxFailures: defaultMaxFailedLoginAttempts,
		baseBackoff: defaultLoginBaseBackoff,
		maxBackoff:  defaultLoginMaxBackoff,
		nowFunc:     time.Now,
	}
}

// SetLoginBackoff customizes the consecutive-failure threshold and exponential cooldown window for Login.
func (s *AuthService) SetLoginBackoff(maxFailures int, baseDelay, maxDelay time.Duration) {
	s.backoffMu.Lock()
	defer s.backoffMu.Unlock()
	if maxFailures > 0 {
		s.maxFailures = maxFailures
	}
	if baseDelay > 0 {
		s.baseBackoff = baseDelay
	}
	if maxDelay >= baseDelay && maxDelay > 0 {
		s.maxBackoff = maxDelay
	}
}

// HasUsers reports whether at least one local user has been configured in the database.
func (s *AuthService) HasUsers(ctx context.Context) (bool, error) {
	total, err := s.users.Count(ctx)
	if err != nil {
		return false, fmt.Errorf("check existing users: %w", err)
	}
	return total > 0, nil
}

// BootstrapInitialUser creates the first default local user and unlocks its vault on initial launch.
func (s *AuthService) BootstrapInitialUser(ctx context.Context, username, password string) (domain.UserProfile, error) {
	total, err := s.users.Count(ctx)
	if err != nil {
		return domain.UserProfile{}, fmt.Errorf("count users before bootstrap: %w", err)
	}
	if total > 0 {
		return domain.UserProfile{}, fmt.Errorf("%w", domain.ErrInitialUserAlreadySet)
	}

	user, err := s.createUserRecord(ctx, username, password, true)
	if err != nil {
		return domain.UserProfile{}, err
	}

	masterKey, err := s.encrypt.DeriveMasterKey(password, user.VaultSalt)
	if err != nil {
		return domain.UserProfile{}, fmt.Errorf("derive initial user master key: %w", err)
	}

	profile := user.Profile()
	s.setSession(profile, masterKey)
	return profile, nil
}

// CreateUser registers an additional isolated local user without modifying the current session.
func (s *AuthService) CreateUser(ctx context.Context, username, password string) (domain.UserProfile, error) {
	user, err := s.createUserRecord(ctx, username, password, false)
	if err != nil {
		return domain.UserProfile{}, err
	}
	return user.Profile(), nil
}

// ListUsers returns safe public profiles for all local users.
func (s *AuthService) ListUsers(ctx context.Context) ([]domain.UserProfile, error) {
	users, err := s.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	profiles := make([]domain.UserProfile, 0, len(users))
	for _, u := range users {
		profiles = append(profiles, u.Profile())
	}
	return profiles, nil
}

// Login authenticates a local user, derives their vault master key, and sets the active session.
func (s *AuthService) Login(ctx context.Context, username, password string) (domain.UserProfile, error) {
	trimmedUser := strings.TrimSpace(username)
	if trimmedUser == "" || password == "" {
		return domain.UserProfile{}, fmt.Errorf("%w: username and password are required", domain.ErrInvalidCredentials)
	}

	userKey := strings.ToLower(trimmedUser)
	if remaining := s.checkLoginBackoff(userKey); remaining > 0 {
		waitSecs := int(math.Ceil(remaining.Seconds()))
		if waitSecs < 1 {
			waitSecs = 1
		}
		return domain.UserProfile{}, fmt.Errorf("%w: try again in %ds", domain.ErrTooManyAttempts, waitSecs)
	}

	user, err := s.users.GetByUsername(ctx, trimmedUser)
	if err != nil {
		s.recordLoginFailure(userKey)
		return domain.UserProfile{}, fmt.Errorf("%w", domain.ErrInvalidCredentials)
	}

	if !s.hasher.VerifyPassword(password, user.AuthSalt, user.PasswordHash) {
		s.recordLoginFailure(userKey)
		return domain.UserProfile{}, fmt.Errorf("%w", domain.ErrInvalidCredentials)
	}

	masterKey, err := s.encrypt.DeriveMasterKey(password, user.VaultSalt)
	if err != nil {
		return domain.UserProfile{}, fmt.Errorf("derive vault master key: %w", err)
	}

	s.clearLoginFailures(userKey)
	profile := user.Profile()
	s.setSession(profile, masterKey)
	return profile, nil
}

// Lock clears the active user session and wipes the derived vault master key from memory.
func (s *AuthService) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	wipeBytes(s.vaultKey)
	s.vaultKey = nil
	s.active = nil
}

// CurrentUser returns the currently authenticated user profile or domain.ErrVaultLocked if locked.
func (s *AuthService) CurrentUser() (domain.UserProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return domain.UserProfile{}, fmt.Errorf("%w", domain.ErrVaultLocked)
	}
	return *s.active, nil
}

// ActiveVaultCredentials returns the active user ID and a copy of the unlocked vault master key.
// Callers must wipe the returned key slice when finished.
func (s *AuthService) ActiveVaultCredentials() (string, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil || len(s.vaultKey) == 0 {
		return "", nil, fmt.Errorf("%w", domain.ErrVaultLocked)
	}
	keyCopy := make([]byte, len(s.vaultKey))
	copy(keyCopy, s.vaultKey)
	return s.active.ID, keyCopy, nil
}

func (s *AuthService) createUserRecord(ctx context.Context, username, password string, isDefault bool) (domain.User, error) {
	if err := domain.ValidateUsername(username); err != nil {
		return domain.User{}, fmt.Errorf("validate username: %w", err)
	}
	if err := domain.ValidatePassword(password); err != nil {
		return domain.User{}, fmt.Errorf("validate password: %w", err)
	}

	authSalt, err := s.hasher.GenerateSalt()
	if err != nil {
		return domain.User{}, fmt.Errorf("generate auth salt: %w", err)
	}
	vaultSalt, err := s.hasher.GenerateSalt()
	if err != nil {
		return domain.User{}, fmt.Errorf("generate vault salt: %w", err)
	}

	passwordHash, err := s.hasher.HashPassword(password, authSalt)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	user := domain.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: passwordHash,
		AuthSalt:     authSalt,
		VaultSalt:    vaultSalt,
		IsDefault:    isDefault,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("persist user: %w", err)
	}
	return user, nil
}

func (s *AuthService) setSession(profile domain.UserProfile, masterKey []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	wipeBytes(s.vaultKey)
	s.vaultKey = masterKey
	profileCopy := profile
	s.active = &profileCopy
}

func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func (s *AuthService) checkLoginBackoff(userKey string) time.Duration {
	s.backoffMu.Lock()
	defer s.backoffMu.Unlock()

	st, ok := s.attempts[userKey]
	if !ok || st.lockedUntil.IsZero() {
		return 0
	}
	now := s.nowFunc()
	if now.Before(st.lockedUntil) {
		return st.lockedUntil.Sub(now)
	}
	return 0
}

func (s *AuthService) recordLoginFailure(userKey string) {
	s.backoffMu.Lock()
	defer s.backoffMu.Unlock()

	st, ok := s.attempts[userKey]
	if !ok {
		st = &loginAttemptState{}
		s.attempts[userKey] = st
	}
	st.failures++

	if st.failures >= s.maxFailures {
		shift := st.failures - s.maxFailures
		if shift > 10 {
			shift = 10
		}
		delay := s.baseBackoff << shift
		if delay > s.maxBackoff || delay <= 0 {
			delay = s.maxBackoff
		}
		st.lockedUntil = s.nowFunc().Add(delay)
	}
}

func (s *AuthService) clearLoginFailures(userKey string) {
	s.backoffMu.Lock()
	defer s.backoffMu.Unlock()
	delete(s.attempts, userKey)
}
