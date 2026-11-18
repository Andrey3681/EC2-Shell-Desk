package app_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/adapters/crypto"
	"ec2-shell-desk/internal/adapters/sftpclient"
	"ec2-shell-desk/internal/adapters/sqlite"
	"ec2-shell-desk/internal/adapters/sshclient"
	"ec2-shell-desk/internal/adapters/winfs"
	"ec2-shell-desk/internal/app"
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

func generateTestPEM(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "handler-test")
	if err != nil {
		t.Fatalf("MarshalPrivateKey: %v", err)
	}
	return string(pem.EncodeToMemory(block))
}

func TestHandlersAuthVaultHostAndLocalFS(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	store, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	vc, err := crypto.NewVaultCrypto(crypto.Argon2Params{
		Time:    1,
		Memory:  1024,
		Threads: 1,
		KeyLen:  32,
		SaltLen: 16,
	})
	if err != nil {
		t.Fatalf("NewVaultCrypto: %v", err)
	}

	userRepo := sqlite.NewUserRepository(store)
	keyRepo := sqlite.NewKeyRepository(store)
	hostRepo := sqlite.NewHostRepository(store)
	knownRepo := sqlite.NewKnownHostRepository(store)
	groupRepo := sqlite.NewGroupRepository(store)
	tunnelRepo := sqlite.NewTunnelRepository(store)
	snippetRepo := sqlite.NewSnippetRepository(store)
	serviceRepo := sqlite.NewServiceRepository(store)

	bridge := app.NewEventBridge()
	sshMgr := sshclient.NewManager(sftpclient.NewFromSSHClient)
	authSvc := services.NewAuthService(userRepo, vc, vc)
	vaultSvc := services.NewVaultService(authSvc, keyRepo, vc, vc)
	hostSvc := services.NewHostService(authSvc, hostRepo, keyRepo, groupRepo, tunnelRepo)
	sessionSvc := services.NewSessionService(authSvc, hostRepo, knownRepo, vaultSvc, sshMgr, bridge)
	transferSvc := services.NewTransferService(authSvc, sessionSvc, winfs.NewWinFS(), bridge)
	tunnelSvc := services.NewTunnelService(authSvc, hostRepo, knownRepo, tunnelRepo, vaultSvc, sshMgr, bridge)
	snippetSvc := services.NewSnippetService(authSvc, snippetRepo, hostRepo, groupRepo, sessionSvc)
	obsSvc := services.NewObservabilityService(authSvc, hostRepo, knownRepo, serviceRepo, vaultSvc, sshMgr, bridge)

	authH := app.NewAuthHandler(bridge, authSvc, sessionSvc, transferSvc, tunnelSvc, obsSvc)
	keyH := app.NewKeyHandler(bridge, vaultSvc)
	hostH := app.NewHostHandler(bridge, hostSvc)
	sftpH := app.NewSFTPHandler(bridge, transferSvc)
	tunnelH := app.NewTunnelHandler(bridge, tunnelSvc)
	snippetH := app.NewSnippetHandler(bridge, snippetSvc)
	obsH := app.NewObservabilityHandler(bridge, obsSvc)

	status, err := authH.Status()
	if err != nil || status.HasUsers || status.Unlocked {
		t.Fatalf("initial Status() = (%+v, %v)", status, err)
	}

	user, err := authH.BootstrapInitialUser("admin", "AdminPass!2026")
	if err != nil || user.Username != "admin" {
		t.Fatalf("BootstrapInitialUser() = (%+v, %v)", user, err)
	}

	status, err = authH.Status()
	if err != nil || !status.HasUsers || !status.Unlocked || status.User == nil {
		t.Fatalf("unlocked Status() = (%+v, %v)", status, err)
	}

	pemPath := filepath.Join(t.TempDir(), "test.pem")
	if err := os.WriteFile(pemPath, []byte(generateTestPEM(t)), 0o600); err != nil {
		t.Fatalf("write test pem: %v", err)
	}

	keySummary, err := keyH.ImportKeyFromFile("ec2-key.pem", pemPath, "")
	if err != nil {
		t.Fatalf("ImportKeyFromFile() error = %v", err)
	}

	group, err := hostH.CreateGroup("Trabajo")
	if err != nil || group.Name != "Trabajo" {
		t.Fatalf("CreateGroup() = (%+v, %v)", group, err)
	}

	host, err := hostH.CreateHost(services.SaveHostInput{
		GroupID:     group.ID,
		Alias:       "Web Prod",
		HostAddress: "54.1.2.3",
		SSHUsername: "ec2-user",
		SSHKeyID:    keySummary.ID,
		Environment: domain.EnvProd,
	})
	if err != nil {
		t.Fatalf("CreateHost() error = %v", err)
	}

	hosts, err := hostH.ListHosts()
	if err != nil || len(hosts) != 1 || hosts[0].ID != host.ID || hosts[0].GroupID != group.ID {
		t.Fatalf("ListHosts() = (%+v, %v)", hosts, err)
	}

	tun, err := tunnelH.CreateTunnel(services.SaveTunnelInput{
		HostID:     host.ID,
		Name:       "RDS Postgres",
		Preset:     domain.TunnelPresetPostgreSQL,
		LocalHost:  domain.DefaultTunnelLocalHost,
		LocalPort:  15432,
		RemoteHost: "rds.internal",
		RemotePort: 5432,
	})
	if err != nil || tun.LocalPort != 15432 {
		t.Fatalf("CreateTunnel() = (%+v, %v)", tun, err)
	}
	tunnels, err := tunnelH.ListTunnels()
	if err != nil || len(tunnels) != 1 {
		t.Fatalf("ListTunnels() = (%+v, %v)", tunnels, err)
	}

	snip, err := snippetH.CreateSnippet(services.SaveSnippetInput{
		HostID:      host.ID,
		GroupID:     group.ID,
		Name:        "Check Disk",
		Command:     "df -h",
		AutoExecute: true,
	})
	if err != nil || snip.Name != "Check Disk" || snip.HostID != host.ID {
		t.Fatalf("CreateSnippet() = (%+v, %v)", snip, err)
	}
	snips, err := snippetH.ListSnippets()
	if err != nil || len(snips) != 1 {
		t.Fatalf("ListSnippets() = (%+v, %v)", snips, err)
	}

	createdSvc, err := obsH.CreateService(services.SaveServiceInput{
		HostID:     host.ID,
		Name:       "Grafana Metrics",
		Preset:     domain.ServicePresetGrafana,
		Protocol:   domain.ServiceProtocolHTTP,
		RemoteHost: "127.0.0.1",
		RemotePort: 3001,
		BasePath:   "/grafana",
		HealthPath: "/api/health",
		OpenMode:   domain.ServiceOpenModeBrowser,
	})
	if err != nil || createdSvc.Name != "Grafana Metrics" {
		t.Fatalf("CreateService() = (%+v, %v)", createdSvc, err)
	}
	svcs, err := obsH.ListServices()
	if err != nil || len(svcs) != 1 {
		t.Fatalf("ListServices() = (%+v, %v)", svcs, err)
	}
	byHostSvcs, err := obsH.ListServicesByHost(host.ID)
	if err != nil || len(byHostSvcs) != 1 {
		t.Fatalf("ListServicesByHost() = (%+v, %v)", byHostSvcs, err)
	}

	roots, err := sftpH.ListLocalRoots()
	if err != nil || len(roots) == 0 {
		t.Fatalf("ListLocalRoots() = (%v, %v)", roots, err)
	}

	authH.Lock()
	lockedStatus, err := authH.Status()
	if err != nil || !lockedStatus.HasUsers || lockedStatus.Unlocked {
		t.Fatalf("locked Status() = (%+v, %v)", lockedStatus, err)
	}
}
