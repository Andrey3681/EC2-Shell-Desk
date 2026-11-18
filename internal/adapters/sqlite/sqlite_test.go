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

	// SSH Tunnels CRUD & strict owner_user_id isolation.
	tunnelRepo := sqlite.NewTunnelRepository(store)
	tun := domain.SSHTunnel{
		ID:          "tun-1",
		OwnerUserID: alice.ID,
		HostID:      aliceHost.ID,
		Name:        "RDS Postgres Prod",
		Preset:      domain.TunnelPresetPostgreSQL,
		LocalHost:   domain.DefaultTunnelLocalHost,
		LocalPort:   15432,
		RemoteHost:  "rds.internal",
		RemotePort:  5432,
		Notes:       "pgAdmin",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := tunnelRepo.Create(ctx, tun); err != nil {
		t.Fatalf("Create(tunnel) error = %v", err)
	}
	if bobTunnels, err := tunnelRepo.ListByUser(ctx, bob.ID); err != nil || bobTunnels == nil || len(bobTunnels) != 0 {
		t.Fatalf("Bob ListByUser(tunnels) = (%v, %v), want non-nil empty slice", bobTunnels, err)
	}
	if _, err := tunnelRepo.GetByID(ctx, bob.ID, tun.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob GetByID(tunnel) = %v, want ErrNotFound", err)
	}
	if err := tunnelRepo.Delete(ctx, bob.ID, tun.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob Delete(tunnel) = %v, want ErrNotFound", err)
	}
	if countByHost, err := tunnelRepo.CountByHost(ctx, alice.ID, aliceHost.ID); err != nil || countByHost != 1 {
		t.Fatalf("CountByHost() = (%d, %v), want (1, nil)", countByHost, err)
	}
	tun.LocalPort = 25432
	tun.UpdatedAt = now.Add(time.Minute)
	if err := tunnelRepo.Update(ctx, tun); err != nil {
		t.Fatalf("Alice Update(tunnel) error = %v", err)
	}
	gotTun, err := tunnelRepo.GetByID(ctx, alice.ID, tun.ID)
	if err != nil || gotTun.LocalPort != 25432 {
		t.Fatalf("Alice GetByID(tunnel) = (%+v, %v)", gotTun, err)
	}
	if err := tunnelRepo.Delete(ctx, alice.ID, tun.ID); err != nil {
		t.Fatalf("Alice Delete(tunnel) error = %v", err)
	}

	// Host Groups CRUD, multi-user isolation, and ON DELETE SET NULL on host_instances.
	groupRepo := sqlite.NewGroupRepository(store)
	uniGroup := domain.HostGroup{
		ID:          "grp-uni",
		OwnerUserID: alice.ID,
		Name:        "Universidad",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := groupRepo.Create(ctx, uniGroup); err != nil {
		t.Fatalf("Create(uniGroup) error = %v", err)
	}
	if err := groupRepo.Create(ctx, uniGroup); !errors.Is(err, domain.ErrDuplicateRecord) {
		t.Fatalf("duplicate group error = %v, want ErrDuplicateRecord", err)
	}
	if bobGroups, err := groupRepo.ListByUser(ctx, bob.ID); err != nil || bobGroups == nil || len(bobGroups) != 0 {
		t.Fatalf("Bob ListByUser(groups) = (%v, %v), want non-nil empty slice", bobGroups, err)
	}
	if _, err := groupRepo.GetByID(ctx, bob.ID, uniGroup.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob GetByID(uniGroup) = %v, want ErrNotFound", err)
	}
	if err := groupRepo.Delete(ctx, bob.ID, uniGroup.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob Delete(uniGroup) = %v, want ErrNotFound", err)
	}

	// Assign Alice's host to Universidad, then delete the group -> host remains with GroupID == "".
	aliceHost.GroupID = uniGroup.ID
	aliceHost.UpdatedAt = now.Add(3 * time.Minute)
	if err := hostRepo.Update(ctx, aliceHost); err != nil {
		t.Fatalf("assign host group error = %v", err)
	}
	gotWithGroup, err := hostRepo.GetByID(ctx, alice.ID, aliceHost.ID)
	if err != nil || gotWithGroup.GroupID != uniGroup.ID {
		t.Fatalf("expected GroupID %q, got (%+v, %v)", uniGroup.ID, gotWithGroup, err)
	}
	if err := groupRepo.Delete(ctx, alice.ID, uniGroup.ID); err != nil {
		t.Fatalf("Alice Delete(uniGroup) error = %v", err)
	}
	gotAfterGroupDel, err := hostRepo.GetByID(ctx, alice.ID, aliceHost.ID)
	if err != nil || gotAfterGroupDel.GroupID != "" {
		t.Fatalf("expected ungrouped host after group delete, got (%+v, %v)", gotAfterGroupDel, err)
	}

	// Command Snippets CRUD, duplicate protection, host scoping, and multi-user isolation.
	snippetRepo := sqlite.NewSnippetRepository(store)
	snip := domain.CommandSnippet{
		ID:          "snip-1",
		OwnerUserID: alice.ID,
		HostID:      aliceHost.ID,
		Name:        "Update Deploy",
		Command:     "git pull && docker compose up -d",
		AutoExecute: true,
		Notes:       "backend update",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := snippetRepo.Create(ctx, snip); err != nil {
		t.Fatalf("Create(snippet) error = %v", err)
	}
	if err := snippetRepo.Create(ctx, snip); !errors.Is(err, domain.ErrDuplicateRecord) {
		t.Fatalf("duplicate snippet error = %v, want ErrDuplicateRecord", err)
	}
	globalSameName := domain.CommandSnippet{
		ID:          "snip-global-same",
		OwnerUserID: alice.ID,
		Name:        "Update Deploy",
		Command:     "git status",
		AutoExecute: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := snippetRepo.Create(ctx, globalSameName); err != nil {
		t.Fatalf("Create(global snippet same name) error = %v", err)
	}
	if bobSnips, err := snippetRepo.ListByUser(ctx, bob.ID); err != nil || bobSnips == nil || len(bobSnips) != 0 {
		t.Fatalf("Bob ListByUser(snippets) = (%v, %v), want non-nil empty slice", bobSnips, err)
	}
	if _, err := snippetRepo.GetByID(ctx, bob.ID, snip.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob GetByID(snippet) = %v, want ErrNotFound", err)
	}
	if err := snippetRepo.Delete(ctx, bob.ID, snip.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob Delete(snippet) = %v, want ErrNotFound", err)
	}
	snip.Command = "docker compose pull && docker compose up -d"
	snip.AutoExecute = false
	snip.UpdatedAt = now.Add(time.Minute)
	if err := snippetRepo.Update(ctx, snip); err != nil {
		t.Fatalf("Alice Update(snippet) error = %v", err)
	}
	gotSnip, err := snippetRepo.GetByID(ctx, alice.ID, snip.ID)
	if err != nil || gotSnip.AutoExecute || gotSnip.Command != snip.Command || gotSnip.HostID != aliceHost.ID {
		t.Fatalf("Alice GetByID(snippet) = (%+v, %v)", gotSnip, err)
	}
	if err := snippetRepo.Delete(ctx, alice.ID, snip.ID); err != nil {
		t.Fatalf("Alice Delete(snippet) error = %v", err)
	}

	// Private Services CRUD, uniqueness by (owner_user_id, host_id, name), host query, and multi-user isolation.
	serviceRepo := sqlite.NewServiceRepository(store)
	service1 := domain.PrivateService{
		ID:          "svc-1",
		OwnerUserID: alice.ID,
		HostID:      aliceHost.ID,
		Name:        "Grafana",
		Preset:      domain.ServicePresetGrafana,
		Protocol:    domain.ServiceProtocolHTTP,
		RemoteHost:  "127.0.0.1",
		RemotePort:  3001,
		BasePath:    "/grafana",
		LocalPort:   13001,
		HealthPath:  "/api/health",
		OpenMode:    domain.ServiceOpenModeBrowser,
		Notes:       "metrics dashboard",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := serviceRepo.Create(ctx, service1); err != nil {
		t.Fatalf("Create(service) error = %v", err)
	}
	if err := serviceRepo.Create(ctx, service1); !errors.Is(err, domain.ErrDuplicateRecord) {
		t.Fatalf("duplicate service error = %v, want ErrDuplicateRecord", err)
	}

	// Bob cannot access or delete Alice's service
	if bobSvcs, err := serviceRepo.ListByUser(ctx, bob.ID); err != nil || len(bobSvcs) != 0 {
		t.Fatalf("Bob ListByUser(services) = (%v, %v), want empty slice", bobSvcs, err)
	}
	if _, err := serviceRepo.GetByID(ctx, bob.ID, service1.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob GetByID(service) = %v, want ErrNotFound", err)
	}
	if err := serviceRepo.Delete(ctx, bob.ID, service1.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob Delete(service) = %v, want ErrNotFound", err)
	}

	// Alice queries
	byHost, err := serviceRepo.ListByHost(ctx, alice.ID, aliceHost.ID)
	if err != nil || len(byHost) != 1 || byHost[0].Name != "Grafana" {
		t.Fatalf("ListByHost() = (%v, %v)", byHost, err)
	}
	svcCount, err := serviceRepo.CountByHost(ctx, alice.ID, aliceHost.ID)
	if err != nil || svcCount != 1 {
		t.Fatalf("CountByHost() = (%d, %v), want 1", svcCount, err)
	}

	service1.Notes = "updated notes"
	service1.UpdatedAt = now.Add(time.Minute)
	if err := serviceRepo.Update(ctx, service1); err != nil {
		t.Fatalf("Update(service) error = %v", err)
	}
	gotSvc, err := serviceRepo.GetByID(ctx, alice.ID, service1.ID)
	if err != nil || gotSvc.Notes != "updated notes" {
		t.Fatalf("GetByID(service) = (%+v, %v)", gotSvc, err)
	}
	if err := serviceRepo.Delete(ctx, alice.ID, service1.ID); err != nil {
		t.Fatalf("Delete(service) error = %v", err)
	}
}
