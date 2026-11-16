package crypto_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/adapters/crypto"
	"ec2-shell-desk/internal/core/domain"
)

func newTestVaultCrypto(t *testing.T) *crypto.VaultCrypto {
	t.Helper()
	vc, err := crypto.NewVaultCrypto(crypto.Argon2Params{
		Time:    1,
		Memory:  1024,
		Threads: 1,
		KeyLen:  32,
		SaltLen: 16,
	})
	if err != nil {
		t.Fatalf("NewVaultCrypto() error = %v", err)
	}
	return vc
}

func generateTestEd25519PEM(t *testing.T, passphrase []byte) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	var block *pem.Block
	if len(passphrase) > 0 {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "test-key", passphrase)
	} else {
		block, err = ssh.MarshalPrivateKey(priv, "test-key")
	}
	if err != nil {
		t.Fatalf("ssh.MarshalPrivateKey() error = %v", err)
	}
	return pem.EncodeToMemory(block)
}

func TestDefaultArgon2Params(t *testing.T) {
	vc, err := crypto.NewVaultCrypto(crypto.DefaultArgon2Params())
	if err != nil {
		t.Fatalf("NewVaultCrypto(DefaultArgon2Params) error = %v", err)
	}
	salt, err := vc.GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}
	hash, err := vc.HashPassword("StrongPassword!2026", salt)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !vc.VerifyPassword("StrongPassword!2026", salt, hash) {
		t.Fatal("VerifyPassword() returned false for valid password")
	}
	if vc.VerifyPassword("WrongPassword!2026", salt, hash) {
		t.Fatal("VerifyPassword() returned true for wrong password")
	}
}

func TestEncryptDecryptAndAADTamping(t *testing.T) {
	vc := newTestVaultCrypto(t)
	salt, err := vc.GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}
	masterKey, err := vc.DeriveMasterKey("vault-passphrase", salt)
	if err != nil {
		t.Fatalf("DeriveMasterKey() error = %v", err)
	}
	defer crypto.ZeroBytes(masterKey)

	plaintext := generateTestEd25519PEM(t, nil)
	aad := []byte("user-1:key-1")

	ciphertext, err := vc.Encrypt(masterKey, plaintext, aad)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if bytes.Contains(ciphertext, []byte("PRIVATE KEY")) {
		t.Fatal("ciphertext contains plaintext PEM header")
	}

	decrypted, err := vc.Decrypt(masterKey, ciphertext, aad)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("decrypted bytes do not match original plaintext")
	}

	if _, err := vc.Decrypt(masterKey, ciphertext, []byte("user-2:key-1")); err == nil {
		t.Fatal("expected Decrypt() to fail with mismatched AAD")
	}

	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 0xff
	if _, err := vc.Decrypt(masterKey, tampered, aad); err == nil {
		t.Fatal("expected Decrypt() to fail on tampered ciphertext")
	}
}

func TestInspectPrivateKey(t *testing.T) {
	vc := newTestVaultCrypto(t)
	unencryptedPEM := generateTestEd25519PEM(t, nil)
	encryptedPEM := generateTestEd25519PEM(t, []byte("key-secret"))

	tests := []struct {
		name       string
		rawPEM     []byte
		passphrase []byte
		wantType   domain.KeyType
		wantErr    error
	}{
		{
			name:     "valid unencrypted ed25519",
			rawPEM:   unencryptedPEM,
			wantType: domain.KeyTypeEd25519,
		},
		{
			name:       "valid encrypted ed25519 with passphrase",
			rawPEM:     encryptedPEM,
			passphrase: []byte("key-secret"),
			wantType:   domain.KeyTypeEd25519,
		},
		{
			name:    "encrypted ed25519 without passphrase",
			rawPEM:  encryptedPEM,
			wantErr: domain.ErrPassphraseRequired,
		},
		{
			name:    "putty ppk rejected with guidance",
			rawPEM:  []byte("PuTTY-User-Key-File-2: ssh-rsa\nEncryption: none\n"),
			wantErr: domain.ErrUnsupportedKeyFormat,
		},
		{
			name:    "empty key",
			rawPEM:  []byte("   "),
			wantErr: domain.ErrInvalidInput,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := vc.InspectPrivateKey(tc.rawPEM, tc.passphrase)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("InspectPrivateKey() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("InspectPrivateKey() unexpected error = %v", err)
			}
			if meta.KeyType != tc.wantType {
				t.Fatalf("KeyType = %q, want %q", meta.KeyType, tc.wantType)
			}
			if !strings.HasPrefix(meta.Fingerprint, "SHA256:") {
				t.Fatalf("Fingerprint = %q, expected SHA256: prefix", meta.Fingerprint)
			}
		})
	}
}

func TestZeroBytes(t *testing.T) {
	secret := []byte("super-secret-bytes")
	crypto.ZeroBytes(secret)
	for i, b := range secret {
		if b != 0 {
			t.Fatalf("byte at index %d was not zeroed: %d", i, b)
		}
	}
}
