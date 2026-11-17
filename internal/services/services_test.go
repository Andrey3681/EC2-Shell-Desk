package services_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/pem"
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	_ "modernc.org/sqlite"

	"ec2-shell-desk/internal/adapters/crypto"
	"ec2-shell-desk/internal/adapters/sqlite"
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

func newFastCrypto(t *testing.T) *crypto.VaultCrypto {
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

func generateDummyEd25519Key(t *testing.T, passphrase []byte) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	var block *pem.Block
	if len(passphrase) > 0 {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "ec2-test", passphrase)
	} else {
		block, err = ssh.MarshalPrivateKey(priv, "ec2-test")
	}
	if err != nil {
		t.Fatalf("ssh.MarshalPrivateKey() error = %v", err)
	}
	return pem.EncodeToMemory(block)
}

func setupPhase2Stack(t *testing.T) (string, *services.AuthService, *services.VaultService, *services.HostService) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	store, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	vc := newFastCrypto(t)
	userRepo := sqlite.NewUserRepository(store)
	keyRepo := sqlite.NewKeyRepository(store)
	hostRepo := sqlite.NewHostRepository(store)
	tunnelRepo := sqlite.NewTunnelRepository(store)

	authSvc := services.NewAuthService(userRepo, vc, vc)
	vaultSvc := services.NewVaultService(authSvc, keyRepo, vc, vc)
	hostSvc := services.NewHostService(authSvc, hostRepo, keyRepo, tunnelRepo)

	return dbPath, authSvc, vaultSvc, hostSvc
}

func TestAuthBootstrapLoginLockAndMultiUser(t *testing.T) {
	ctx := context.Background()
	_, authSvc, _, _ := setupPhase2Stack(t)

	hasUsers, err := authSvc.HasUsers(ctx)
	if err != nil || hasUsers {
		t.Fatalf("HasUsers() = (%v, %v), want (false, nil)", hasUsers, err)
	}

	if _, err := authSvc.CurrentUser(); !errors.Is(err, domain.ErrVaultLocked) {
		t.Fatalf("CurrentUser() before login = %v, want ErrVaultLocked", err)
	}

	// Bootstrap initial default user.
	admin, err := authSvc.BootstrapInitialUser(ctx, "admin", "AdminPass!2026")
	if err != nil {
		t.Fatalf("BootstrapInitialUser() error = %v", err)
	}
	if !admin.IsDefault || admin.Username != "admin" {
		t.Fatalf("unexpected initial user profile: %+v", admin)
	}

	// Second bootstrap attempt must fail.
	if _, err := authSvc.BootstrapInitialUser(ctx, "other", "OtherPass!2026"); !errors.Is(err, domain.ErrInitialUserAlreadySet) {
		t.Fatalf("second BootstrapInitialUser() = %v, want ErrInitialUserAlreadySet", err)
	}

	// Create secondary user.
	devUser, err := authSvc.CreateUser(ctx, "developer", "DevPass!2026")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if devUser.IsDefault {
		t.Fatal("secondary user must not be marked IsDefault")
	}

	// Lock wipes active session.
	authSvc.Lock()
	if _, err := authSvc.CurrentUser(); !errors.Is(err, domain.ErrVaultLocked) {
		t.Fatalf("CurrentUser() after Lock = %v, want ErrVaultLocked", err)
	}

	// Wrong password fails.
	if _, err := authSvc.Login(ctx, "admin", "WrongPassword"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("Login(wrong) = %v, want ErrInvalidCredentials", err)
	}

	// Valid login unlocks.
	loggedIn, err := authSvc.Login(ctx, "admin", "AdminPass!2026")
	if err != nil || loggedIn.ID != admin.ID {
		t.Fatalf("Login(valid) = (%+v, %v)", loggedIn, err)
	}
}

