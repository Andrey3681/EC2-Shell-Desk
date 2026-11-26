package services

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const (
	httpStatusBadRequest   = 400
	httpStatusUnauthorized = 401
)

// SaveServiceInput captures fields required to create or update a private web service.
type SaveServiceInput struct {
	ID               string                 `json:"id,omitempty"`
	HostID           string                 `json:"hostId"`
	Name             string                 `json:"name"`
	Preset           domain.ServicePreset   `json:"preset"`
	Protocol         domain.ServiceProtocol `json:"protocol"`
	RemoteHost       string                 `json:"remoteHost"`
	RemotePort       int                    `json:"remotePort"`
	BasePath         string                 `json:"basePath"`
	LocalPort        int                    `json:"localPort"`
	HealthPath       string                 `json:"healthPath"`
	OpenMode         domain.ServiceOpenMode `json:"openMode"`
	AllowInsecureTLS bool                   `json:"allowInsecureTls"`
	Notes            string                 `json:"notes"`
}

// ServiceEventSink notifies the UI about runtime changes to a service's tunnel and health status.
type ServiceEventSink interface {
	OnServiceStatusChanged(status domain.ServiceRuntimeStatus)
}

type runningService struct {
	service   domain.PrivateService
	handle    ports.TunnelHandle
	localPort int
	status    domain.ServiceRuntimeStatus
}

// ObservabilityService manages saved private web services and their local loopback SSH tunnels.
type ObservabilityService struct {
	auth       *AuthService
	hosts      ports.HostRepository
	knownHosts ports.KnownHostRepository
	services   ports.ServiceRepository
	vault      *VaultService
	sshManager ports.SSHSessionManager
	prober     ports.HealthProber
	sink       ServiceEventSink

	mu     sync.RWMutex
	active map[string]*runningService
}

// NewObservabilityService constructs an ObservabilityService.
func NewObservabilityService(
	auth *AuthService,
	hosts ports.HostRepository,
	knownHosts ports.KnownHostRepository,
	services ports.ServiceRepository,
	vault *VaultService,
	sshManager ports.SSHSessionManager,
	prober ports.HealthProber,
	sink ServiceEventSink,
) *ObservabilityService {
	return &ObservabilityService{
		auth:       auth,
		hosts:      hosts,
		knownHosts: knownHosts,
		services:   services,
		vault:      vault,
		sshManager: sshManager,
		prober:     prober,
		sink:       sink,
		active:     make(map[string]*runningService),
	}
}

// CreateService validates and persists a new PrivateService scoped to the current user.
func (s *ObservabilityService) CreateService(ctx context.Context, input SaveServiceInput) (domain.PrivateService, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.PrivateService{}, err
	}

	svc := normalizeServiceInput(input)
	svc.ID = uuid.NewString()
	svc.OwnerUserID = user.ID
	now := time.Now().UTC()
	svc.CreatedAt = now
	svc.UpdatedAt = now

	if err := svc.Validate(); err != nil {
		return domain.PrivateService{}, fmt.Errorf("validate private service: %w", err)
	}
	if _, err := s.hosts.GetByID(ctx, user.ID, svc.HostID); err != nil {
		return domain.PrivateService{}, fmt.Errorf("verify host ownership: %w", err)
	}

	if err := s.services.Create(ctx, svc); err != nil {
		return domain.PrivateService{}, fmt.Errorf("create private service: %w", err)
	}
	return svc, nil
}

// ListServices returns all saved services owned by the active user.
func (s *ObservabilityService) ListServices(ctx context.Context) ([]domain.PrivateService, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}
	list, err := s.services.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list private services: %w", err)
	}
	return list, nil
}

// ListServicesByHost returns saved services for a specific host owned by the active user.
func (s *ObservabilityService) ListServicesByHost(ctx context.Context, hostID string) ([]domain.PrivateService, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}
	list, err := s.services.ListByHost(ctx, user.ID, hostID)
	if err != nil {
		return nil, fmt.Errorf("list host private services: %w", err)
	}
	return list, nil
}

