package services_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ec2-shell-desk/internal/adapters/sqlite"
	"ec2-shell-desk/internal/adapters/sshclient"
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
	"ec2-shell-desk/internal/services"
)

type spyServiceSink struct {
	mu     sync.Mutex
	events []domain.ServiceRuntimeStatus
}

func (s *spyServiceSink) OnServiceStatusChanged(status domain.ServiceRuntimeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, status)
}

func (s *spyServiceSink) LastState(serviceID string) (domain.TunnelState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].ServiceID == serviceID {
			return s.events[i].State, true
		}
	}
	return "", false
}

func TestObservabilityServiceLifecycleHealthAndIsolation(t *testing.T) {
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
	serviceRepo := sqlite.NewServiceRepository(store)

	authSvc := services.NewAuthService(userRepo, vc, vc)
	vaultSvc := services.NewVaultService(authSvc, keyRepo, vc, vc)
	hostSvc := services.NewHostService(authSvc, hostRepo, keyRepo, groupRepo, nil, serviceRepo)

	sink := &spyServiceSink{}
	sshMgr := sshclient.NewManager(nil, sshclient.WithDialTimeout(5*time.Second))
	obsSvc := services.NewObservabilityService(authSvc, hostRepo, knownRepo, serviceRepo, vaultSvc, sshMgr, sink)
	t.Cleanup(obsSvc.CloseAll)

	if _, err := authSvc.BootstrapInitialUser(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("BootstrapInitialUser: %v", err)
	}
	if _, err := authSvc.CreateUser(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("CreateUser(bob): %v", err)
	}

	rawPEM, clientPub := generateClientPEMAndPub(t)
	srv := startFullTestSSHServer(t, clientPub, t.TempDir())

	keySummary, err := vaultSvc.ImportKey(ctx, "obs-deploy.pem", rawPEM, nil)
	if err != nil {
		t.Fatalf("ImportKey: %v", err)
	}

	host, err := hostSvc.CreateHost(ctx, services.SaveHostInput{
		Alias:       "Observability Host",
		HostAddress: srv.host,
		Port:        srv.port,
		SSHUsername: "ec2-user",
		SSHKeyID:    keySummary.ID,
		Environment: domain.EnvProd,
	})
	if err != nil {
		t.Fatalf("CreateHost: %v", err)
	}

	// Mock web target (e.g. Grafana / Prometheus) running on local interface
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(mockServer.Close)

	mockHost, mockPortStr, err := net.SplitHostPort(mockServer.Listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	var mockPort int
	_, _ = fmt.Sscanf(mockPortStr, "%d", &mockPort)

	// Create PrivateService for Alice
	created, err := obsSvc.CreateService(ctx, services.SaveServiceInput{
		HostID:     host.ID,
		Name:       "Grafana Metrics",
		Preset:     domain.ServicePresetGrafana,
		Protocol:   domain.ServiceProtocolHTTP,
		RemoteHost: mockHost,
		RemotePort: mockPort,
		BasePath:   "/grafana",
		HealthPath: "/api/health",
		OpenMode:   domain.ServiceOpenModeBrowser,
	})
	if err != nil {
		t.Fatalf("CreateService: %v", err)
	}

	tofuVerifier := func(_ context.Context, _ ports.HostKeyChallenge) (bool, error) {
		return true, nil
	}

	// Start service tunnel and test health check
	status, err := obsSvc.StartService(ctx, created.ID, "", tofuVerifier)
	if err != nil {
		t.Fatalf("StartService: %v", err)
	}
	if status.State != domain.TunnelStateActive {
		t.Fatalf("expected state active, got %s", status.State)
	}
	if status.HealthStatus != domain.ServiceHealthHealthy {
		t.Fatalf("expected health status healthy, got %s (err: %s)", status.HealthStatus, status.ErrorMessage)
	}
	if status.HTTPStatus != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", status.HTTPStatus)
	}

	// List statuses
	statuses, err := obsSvc.ListStatuses()
	if err != nil || len(statuses) != 1 {
		t.Fatalf("ListStatuses = (%+v, %v), want 1", statuses, err)
	}

	// Bob isolation checks
	if _, err := authSvc.Login(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("Login(bob): %v", err)
	}
	bobList, err := obsSvc.ListServices(ctx)
	if err != nil || len(bobList) != 0 {
		t.Fatalf("Bob ListServices = (%d, %v), want 0", len(bobList), err)
	}
	if _, err := obsSvc.StartService(ctx, created.ID, "", tofuVerifier); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob StartService = %v, want ErrNotFound", err)
	}
	if err := obsSvc.DeleteService(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob DeleteService = %v, want ErrNotFound", err)
	}

	// Alice logs back in and stops service
	if _, err := authSvc.Login(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("Login(alice): %v", err)
	}
	if err := obsSvc.StopService(created.ID); err != nil {
		t.Fatalf("StopService: %v", err)
	}
	// Deleting the host while a private service still references it must return ErrHostInUse.
	if err := hostSvc.DeleteHost(ctx, host.ID); !errors.Is(err, domain.ErrHostInUse) {
		t.Fatalf("DeleteHost(with private service) = %v, want ErrHostInUse", err)
	}
	if err := obsSvc.DeleteService(ctx, created.ID); err != nil {
		t.Fatalf("DeleteService: %v", err)
	}
	// Once the private service is removed, DeleteHost succeeds cleanly.
	if err := hostSvc.DeleteHost(ctx, host.ID); err != nil {
		t.Fatalf("DeleteHost(after service deleted) error = %v", err)
	}
}
