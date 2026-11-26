package services_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ec2-shell-desk/internal/adapters/sqlite"
	"ec2-shell-desk/internal/adapters/sshclient"
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
	"ec2-shell-desk/internal/services"
)

func TestSnippetServiceCRUDExecutionAndIsolation(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	store, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	vc := newFastCrypto(t)
	userRepo := sqlite.NewUserRepository(store)
	keyRepo := sqlite.NewKeyRepository(store)
	hostRepo := sqlite.NewHostRepository(store)
	knownRepo := sqlite.NewKnownHostRepository(store)
	groupRepo := sqlite.NewGroupRepository(store)
	snippetRepo := sqlite.NewSnippetRepository(store)

	authSvc := services.NewAuthService(userRepo, vc, vc)
	vaultSvc := services.NewVaultService(authSvc, keyRepo, vc, vc)
	hostSvc := services.NewHostService(authSvc, hostRepo, keyRepo, groupRepo, nil, nil)

	termSink := newSpyTerminalSink()
	sshMgr := sshclient.NewManager(nil, sshclient.WithDialTimeout(5*time.Second))
	sessionSvc := services.NewSessionService(authSvc, hostRepo, knownRepo, vaultSvc, sshMgr, termSink)
	t.Cleanup(sessionSvc.CloseAll)

	snippetSvc := services.NewSnippetService(authSvc, snippetRepo, hostRepo, groupRepo, sessionSvc)

	if _, err := authSvc.BootstrapInitialUser(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("BootstrapInitialUser: %v", err)
	}
	if _, err := authSvc.CreateUser(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("CreateUser(bob): %v", err)
	}

	rawPEM, clientPub := generateClientPEMAndPub(t)
	srv := startFullTestSSHServer(t, clientPub, t.TempDir())

	keySummary, err := vaultSvc.ImportKey(ctx, "deploy.pem", rawPEM, nil)
	if err != nil {
		t.Fatalf("ImportKey: %v", err)
	}
	grp, err := hostSvc.CreateGroup(ctx, "Trabajo")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	host, err := hostSvc.CreateHost(ctx, services.SaveHostInput{
		GroupID:     grp.ID,
		Alias:       "App Prod",
		HostAddress: srv.host,
		Port:        srv.port,
		SSHUsername: "ec2-user",
		SSHKeyID:    keySummary.ID,
	})
	if err != nil {
		t.Fatalf("CreateHost: %v", err)
	}

	host2, err := hostSvc.CreateHost(ctx, services.SaveHostInput{
		GroupID:     grp.ID,
		Alias:       "App Secondary",
		HostAddress: srv.host,
		Port:        srv.port,
		SSHUsername: "ec2-user",
		SSHKeyID:    keySummary.ID,
	})
	if err != nil {
		t.Fatalf("CreateHost(host2): %v", err)
	}

	sess, err := sessionSvc.OpenSession(ctx, host.ID, 80, 24, "", func(_ context.Context, _ ports.HostKeyChallenge) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}

	snip, err := snippetSvc.CreateSnippet(ctx, services.SaveSnippetInput{
		HostID:      host.ID,
		GroupID:     grp.ID,
		Name:        "Deploy Pull",
		Command:     "git pull && docker compose up -d",
		AutoExecute: true,
		Notes:       "update staging/prod",
	})
	if err != nil || snip.HostID != host.ID {
		t.Fatalf("CreateSnippet = (%+v, %v)", snip, err)
	}
	if _, err := snippetSvc.CreateSnippet(ctx, services.SaveSnippetInput{
		HostID:      host2.ID,
		GroupID:     grp.ID,
		Name:        "Deploy Pull",
		Command:     "cd /srv/secondary && git pull",
		AutoExecute: true,
	}); err != nil {
		t.Fatalf("CreateSnippet same name on second host: %v", err)
	}

	if err := snippetSvc.RunSnippetInSession(ctx, sess.SessionID, snip.ID); err != nil {
		t.Fatalf("RunSnippetInSession: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(termSink.OutputString(), "git pull && docker compose up -d\r") {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if !strings.Contains(termSink.OutputString(), "git pull && docker compose up -d\r") {
		t.Fatalf("expected snippet command with carriage return in terminal output, got %q", termSink.OutputString())
	}

	// Update to paste-only (AutoExecute = false) and broadcast.
	updated, err := snippetSvc.UpdateSnippet(ctx, services.SaveSnippetInput{
		ID:          snip.ID,
		HostID:      host.ID,
		GroupID:     grp.ID,
		Name:        "Docker Logs Prefix",
		Command:     "docker logs -f ",
		AutoExecute: false,
	})
	if err != nil || updated.AutoExecute {
		t.Fatalf("UpdateSnippet = (%+v, %v)", updated, err)
	}
	if err := snippetSvc.BroadcastSnippet(ctx, []string{sess.SessionID}, updated.ID); err != nil {
		t.Fatalf("BroadcastSnippet: %v", err)
	}

	// Multi-user isolation: Bob cannot list, run, link Alice's group, or delete Alice's snippet.
	if _, err := authSvc.Login(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("Login(bob): %v", err)
	}
	if bobSnips, err := snippetSvc.ListSnippets(ctx); err != nil || len(bobSnips) != 0 {
		t.Fatalf("Bob ListSnippets = (%d, %v), want (0, nil)", len(bobSnips), err)
	}
	if err := snippetSvc.RunSnippetInSession(ctx, sess.SessionID, snip.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob RunSnippetInSession = %v, want ErrNotFound", err)
	}
	if _, err := snippetSvc.CreateSnippet(ctx, services.SaveSnippetInput{
		HostID:  host.ID,
		Name:    "Hijack Host",
		Command: "ls",
	}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob CreateSnippet with Alice's host = %v, want ErrNotFound", err)
	}
	if _, err := snippetSvc.CreateSnippet(ctx, services.SaveSnippetInput{
		GroupID: grp.ID,
		Name:    "Hijack Group",
		Command: "ls",
	}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob CreateSnippet with Alice's group = %v, want ErrNotFound", err)
	}
	if err := snippetSvc.DeleteSnippet(ctx, snip.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob DeleteSnippet = %v, want ErrNotFound", err)
	}

	// Switch back to Alice and delete snippet.
	if _, err := authSvc.Login(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("Login(alice): %v", err)
	}
	if err := snippetSvc.DeleteSnippet(ctx, snip.ID); err != nil {
		t.Fatalf("Alice DeleteSnippet: %v", err)
	}
}