// UpdateService modifies an existing private service definition.
func (s *ObservabilityService) UpdateService(ctx context.Context, input SaveServiceInput) (domain.PrivateService, error) {
	if strings.TrimSpace(input.ID) == "" {
		return domain.PrivateService{}, fmt.Errorf("%w: service id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.PrivateService{}, err
	}

	if s.isServiceActive(user.ID, input.ID) {
		return domain.PrivateService{}, fmt.Errorf("%w: stop the service before editing", domain.ErrTunnelAlreadyActive)
	}

	existing, err := s.services.GetByID(ctx, user.ID, input.ID)
	if err != nil {
		return domain.PrivateService{}, fmt.Errorf("lookup private service: %w", err)
	}

	updated := normalizeServiceInput(input)
	updated.ID = existing.ID
	updated.OwnerUserID = user.ID
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = time.Now().UTC()

	if err := updated.Validate(); err != nil {
		return domain.PrivateService{}, fmt.Errorf("validate updated service: %w", err)
	}
	if _, err := s.hosts.GetByID(ctx, user.ID, updated.HostID); err != nil {
		return domain.PrivateService{}, fmt.Errorf("verify host ownership: %w", err)
	}

	if err := s.services.Update(ctx, updated); err != nil {
		return domain.PrivateService{}, fmt.Errorf("update private service: %w", err)
	}
	return updated, nil
}

// DeleteService removes a stopped private service definition.
func (s *ObservabilityService) DeleteService(ctx context.Context, serviceID string) error {
	if strings.TrimSpace(serviceID) == "" {
		return fmt.Errorf("%w: service id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	if s.isServiceActive(user.ID, serviceID) {
		return fmt.Errorf("%w: stop the service before deleting", domain.ErrTunnelAlreadyActive)
	}

	if err := s.services.Delete(ctx, user.ID, serviceID); err != nil {
		return fmt.Errorf("delete private service: %w", err)
	}
	return nil
}

// StartService starts the local loopback SSH tunnel for the service, checks its health, and emits status.
func (s *ObservabilityService) StartService(
	ctx context.Context,
	serviceID string,
	passwordFallback string,
	tofuConfirm TOFUTrustPromptFunc,
) (domain.ServiceRuntimeStatus, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.ServiceRuntimeStatus{}, err
	}

	svc, err := s.services.GetByID(ctx, user.ID, serviceID)
	if err != nil {
		return domain.ServiceRuntimeStatus{}, fmt.Errorf("lookup private service: %w", err)
	}

	s.mu.Lock()
	if existing, ok := s.active[svc.ID]; ok && existing.status.OwnerUserID == user.ID {
		s.mu.Unlock()
		return existing.status, nil
	}
	initialStatus := domain.ServiceRuntimeStatus{
		ServiceID:    svc.ID,
		OwnerUserID:  user.ID,
		State:        domain.TunnelStateConnecting,
		HealthStatus: domain.ServiceHealthUnknown,
	}
	s.mu.Unlock()
	s.emitStatus(initialStatus)

	host, err := s.hosts.GetByID(ctx, user.ID, svc.HostID)
	if err != nil {
		return s.failStatus(svc.ID, user.ID, fmt.Sprintf("lookup host: %v", err))
	}

	var rawPEM, passphrase []byte
	if host.SSHKeyID != "" {
		rawPEM, passphrase, err = s.vault.DecryptKeyForSession(ctx, host.SSHKeyID)
		if err != nil {
			return s.failStatus(svc.ID, user.ID, fmt.Sprintf("unlock host key: %v", err))
		}
		defer wipeBytes(rawPEM)
		defer wipeBytes(passphrase)
	}

	verifier := newTOFUHostKeyVerifier(s.knownHosts, user.ID, tofuConfirm)

	handle, err := s.sshManager.StartLocalForward(ctx, ports.TunnelForwardRequest{
		Host:          host,
		PrivateKeyPEM: rawPEM,
		Passphrase:    passphrase,
		Password:      passwordFallback,
		LocalHost:     "127.0.0.1",
		LocalPort:     svc.LocalPort,
		RemoteHost:    svc.RemoteHost,
		RemotePort:    svc.RemotePort,
		VerifyHostKey: verifier,
	})
	if err != nil {
		return s.failStatus(svc.ID, user.ID, err.Error())
	}

	localPort := extractPortFromAddr(handle.LocalAddr())
	localURL := svc.URL(localPort)
	now := time.Now().UTC()

	rs := &runningService{
		service:   svc,
		handle:    handle,
		localPort: localPort,
		status: domain.ServiceRuntimeStatus{
			ServiceID:    svc.ID,
			OwnerUserID:  user.ID,
			State:        domain.TunnelStateActive,
			LocalURL:     localURL,
			LocalPort:    localPort,
			HealthStatus: domain.ServiceHealthUnknown,
			LastChecked:  &now,
		},
	}

	s.mu.Lock()
	s.active[svc.ID] = rs
	s.mu.Unlock()

	s.emitStatus(rs.status)
	go s.watchTunnelExit(svc.ID, handle)

	// Perform initial health probe over the newly established loopback tunnel.
	status, probeErr := s.ProbeServiceHealth(ctx, svc.ID)
	if probeErr != nil {
		return rs.status, nil
	}
	return status, nil
}

// StopService closes the local tunnel for the given service.
func (s *ObservabilityService) StopService(serviceID string) error {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	s.mu.Lock()
	rs, ok := s.active[serviceID]
	if !ok || rs.status.OwnerUserID != user.ID {
		s.mu.Unlock()
		return nil
	}
	delete(s.active, serviceID)
	s.mu.Unlock()

	_ = rs.handle.Close()
	status := domain.ServiceRuntimeStatus{
		ServiceID:    serviceID,
		OwnerUserID:  user.ID,
		State:        domain.TunnelStateStopped,
		HealthStatus: domain.ServiceHealthUnknown,
	}
	s.emitStatus(status)
	return nil
}

// ProbeServiceHealth sends an HTTP GET probe to an owned service's health path through its active tunnel.
func (s *ObservabilityService) ProbeServiceHealth(ctx context.Context, serviceID string) (domain.ServiceRuntimeStatus, error) {
	trimmedID := strings.TrimSpace(serviceID)
	if trimmedID == "" {
		return domain.ServiceRuntimeStatus{}, fmt.Errorf("%w: service id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.ServiceRuntimeStatus{}, err
	}

	if _, err := s.services.GetByID(ctx, user.ID, trimmedID); err != nil {
		return domain.ServiceRuntimeStatus{}, fmt.Errorf("lookup private service: %w", err)
	}

	s.mu.RLock()
	rs, ok := s.active[trimmedID]
	if !ok || rs.handle == nil || rs.status.OwnerUserID != user.ID {
		s.mu.RUnlock()
		return domain.ServiceRuntimeStatus{
			ServiceID:    trimmedID,
			OwnerUserID:  user.ID,
			State:        domain.TunnelStateStopped,
			HealthStatus: domain.ServiceHealthUnknown,
		}, nil
	}
	svc := rs.service
	localPort := rs.localPort
	s.mu.RUnlock()

	proto := string(svc.Protocol)
	if proto == "" {
		proto = string(domain.ServiceProtocolHTTP)
	}
	probeURL := fmt.Sprintf("%s://127.0.0.1:%d%s", proto, localPort, svc.NormalizedHealthPath())
	now := time.Now().UTC()

	if s.prober == nil {
		return s.updateHealth(trimmedID, user.ID, domain.ServiceHealthUnhealthy, 0, 0, "health prober is not configured", now), nil
	}

	probeRes, probeErr := s.prober.Probe(ctx, ports.HealthProbeRequest{
		URL:              probeURL,
		AllowInsecureTLS: svc.AllowInsecureTLS,
	})
	if probeErr != nil {
		return s.updateHealth(trimmedID, user.ID, domain.ServiceHealthUnhealthy, 0, probeRes.LatencyMs, probeErr.Error(), now), nil
	}

	healthStatus := domain.ServiceHealthHealthy
	if probeRes.HTTPStatus >= httpStatusBadRequest && probeRes.HTTPStatus != httpStatusUnauthorized {
		// 401 Unauthorized is common for Grafana when unauthenticated, but indicates the service is up.
		healthStatus = domain.ServiceHealthDegraded
	}

	return s.updateHealth(trimmedID, user.ID, healthStatus, probeRes.HTTPStatus, probeRes.LatencyMs, "", now), nil
}

func (s *ObservabilityService) updateHealth(
	serviceID string,
	ownerUserID string,
	health domain.ServiceHealthStatus,
	httpStatus int,
	latencyMs int64,
	errMsg string,
	checked time.Time,
) domain.ServiceRuntimeStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	rs, ok := s.active[serviceID]
	if !ok || rs.status.OwnerUserID != ownerUserID {
		return domain.ServiceRuntimeStatus{
			ServiceID:    serviceID,
			OwnerUserID:  ownerUserID,
			State:        domain.TunnelStateStopped,
			HealthStatus: health,
		}
	}

	rs.status.HealthStatus = health
	rs.status.HTTPStatus = httpStatus
	rs.status.LatencyMs = latencyMs
	rs.status.ErrorMessage = errMsg
	rs.status.LastChecked = &checked

	snapshot := rs.status
	s.emitStatus(snapshot)
	return snapshot
}

// ListStatuses returns runtime status snapshots for all active services owned by current user.
func (s *ObservabilityService) ListStatuses() ([]domain.ServiceRuntimeStatus, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]domain.ServiceRuntimeStatus, 0)
	for _, rs := range s.active {
		if rs.status.OwnerUserID == user.ID {
			list = append(list, rs.status)
		}
	}
	return list, nil
}