func TestVaultAndHostMultiUserIsolationEndToEnd(t *testing.T) {
	ctx := context.Background()
	dbPath, authSvc, vaultSvc, hostSvc := setupPhase2Stack(t)

	alice, err := authSvc.BootstrapInitialUser(ctx, "alice", "AlicePass!2026")
	if err != nil {
		t.Fatalf("BootstrapInitialUser(alice) error = %v", err)
	}
	if _, err := authSvc.CreateUser(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}

	rawKey := generateDummyEd25519Key(t, []byte("pem-secret"))
	aliceKey, err := vaultSvc.ImportKey(ctx, "aws-prod-east.pem", rawKey, []byte("pem-secret"))
	if err != nil {
		t.Fatalf("ImportKey() error = %v", err)
	}
	if aliceKey.OwnerUserID != alice.ID || !aliceKey.HasPassphrase {
		t.Fatalf("unexpected imported key summary: %+v", aliceKey)
	}

	// Verify SQLite never stored plaintext PEM bytes.
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer func() { _ = rawDB.Close() }()

	var storedPEM []byte
	if err := rawDB.QueryRowContext(ctx, `SELECT encrypted_pem FROM ssh_keys WHERE id = ?`, aliceKey.ID).Scan(&storedPEM); err != nil {
		t.Fatalf("query encrypted_pem error = %v", err)
	}
	if bytes.Contains(storedPEM, []byte("OPENSSH PRIVATE KEY")) {
		t.Fatal("sqlite stored unencrypted PEM key material")
	}

	// Decrypt key for Alice's session and confirm exact roundtrip.
	decPEM, decPass, err := vaultSvc.DecryptKeyForSession(ctx, aliceKey.ID)
	if err != nil {
		t.Fatalf("DecryptKeyForSession() error = %v", err)
	}
	if !bytes.Equal(decPEM, rawKey) || string(decPass) != "pem-secret" {
		t.Fatal("decrypted key or passphrase mismatch")
	}

	// Alice creates a host bound to her key.
	aliceHost, err := hostSvc.CreateHost(ctx, services.SaveHostInput{
		Alias:       "Backend API Prod",
		HostAddress: "54.123.45.67",
		SSHUsername: "ec2-user",
		SSHKeyID:    aliceKey.ID,
		Environment: domain.EnvProd,
		Notes:       "us-east-1",
	})
	if err != nil {
		t.Fatalf("CreateHost() error = %v", err)
	}
	if aliceHost.Port != domain.DefaultSSHPort {
		t.Fatalf("expected default SSH port 22, got %d", aliceHost.Port)
	}

	// Key in use cannot be deleted.
	if err := vaultSvc.DeleteKey(ctx, aliceKey.ID); !errors.Is(err, domain.ErrKeyInUse) {
		t.Fatalf("DeleteKey(in use) = %v, want ErrKeyInUse", err)
	}

	// Switch session to Bob: Bob cannot list, read, link, or delete Alice's keys or hosts.
	if _, err := authSvc.Login(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("Login(bob) error = %v", err)
	}

	if bobKeys, err := vaultSvc.ListKeys(ctx); err != nil || len(bobKeys) != 0 {
		t.Fatalf("Bob ListKeys() = (%d, %v), want (0, nil)", len(bobKeys), err)
	}
	if bobHosts, err := hostSvc.ListHosts(ctx); err != nil || len(bobHosts) != 0 {
		t.Fatalf("Bob ListHosts() = (%d, %v), want (0, nil)", len(bobHosts), err)
	}
	if _, _, err := vaultSvc.DecryptKeyForSession(ctx, aliceKey.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob DecryptKeyForSession(aliceKey) = %v, want ErrNotFound", err)
	}
	if _, err := hostSvc.CreateHost(ctx, services.SaveHostInput{
		Alias:       "Cross-User Attempt",
		HostAddress: "10.0.0.1",
		SSHUsername: "ubuntu",
		SSHKeyID:    aliceKey.ID,
	}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob CreateHost with Alice's key = %v, want ErrNotFound", err)
	}
	if err := hostSvc.DeleteHost(ctx, aliceHost.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob DeleteHost(aliceHost) = %v, want ErrNotFound", err)
	}

	// Switch back to Alice, update host, verify GetHost, delete host, then delete key cleanly.
	if _, err := authSvc.Login(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("Login(alice) error = %v", err)
	}
	updatedHost, err := hostSvc.UpdateHost(ctx, services.SaveHostInput{
		ID:          aliceHost.ID,
		Alias:       "Backend API Staging",
		HostAddress: "54.123.45.99",
		Port:        2222,
		SSHUsername: "ubuntu",
		SSHKeyID:    aliceKey.ID,
		Environment: domain.EnvStaging,
		Notes:       "updated",
	})
	if err != nil {
		t.Fatalf("UpdateHost() error = %v", err)
	}
	fetchedHost, err := hostSvc.GetHost(ctx, updatedHost.ID)
	if err != nil || fetchedHost.Alias != "Backend API Staging" || fetchedHost.Port != 2222 {
		t.Fatalf("GetHost() = (%+v, %v)", fetchedHost, err)
	}

	// Reject PuTTY .ppk imports cleanly.
	if _, err := vaultSvc.ImportKey(ctx, "putty.ppk", []byte("PuTTY-User-Key-File-2: ssh-rsa\n"), nil); !errors.Is(err, domain.ErrUnsupportedKeyFormat) {
		t.Fatalf("ImportKey(.ppk) = %v, want ErrUnsupportedKeyFormat", err)
	}

	if err := hostSvc.DeleteHost(ctx, aliceHost.ID); err != nil {
		t.Fatalf("Alice DeleteHost() error = %v", err)
	}
	if err := vaultSvc.DeleteKey(ctx, aliceKey.ID); err != nil {
		t.Fatalf("Alice DeleteKey() error = %v", err)
	}

	// Locked vault rejects sensitive operations.
	authSvc.Lock()
	if _, err := vaultSvc.ImportKey(ctx, "locked.pem", rawKey, nil); !errors.Is(err, domain.ErrVaultLocked) {
		t.Fatalf("ImportKey(locked) = %v, want ErrVaultLocked", err)
	}
	if _, err := hostSvc.CreateHost(ctx, services.SaveHostInput{Alias: "x", HostAddress: "1.2.3.4", SSHUsername: "ec2-user"}); !errors.Is(err, domain.ErrVaultLocked) {
		t.Fatalf("CreateHost(locked) = %v, want ErrVaultLocked", err)
	}
}
