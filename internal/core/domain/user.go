package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	// MinUsernameLength defines the minimum allowed character length for a local username.
	MinUsernameLength = 3
	// MaxUsernameLength defines the maximum allowed character length for a local username.
	MaxUsernameLength = 64
	// MinPasswordLength defines the minimum allowed character length for a local user password.
	MinPasswordLength = 8
)

// User represents a local application user with an isolated SSH key vault and host inventory.
type User struct {
	ID           string
	Username     string
	PasswordHash []byte
	AuthSalt     []byte
	VaultSalt    []byte
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserProfile represents a safe, non-sensitive projection of a User for UI and service responses.
type UserProfile struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	IsDefault bool      `json:"isDefault"`
	CreatedAt time.Time `json:"createdAt"`
}

// Profile returns the public UserProfile projection without password hashes or cryptographic salts.
func (u User) Profile() UserProfile {
	return UserProfile{
		ID:        u.ID,
		Username:  u.Username,
		IsDefault: u.IsDefault,
		CreatedAt: u.CreatedAt,
	}
}

// ValidateUsername checks whether a username satisfies length and character constraints.
func ValidateUsername(username string) error {
	trimmed := strings.TrimSpace(username)
	if len(trimmed) < MinUsernameLength || len(trimmed) > MaxUsernameLength {
		return fmt.Errorf("%w: username must be between %d and %d characters", ErrInvalidInput, MinUsernameLength, MaxUsernameLength)
	}
	if trimmed != username {
		return fmt.Errorf("%w: username must not contain leading or trailing whitespace", ErrInvalidInput)
	}
	return nil
}

// ValidatePassword checks whether a raw password satisfies minimum strength requirements.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, MinPasswordLength)
	}
	return nil
}
