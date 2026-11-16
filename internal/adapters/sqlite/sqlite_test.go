package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"ec2-shell-desk/internal/adapters/sqlite"
	"ec2-shell-desk/internal/core/domain"
)

func openTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	store, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestMigrationsIdempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "reopen.db")

	s1, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("first sqlite.Open() error = %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	s2, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("second sqlite.Open() error = %v", err)
	}
	_ = s2.Close()
}

func TestMultiUserIsolationAndCRUD(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	userRepo := sqlite.NewUserRepository(store)
	keyRepo := sqlite.NewKeyRepository(store)
	hostRepo := sqlite.NewHostRepository(store)
	knownRepo := sqlite.NewKnownHostRepository(store)

	now := time.Now().UTC().Truncate(time.Millisecond)

	alice := domain.User{
		ID:           "user-alice",
		Username:     "alice",
		PasswordHash: []byte("hash-a"),
		AuthSalt:     []byte("0123456789abcdef"),
		VaultSalt:    []byte("fedcba9876543210"),
		IsDefault:    true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	bob := domain.User{
		ID:           "user-bob",
		Username:     "bob",
		PasswordHash: []byte("hash-b"),
		AuthSalt:     []byte("1123456789abcdef"),
		VaultSalt:    []byte("eedcba9876543210"),
		IsDefault:    false,
		CreatedAt:    now.Add(time.Second),
		UpdatedAt:    now.Add(time.Second),
	}

	if err := userRepo.Create(ctx, alice); err != nil {
		t.Fatalf("Create(alice) error = %v", err)
	}
	if err := userRepo.Create(ctx, bob); err != nil {
		t.Fatalf("Create(bob) error = %v", err)
	}
	if err := userRepo.Create(ctx, alice); !errors.Is(err, domain.ErrUserAlreadyExists) {
		t.Fatalf("duplicate user error = %v, want ErrUserAlreadyExists", err)
	}

	count, err := userRepo.Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("Count() = (%d, %v), want (2, nil)", count, err)
	}

	aliceKey := domain.SSHKey{
		ID:           "key-alice-1",
		OwnerUserID:  alice.ID,
		Name:         "aws-prod.pem",
		KeyType:      domain.KeyTypeEd25519,
		Fingerprint:  "SHA256:alicefp",
		EncryptedPEM: []byte("cipher-pem-alice"),
		CreatedAt:    now,
	}
	if err := keyRepo.Create(ctx, aliceKey); err != nil {
		t.Fatalf("Create(aliceKey) error = %v", err)
	}

	// Bob must not see or delete Alice's key, and empty slices must be non-nil for JSON marshaling.
	if _, err := keyRepo.GetByID(ctx, bob.ID, aliceKey.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob GetByID(aliceKey) = %v, want ErrNotFound", err)
	}
	if bobKeys, err := keyRepo.ListByUser(ctx, bob.ID); err != nil || bobKeys == nil || len(bobKeys) != 0 {
		t.Fatalf("Bob ListByUser() = (%v, %v), want non-nil empty slice", bobKeys, err)
	}
	if err := keyRepo.Delete(ctx, bob.ID, aliceKey.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob Delete(aliceKey) = %v, want ErrNotFound", err)
	}

	// Alice creates a host referencing her key.
	aliceHost := domain.HostInstance{
		ID:          "host-alice-1",
		OwnerUserID: alice.ID,
		Alias:       "EC2 API",
		HostAddress: "54.10.20.30",
		Port:        22,
		SSHUsername: "ec2-user",
		SSHKeyID:    aliceKey.ID,
		Environment: domain.EnvProd,
		Notes:       "Primary instance",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := hostRepo.Create(ctx, aliceHost); err != nil {
		t.Fatalf("Create(aliceHost) error = %v", err)
	}

	refs, err := keyRepo.CountHostReferences(ctx, alice.ID, aliceKey.ID)
	if err != nil || refs != 1 {
		t.Fatalf("CountHostReferences() = (%d, %v), want (1, nil)", refs, err)
	}

	// Bob must not see, update, touch, or delete Alice's host.
	if _, err := hostRepo.GetByID(ctx, bob.ID, aliceHost.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob GetByID(aliceHost) = %v, want ErrNotFound", err)
	}
	if bobHosts, err := hostRepo.ListByUser(ctx, bob.ID); err != nil || bobHosts == nil || len(bobHosts) != 0 {
		t.Fatalf("Bob ListByUser() = (%v, %v), want non-nil empty slice", bobHosts, err)
	}
	bobSpoof := aliceHost
	bobSpoof.OwnerUserID = bob.ID
	bobSpoof.Alias = "Hijacked"
	if err := hostRepo.Update(ctx, bobSpoof); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob Update(aliceHost) = %v, want ErrNotFound", err)
	}
	if err := hostRepo.Delete(ctx, bob.ID, aliceHost.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob Delete(aliceHost) = %v, want ErrNotFound", err)
	}

	// Alice updates and touches last_connected_at successfully.
	aliceHost.Alias = "EC2 API Updated"
	aliceHost.UpdatedAt = now.Add(time.Minute)
	if err := hostRepo.Update(ctx, aliceHost); err != nil {
		t.Fatalf("Alice Update() error = %v", err)
	}
	if err := hostRepo.TouchLastConnected(ctx, alice.ID, aliceHost.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("TouchLastConnected() error = %v", err)
	}

	gotHost, err := hostRepo.GetByID(ctx, alice.ID, aliceHost.ID)
	if err != nil {
		t.Fatalf("Alice GetByID(aliceHost) error = %v", err)
	}
	if gotHost.Alias != "EC2 API Updated" || gotHost.LastConnectedAt == nil {
		t.Fatalf("unexpected updated host: %+v", gotHost)
	}

	// Known hosts isolation.
	kh := domain.KnownHost{
		ID:            "kh-1",
		OwnerUserID:   alice.ID,
		HostAddress:   "54.10.20.30",
		Port:          22,
		KeyType:       "ssh-ed25519",
		Fingerprint:   "SHA256:hostfp",
		PublicKeyBlob: []byte("pubkey-blob"),
		TrustedAt:     now,
	}
	if err := knownRepo.Upsert(ctx, kh); err != nil {
		t.Fatalf("Upsert(knownHost) error = %v", err)
	}
	if _, err := knownRepo.GetByHostPort(ctx, bob.ID, "54.10.20.30", 22); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob GetByHostPort() = %v, want ErrNotFound", err)
	}
	if gotKH, err := knownRepo.GetByHostPort(ctx, alice.ID, "54.10.20.30", 22); err != nil || gotKH.Fingerprint != kh.Fingerprint {
		t.Fatalf("Alice GetByHostPort() = (%+v, %v)", gotKH, err)
	}
}
