package domain

import (
	"fmt"
	"strings"
	"time"
)

// Environment classifies the operational stage of a HostInstance.
type Environment string

const (
	// EnvProd marks a production virtual machine or EC2 instance.
	EnvProd Environment = "Prod"
	// EnvStaging marks a staging or pre-production instance.
	EnvStaging Environment = "Staging"
	// EnvDev marks a development or sandbox instance.
	EnvDev Environment = "Dev"
)

// DefaultSSHPort is the standard TCP port for SSH connections.
const DefaultSSHPort = 22

// HostInstance represents a remote virtual machine or EC2 instance owned by a local user.
type HostInstance struct {
	ID              string      `json:"id"`
	OwnerUserID     string      `json:"ownerUserId"`
	GroupID         string      `json:"groupId"`
	Alias           string      `json:"alias"`
	HostAddress     string      `json:"hostAddress"`
	Port            int         `json:"port"`
	SSHUsername     string      `json:"sshUsername"`
	SSHKeyID        string      `json:"sshKeyId"`
	Environment     Environment `json:"environment"`
	Notes           string      `json:"notes"`
	LastConnectedAt *time.Time  `json:"lastConnectedAt,omitempty"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
}

// Validate checks that required HostInstance connection parameters are well-formed.
func (h HostInstance) Validate() error {
	if strings.TrimSpace(h.OwnerUserID) == "" {
		return fmt.Errorf("%w: owner user id is required", ErrInvalidInput)
	}
	if strings.TrimSpace(h.Alias) == "" || len(strings.TrimSpace(h.Alias)) > 128 {
		return fmt.Errorf("%w: alias must be between 1 and 128 characters", ErrInvalidInput)
	}
	addr := strings.TrimSpace(h.HostAddress)
	if addr == "" || strings.ContainsAny(addr, " \t\r\n") {
		return fmt.Errorf("%w: host address must be a non-empty IP or hostname without whitespace", ErrInvalidInput)
	}
	if h.Port < 1 || h.Port > 65535 {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidInput)
	}
	sshUser := strings.TrimSpace(h.SSHUsername)
	if sshUser == "" || strings.ContainsAny(sshUser, " \t\r\n") {
		return fmt.Errorf("%w: ssh username must be non-empty without whitespace", ErrInvalidInput)
	}
	switch h.Environment {
	case "", EnvProd, EnvStaging, EnvDev:
	default:
		return fmt.Errorf("%w: unsupported environment %q", ErrInvalidInput, h.Environment)
	}
	return nil
}

// KnownHost represents a TOFU-verified SSH host public key pinned for a specific user.
type KnownHost struct {
	ID            string    `json:"id"`
	OwnerUserID   string    `json:"ownerUserId"`
	HostAddress   string    `json:"hostAddress"`
	Port          int       `json:"port"`
	KeyType       string    `json:"keyType"`
	Fingerprint   string    `json:"fingerprint"`
	PublicKeyBlob []byte    `json:"-"`
	TrustedAt     time.Time `json:"trustedAt"`
}
