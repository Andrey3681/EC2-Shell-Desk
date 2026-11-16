package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const (
	defaultArgon2Time    uint32 = 3
	defaultArgon2Memory  uint32 = 64 * 1024
	defaultArgon2Threads uint8  = 4
	defaultArgon2KeyLen  uint32 = 32
	defaultSaltLength    int    = 16
	gcmNonceSize         int    = 12
)

// Argon2Params configures Argon2id key derivation cost and output length.
type Argon2Params struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
	SaltLen int
}

// DefaultArgon2Params returns production-grade Argon2id parameters for local vault derivation.
func DefaultArgon2Params() Argon2Params {
	return Argon2Params{
		Time:    defaultArgon2Time,
		Memory:  defaultArgon2Memory,
		Threads: defaultArgon2Threads,
		KeyLen:  defaultArgon2KeyLen,
		SaltLen: defaultSaltLength,
	}
}

// VaultCrypto implements password hashing, AES-256-GCM encryption, and SSH private key inspection.
type VaultCrypto struct {
	params Argon2Params
}

// NewVaultCrypto constructs a VaultCrypto adapter validated against minimum security constraints.
func NewVaultCrypto(params Argon2Params) (*VaultCrypto, error) {
	if params.Time == 0 || params.Memory == 0 || params.Threads == 0 || params.KeyLen != 32 || params.SaltLen < 16 {
		return nil, fmt.Errorf("%w: invalid Argon2id parameters (requires 32-byte key and >=16-byte salt)", domain.ErrInvalidInput)
	}
	return &VaultCrypto{params: params}, nil
}

// ZeroBytes overwrites every byte in b with zero to reduce sensitive data retention in memory.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// GenerateSalt generates a cryptographically random salt of the configured length.
func (v *VaultCrypto) GenerateSalt() ([]byte, error) {
	salt := make([]byte, v.params.SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate random salt: %w", err)
	}
	return salt, nil
}

// HashPassword derives an Argon2id password verifier hash using the provided salt.
func (v *VaultCrypto) HashPassword(password string, salt []byte) ([]byte, error) {
	if password == "" || len(salt) < 16 {
		return nil, fmt.Errorf("%w: password and >=16-byte salt are required", domain.ErrInvalidInput)
	}
	pwBytes := []byte(password)
	defer ZeroBytes(pwBytes)

	hash := argon2.IDKey(pwBytes, salt, v.params.Time, v.params.Memory, v.params.Threads, v.params.KeyLen)
	return hash, nil
}

// VerifyPassword checks whether password produces expectedHash using constant-time comparison.
func (v *VaultCrypto) VerifyPassword(password string, salt, expectedHash []byte) bool {
	if password == "" || len(salt) < 16 || len(expectedHash) == 0 {
		return false
	}
	computed, err := v.HashPassword(password, salt)
	if err != nil {
		return false
	}
	defer ZeroBytes(computed)

	return subtle.ConstantTimeCompare(computed, expectedHash) == 1
}

// DeriveMasterKey derives a 256-bit AES master key from the user's password and vault salt.
func (v *VaultCrypto) DeriveMasterKey(password string, vaultSalt []byte) ([]byte, error) {
	return v.HashPassword(password, vaultSalt)
}

// Encrypt seals plaintext using AES-256-GCM with a random 12-byte nonce and authenticated AAD.
func (v *VaultCrypto) Encrypt(masterKey, plaintext, aad []byte) ([]byte, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("%w: master key must be 32 bytes", domain.ErrInvalidInput)
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create aes-gcm: %w", err)
	}

	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate aes-gcm nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, plaintext, aad)
	return sealed, nil
}

// Decrypt authenticates and decrypts ciphertext produced by Encrypt using the same masterKey and aad.
func (v *VaultCrypto) Decrypt(masterKey, ciphertext, aad []byte) ([]byte, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("%w: master key must be 32 bytes", domain.ErrInvalidInput)
	}
	if len(ciphertext) < gcmNonceSize {
		return nil, fmt.Errorf("%w: ciphertext too short", domain.ErrInvalidInput)
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create aes-gcm: %w", err)
	}

	nonce := ciphertext[:gcmNonceSize]
	payload := ciphertext[gcmNonceSize:]

	plaintext, err := gcm.Open(nil, nonce, payload, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt aes-gcm payload: %w", err)
	}
	return plaintext, nil
}

// InspectPrivateKey parses PEM/OpenSSH private key bytes, validates any passphrase, and extracts metadata.
func (v *VaultCrypto) InspectPrivateKey(rawPEM, passphrase []byte) (ports.KeyMetadata, error) {
	trimmed := bytes.TrimSpace(rawPEM)
	if len(trimmed) == 0 {
		return ports.KeyMetadata{}, fmt.Errorf("%w: private key content is empty", domain.ErrInvalidInput)
	}

	if bytes.HasPrefix(trimmed, []byte("PuTTY-User-Key-File")) {
		return ports.KeyMetadata{}, fmt.Errorf("%w: PuTTY .ppk keys must be exported to OpenSSH PEM format first", domain.ErrUnsupportedKeyFormat)
	}

	var signer ssh.Signer
	var err error
	if len(passphrase) > 0 {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(trimmed, passphrase)
	} else {
		signer, err = ssh.ParsePrivateKey(trimmed)
	}
	if err != nil {
		var missingErr *ssh.PassphraseMissingError
		if errors.As(err, &missingErr) {
			return ports.KeyMetadata{}, fmt.Errorf("%w", domain.ErrPassphraseRequired)
		}
		return ports.KeyMetadata{}, fmt.Errorf("%w: failed to parse private key: %w", domain.ErrUnsupportedKeyFormat, err)
	}

	pub := signer.PublicKey()
	keyType, mapErr := mapSSHKeyType(pub.Type())
	if mapErr != nil {
		return ports.KeyMetadata{}, mapErr
	}

	return ports.KeyMetadata{
		KeyType:     keyType,
		Fingerprint: ssh.FingerprintSHA256(pub),
	}, nil
}

func mapSSHKeyType(sshAlgo string) (domain.KeyType, error) {
	switch {
	case sshAlgo == ssh.KeyAlgoRSA:
		return domain.KeyTypeRSA, nil
	case sshAlgo == ssh.KeyAlgoED25519:
		return domain.KeyTypeEd25519, nil
	case strings.HasPrefix(sshAlgo, "ecdsa-sha2-"):
		return domain.KeyTypeECDSA, nil
	default:
		return "", fmt.Errorf("%w: unsupported key algorithm %q", domain.ErrUnsupportedKeyFormat, sshAlgo)
	}
}
