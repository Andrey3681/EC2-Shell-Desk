package services_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"ec2-shell-desk/internal/adapters/sqlite"
	"ec2-shell-desk/internal/adapters/sshclient"
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
	"ec2-shell-desk/internal/services"
)

type spyTunnelSink struct {
	mu     sync.Mutex
	events []domain.TunnelRuntimeStatus
}

func (s *spyTunnelSink) OnTunnelStatusChanged(status domain.TunnelRuntimeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, status)
}

func (s *spyTunnelSink) LastState(tunnelID string) (domain.TunnelState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].TunnelID == tunnelID {
			return s.events[i].State, true
		}
	}
	return "", false
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestTunnelServiceEndToEndAndMultiUserIsolation(t *testing.T) {
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
	tunnelRepo := sqlite.NewTunnelRepository(store)

	authSvc := services.NewAuthService(userRepo, vc, vc)
	vaultSvc := services.NewVaultService(authSvc, keyRepo, vc, vc)
	hostSvc := services.NewHostService(authSvc, hostRepo, keyRepo, nil, tunnelRepo, nil)
	tunnelSink := &spyTunnelSink{}

	sshMgr := sshclient.NewManager(nil, sshclient.WithDialTimeout(5*time.Second))
	tunnelSvc := services.NewTunnelService(authSvc, hostRepo, knownRepo, tunnelRepo, vaultSvc, sshMgr, tunnelSink)
	t.Cleanup(tunnelSvc.CloseAll)

	// Start mock private DB server (simulates RDS / MongoDB behind EC2).
	dbLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock db: %v", err)
	}
	t.Cleanup(func() { _ = dbLn.Close() })
	go func() {
		for {
			c, aErr := dbLn.Accept()
			if aErr != nil {
				return
			}
			go func(conn net.Conn) {
				defer func() { _ = conn.Close() }()
				_, _ = io.Copy(conn, conn)
			}(c)
		}
	}()
	dbAddr := dbLn.Addr().(*net.TCPAddr)

	rawPEM, clientPub := generateClientPEMAndPub(t)
	sshSrv := startFullTestSSHServer(t, clientPub, t.TempDir())

	if _, err := authSvc.BootstrapInitialUser(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("BootstrapInitialUser: %v", err)
	}
	if _, err := authSvc.CreateUser(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("CreateUser(bob): %v", err)
	}

	keySummary, err := vaultSvc.ImportKey(ctx, "bastion.pem", rawPEM, nil)
	if err != nil {
		t.Fatalf("ImportKey: %v", err)
	}
	bastion, err := hostSvc.CreateHost(ctx, services.SaveHostInput{
		Alias:       "AWS Bastion",
		HostAddress: sshSrv.host,
		Port:        sshSrv.port,
		SSHUsername: "ec2-user",
		SSHKeyID:    keySummary.ID,
		Environment: domain.EnvProd,
	})
	if err != nil {
		t.Fatalf("CreateHost: %v", err)
	}

	localPort := freeLocalPort(t)
	tun, err := tunnelSvc.CreateTunnel(ctx, services.SaveTunnelInput{
		HostID:     bastion.ID,
		Name:       "RDS Prod Postgres",
		Preset:     domain.TunnelPresetPostgreSQL,
		LocalHost:  domain.DefaultTunnelLocalHost,
		LocalPort:  localPort,
		RemoteHost: dbAddr.IP.String(),
		RemotePort: dbAddr.Port,
		Notes:      "pgAdmin local connection",
	})
	if err != nil {
		t.Fatalf("CreateTunnel: %v", err)
	}

	// Host referenced by tunnel cannot be deleted.
	if err := hostSvc.DeleteHost(ctx, bastion.ID); !errors.Is(err, domain.ErrHostInUse) {
		t.Fatalf("DeleteHost(in use) = %v, want ErrHostInUse", err)
	}

	// Start tunnel with TOFU acceptance.
	status, err := tunnelSvc.StartTunnel(ctx, tun.ID, "", func(_ context.Context, _ ports.HostKeyChallenge) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatalf("StartTunnel: %v", err)
	}
	if status.State != domain.TunnelStateActive {
		t.Fatalf("StartTunnel state = %q, want active", status.State)
	}

	// Cannot edit or delete an active tunnel.
	if err := tunnelSvc.DeleteTunnel(ctx, tun.ID); !errors.Is(err, domain.ErrTunnelAlreadyActive) {
		t.Fatalf("DeleteTunnel(active) = %v, want ErrTunnelAlreadyActive", err)
	}

	// Forward real bytes through 127.0.0.1:<localPort>.
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), 2*time.Second)
	if err != nil {
		t.Fatalf("dial local tunnel: %v", err)
	}
	payload := []byte("POSTGRES-HANDSHAKE-PING")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write to tunnel: %v", err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read from tunnel: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("echo mismatch: got %q, want %q", got, payload)
	}
	_ = conn.Close()

	// Multi-user isolation: Bob cannot see, start, or stop Alice's tunnel.
	if _, err := authSvc.Login(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("Login(bob): %v", err)
	}
	if bobTunnels, err := tunnelSvc.ListTunnels(ctx); err != nil || len(bobTunnels) != 0 {
		t.Fatalf("Bob ListTunnels = (%d, %v), want (0, nil)", len(bobTunnels), err)
	}
	if err := tunnelSvc.StopTunnel(tun.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob StopTunnel = %v, want ErrNotFound", err)
	}
	if _, err := tunnelSvc.CreateTunnel(ctx, services.SaveTunnelInput{
		HostID:     bastion.ID,
		Name:       "Hijack Bastion",
		LocalPort:  freeLocalPort(t),
		RemoteHost: "127.0.0.1",
		RemotePort: 5432,
	}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob CreateTunnel with Alice's host = %v, want ErrNotFound", err)
	}

	// Switch back to Alice, stop tunnel, delete tunnel, then delete bastion host.
	if _, err := authSvc.Login(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("Login(alice): %v", err)
	}
	if err := tunnelSvc.StopTunnel(tun.ID); err != nil {
		t.Fatalf("Alice StopTunnel: %v", err)
	}
	if st, ok := tunnelSink.LastState(tun.ID); !ok || st != domain.TunnelStateStopped {
		t.Fatalf("LastState = (%q, %v), want stopped", st, ok)
	}
	if err := tunnelSvc.DeleteTunnel(ctx, tun.ID); err != nil {
		t.Fatalf("Alice DeleteTunnel: %v", err)
	}
	if err := hostSvc.DeleteHost(ctx, bastion.ID); err != nil {
		t.Fatalf("Alice DeleteHost: %v", err)
	}
}
