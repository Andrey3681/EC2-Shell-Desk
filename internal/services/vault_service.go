package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

// VaultService manages per-user encrypted SSH private keys.
type VaultService struct {
	auth      *AuthService
	keys      ports.KeyRepository
	encryptor ports.KeyEncryptor
	inspector ports.KeyInspector
}

// NewVaultService constructs a VaultService bound to the active AuthService and key ports.
func NewVaultService(
	auth *AuthService,
	keys ports.KeyRepository,
	encryptor ports.KeyEncryptor,
	inspector ports.KeyInspector,
) *VaultService {
	return &VaultService{
		auth:      auth,
		keys:      keys,
		encryptor: encryptor,
		inspector: inspector,
	}
}

// ImportKey validates, encrypts at rest, and stores an SSH private key for the authenticated user.
func (s *VaultService) ImportKey(ctx context.Context, name string, rawPEM, passphrase []byte) (domain.SSHKeySummary, error) {
	trimmedName := strings.TrimSpace(name)
	if err := domain.ValidateKeyName(trimmedName); err != nil {
		return domain.SSHKeySummary{}, fmt.Errorf("validate key name: %w", err)
	}

	userID, masterKey, err := s.auth.ActiveVaultCredentials()
	if err != nil {
		return domain.SSHKeySummary{}, err
	}
	defer wipeBytes(masterKey)

	meta, err := s.inspector.InspectPrivateKey(rawPEM, passphrase)
	if err != nil {
		return domain.SSHKeySummary{}, fmt.Errorf("inspect private key: %w", err)
	}

	keyID := uuid.NewString()
	aad := keyAAD(userID, keyID)

	encryptedPEM, err := s.encryptor.Encrypt(masterKey, rawPEM, aad)
	if err != nil {
		return domain.SSHKeySummary{}, fmt.Errorf("encrypt private key: %w", err)
	}

	var encryptedPass []byte
	hasPassphrase := len(passphrase) > 0
	if hasPassphrase {
		encryptedPass, err = s.encryptor.Encrypt(masterKey, passphrase, aad)
		if err != nil {
			return domain.SSHKeySummary{}, fmt.Errorf("encrypt key passphrase: %w", err)
		}
	}

	record := domain.SSHKey{
		ID:                  keyID,
		OwnerUserID:         userID,
		Name:                trimmedName,
		KeyType:             meta.KeyType,
		Fingerprint:         meta.Fingerprint,
		EncryptedPEM:        encryptedPEM,
		EncryptedPassphrase: encryptedPass,
		HasPassphrase:       hasPassphrase,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.keys.Create(ctx, record); err != nil {
		return domain.SSHKeySummary{}, fmt.Errorf("store encrypted ssh key: %w", err)
	}
	return record.Summary(), nil
}

// ListKeys returns safe SSH key metadata summaries owned exclusively by the active user.
func (s *VaultService) ListKeys(ctx context.Context) ([]domain.SSHKeySummary, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}

	keys, err := s.keys.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list user ssh keys: %w", err)
	}

	summaries := make([]domain.SSHKeySummary, 0, len(keys))
	for _, k := range keys {
		summaries = append(summaries, k.Summary())
	}
	return summaries, nil
}

// DecryptKeyForSession decrypts a user-owned SSH key and optional passphrase in memory for SSH dialing.
// Callers must wipe both returned slices after creating an SSH signer.
func (s *VaultService) DecryptKeyForSession(ctx context.Context, keyID string) ([]byte, []byte, error) {
	if strings.TrimSpace(keyID) == "" {
		return nil, nil, fmt.Errorf("%w: ssh key id is required", domain.ErrInvalidInput)
	}

	userID, masterKey, err := s.auth.ActiveVaultCredentials()
	if err != nil {
		return nil, nil, err
	}
	defer wipeBytes(masterKey)

	key, err := s.keys.GetByID(ctx, userID, keyID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch ssh key: %w", err)
	}

	aad := keyAAD(userID, key.ID)
	rawPEM, err := s.encryptor.Decrypt(masterKey, key.EncryptedPEM, aad)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt ssh private key: %w", err)
	}

	var passphrase []byte
	if key.HasPassphrase && len(key.EncryptedPassphrase) > 0 {
		passphrase, err = s.encryptor.Decrypt(masterKey, key.EncryptedPassphrase, aad)
		if err != nil {
			wipeBytes(rawPEM)
			return nil, nil, fmt.Errorf("decrypt ssh key passphrase: %w", err)
		}
	}

	return rawPEM, passphrase, nil
}

// DeleteKey deletes an SSH key owned by the active user unless host instances still reference it.
func (s *VaultService) DeleteKey(ctx context.Context, keyID string) error {
	if strings.TrimSpace(keyID) == "" {
		return fmt.Errorf("%w: ssh key id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	refs, err := s.keys.CountHostReferences(ctx, user.ID, keyID)
	if err != nil {
		return fmt.Errorf("check ssh key references: %w", err)
	}
	if refs > 0 {
		return fmt.Errorf("%w: referenced by %d host instance(s)", domain.ErrKeyInUse, refs)
	}

	if err := s.keys.Delete(ctx, user.ID, keyID); err != nil {
		return fmt.Errorf("delete ssh key: %w", err)
	}
	return nil
}

func keyAAD(userID, keyID string) []byte {
	return []byte(userID + ":" + keyID)
}
