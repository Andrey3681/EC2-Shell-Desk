package ports

import "ec2-shell-desk/internal/core/domain"

// KeyMetadata describes parsed non-sensitive properties of an SSH private key.
type KeyMetadata struct {
	KeyType     domain.KeyType
	Fingerprint string
}

// PasswordHasher derives salts and verifies local user passwords in constant time.
type PasswordHasher interface {
	GenerateSalt() ([]byte, error)
	HashPassword(password string, salt []byte) ([]byte, error)
	VerifyPassword(password string, salt, expectedHash []byte) bool
}

// KeyEncryptor derives per-user master keys and performs authenticated encryption of vault secrets.
type KeyEncryptor interface {
	DeriveMasterKey(password string, vaultSalt []byte) ([]byte, error)
	Encrypt(masterKey, plaintext, aad []byte) ([]byte, error)
	Decrypt(masterKey, ciphertext, aad []byte) ([]byte, error)
}

// KeyInspector validates raw private key bytes, detects unsupported formats, and computes fingerprints.
type KeyInspector interface {
	InspectPrivateKey(rawPEM, passphrase []byte) (KeyMetadata, error)
}
