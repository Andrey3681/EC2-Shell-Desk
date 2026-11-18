package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	// MaxSnippetNameLength defines the maximum allowed character length for a CommandSnippet name.
	MaxSnippetNameLength = 128
	// MaxSnippetCommandLength defines the maximum allowed byte length for a CommandSnippet body.
	MaxSnippetCommandLength = 16 * 1024
)

// CommandSnippet represents a reusable shell command or deployment script owned by a local user.
type CommandSnippet struct {
	ID          string    `json:"id"`
	OwnerUserID string    `json:"ownerUserId"`
	HostID      string    `json:"hostId"`
	GroupID     string    `json:"groupId"`
	Name        string    `json:"name"`
	Command     string    `json:"command"`
	AutoExecute bool      `json:"autoExecute"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Validate checks that required CommandSnippet fields and size bounds are valid.
func (s CommandSnippet) Validate() error {
	if strings.TrimSpace(s.OwnerUserID) == "" {
		return fmt.Errorf("%w: owner user id is required", ErrInvalidInput)
	}
	name := strings.TrimSpace(s.Name)
	if name == "" || len(name) > MaxSnippetNameLength {
		return fmt.Errorf("%w: snippet name must be between 1 and %d characters", ErrInvalidInput, MaxSnippetNameLength)
	}
	cmd := strings.TrimSpace(s.Command)
	if cmd == "" || len(s.Command) > MaxSnippetCommandLength {
		return fmt.Errorf("%w: snippet command must be between 1 and %d bytes", ErrInvalidInput, MaxSnippetCommandLength)
	}
	return nil
}
