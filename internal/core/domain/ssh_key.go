package domain

import (
	"fmt"
	"strings"
	"time"
)

// KeyType identifies the cryptographic algorithm family of an imported SSH private key.
type KeyType string

const (
	// KeyTypeRSA identifies an RSA SSH private key.
	KeyTypeRSA KeyType = "rsa"
	// KeyTypeEd25519 identifies an Ed25519 SSH private key.
	KeyTypeEd25519 KeyType = "ed25519"
	// KeyTypeECDSA identifies an ECDSA SSH private key.
	KeyTypeECDSA KeyType = "ecdsa"
)

// SSHKey represents an encrypted SSH private key owned by a single local user.
type SSHKey struct {
	ID                  string
	OwnerUserID         string
	Name                string
	KeyType             KeyType
	Fingerprint         string
	EncryptedPEM        []byte
	EncryptedPassphrase []byte
	HasPassphrase       bool
	CreatedAt           time.Time
}

// SSHKeySummary represents a safe metadata view of an SSHKey without encrypted key material.
type SSHKeySummary struct {
	ID            string    `json:"id"`
	OwnerUserID   string    `json:"ownerUserId"`
	Name          string    `json:"name"`
	KeyType       KeyType   `json:"keyType"`
	Fingerprint   string    `json:"fingerprint"`
	HasPassphrase bool      `json:"hasPassphrase"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Summary returns a safe SSHKeySummary stripping encrypted key and passphrase bytes.
func (k SSHKey) Summary() SSHKeySummary {
	return SSHKeySummary{
		ID:            k.ID,
		OwnerUserID:   k.OwnerUserID,
		Name:          k.Name,
		KeyType:       k.KeyType,
		Fingerprint:   k.Fingerprint,
		HasPassphrase: k.HasPassphrase,
		CreatedAt:     k.CreatedAt,
	}
}

// ValidateKeyName ensures an SSH key label is non-empty and within safe display bounds.
func ValidateKeyName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len(trimmed) > 128 {
		return fmt.Errorf("%w: key name must be between 1 and 128 characters", ErrInvalidInput)
	}
	return nil
}