// CloseAll stops all active tunnels during application shutdown or user vault lock.
func (s *ObservabilityService) CloseAll() {
	s.mu.Lock()
	toClose := make([]ports.TunnelHandle, 0, len(s.active))
	for id, rs := range s.active {
		toClose = append(toClose, rs.handle)
		delete(s.active, id)
	}
	s.mu.Unlock()

	for _, h := range toClose {
		_ = h.Close()
	}
}

func (s *ObservabilityService) isServiceActive(ownerUserID, serviceID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rs, ok := s.active[serviceID]
	return ok && rs.status.OwnerUserID == ownerUserID && rs.status.State == domain.TunnelStateActive
}

func (s *ObservabilityService) failStatus(serviceID, ownerUserID, errMsg string) (domain.ServiceRuntimeStatus, error) {
	status := domain.ServiceRuntimeStatus{
		ServiceID:    serviceID,
		OwnerUserID:  ownerUserID,
		State:        domain.TunnelStateError,
		HealthStatus: domain.ServiceHealthUnhealthy,
		ErrorMessage: errMsg,
	}
	s.emitStatus(status)
	return status, fmt.Errorf("start private service: %s", errMsg)
}

func (s *ObservabilityService) emitStatus(status domain.ServiceRuntimeStatus) {
	if s.sink != nil {
		s.sink.OnServiceStatusChanged(status)
	}
}

