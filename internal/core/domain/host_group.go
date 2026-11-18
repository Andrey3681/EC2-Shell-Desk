package domain

import (
	"fmt"
	"strings"
	"time"
)

// MaxGroupNameLength defines the maximum allowed character length for a HostGroup name.
const MaxGroupNameLength = 64

// HostGroup represents a user-defined folder or category (e.g., University, Work, Personal) for organizing HostInstance records.
type HostGroup struct {
	ID          string    `json:"id"`
	OwnerUserID string    `json:"ownerUserId"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ValidateGroupName ensures a host group name is non-empty and within display length limits.
func ValidateGroupName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len(trimmed) > MaxGroupNameLength {
		return fmt.Errorf("%w: group name must be between 1 and %d characters", ErrInvalidInput, MaxGroupNameLength)
	}
	return nil
}