func (s *ObservabilityService) watchTunnelExit(serviceID string, handle ports.TunnelHandle) {
	<-handle.Done()
	s.mu.Lock()
	rs, ok := s.active[serviceID]
	if !ok || rs.handle != handle {
		s.mu.Unlock()
		return
	}
	delete(s.active, serviceID)
	user := rs.status.OwnerUserID
	s.mu.Unlock()

	err := handle.Err()
	status := domain.ServiceRuntimeStatus{
		ServiceID:    serviceID,
		OwnerUserID:  user,
		State:        domain.TunnelStateStopped,
		HealthStatus: domain.ServiceHealthUnknown,
	}
	if err != nil {
		status.State = domain.TunnelStateError
		status.ErrorMessage = err.Error()
	}
	s.emitStatus(status)
}

func normalizeServiceInput(input SaveServiceInput) domain.PrivateService {
	protocol := input.Protocol
	if protocol == "" {
		protocol = domain.ServiceProtocolHTTP
	}
	openMode := input.OpenMode
	if openMode == "" {
		openMode = domain.ServiceOpenModeBrowser
	}
	remoteHost := strings.TrimSpace(input.RemoteHost)
	if remoteHost == "" {
		remoteHost = "127.0.0.1"
	}
	preset := input.Preset
	if preset == "" {
		preset = domain.ServicePresetCustom
	}

	return domain.PrivateService{
		ID:               strings.TrimSpace(input.ID),
		HostID:           strings.TrimSpace(input.HostID),
		Name:             strings.TrimSpace(input.Name),
		Preset:           preset,
		Protocol:         protocol,
		RemoteHost:       remoteHost,
		RemotePort:       input.RemotePort,
		BasePath:         strings.TrimSpace(input.BasePath),
		LocalPort:        input.LocalPort,
		HealthPath:       strings.TrimSpace(input.HealthPath),
		OpenMode:         openMode,
		AllowInsecureTLS: protocol == domain.ServiceProtocolHTTPS && input.AllowInsecureTLS,
		Notes:            strings.TrimSpace(input.Notes),
	}
}

func extractPortFromAddr(addr string) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(portStr)
	return p
}
