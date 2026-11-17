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

const tunnelTrafficEmitInterval = 150 * time.Millisecond

// SaveTunnelInput captures fields required to create or update an SSH port-forwarding tunnel.
type SaveTunnelInput struct {
	ID         string              `json:"id,omitempty"`
	HostID     string              `json:"hostId"`
	Name       string              `json:"name"`
	Preset     domain.TunnelPreset `json:"preset"`
	LocalHost  string              `json:"localHost"`
	LocalPort  int                 `json:"localPort"`
	RemoteHost string              `json:"remoteHost"`
	RemotePort int                 `json:"remotePort"`
	Notes      string              `json:"notes"`
}

// TunnelEventSink receives live tunnel status and traffic updates for the desktop UI.
type TunnelEventSink interface {
	OnTunnelStatusChanged(status domain.TunnelRuntimeStatus)
}

type runningTunnel struct {
	tunnel domain.SSHTunnel
	handle ports.TunnelHandle
	status domain.TunnelRuntimeStatus
}

// TunnelService manages user-isolated SSH local port-forwarding tunnels for databases and internal endpoints.
type TunnelService struct {
	auth       *AuthService
	hosts      ports.HostRepository
	knownHosts ports.KnownHostRepository
	tunnels    ports.TunnelRepository
	vault      *VaultService
	sshManager ports.SSHSessionManager
	sink       TunnelEventSink

	mu     sync.RWMutex
	active map[string]*runningTunnel
}

// NewTunnelService constructs a TunnelService orchestrating saved SSH port forwards for the active user.
func NewTunnelService(
	auth *AuthService,
	hosts ports.HostRepository,
	knownHosts ports.KnownHostRepository,
	tunnels ports.TunnelRepository,
	vault *VaultService,
	sshManager ports.SSHSessionManager,
	sink TunnelEventSink,
) *TunnelService {
	return &TunnelService{
		auth:       auth,
		hosts:      hosts,
		knownHosts: knownHosts,
		tunnels:    tunnels,
		vault:      vault,
		sshManager: sshManager,
		sink:       sink,
		active:     make(map[string]*runningTunnel),
	}
}

// CreateTunnel validates and persists a new SSHTunnel linked to a user-owned HostInstance bastion.
func (s *TunnelService) CreateTunnel(ctx context.Context, input SaveTunnelInput) (domain.SSHTunnel, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.SSHTunnel{}, err
	}

	tun := normalizeTunnelInput(input)
	tun.ID = uuid.NewString()
	tun.OwnerUserID = user.ID
	now := time.Now().UTC()
	tun.CreatedAt = now
	tun.UpdatedAt = now

	if err := tun.Validate(); err != nil {
		return domain.SSHTunnel{}, fmt.Errorf("validate ssh tunnel: %w", err)
	}
	if _, err := s.hosts.GetByID(ctx, user.ID, tun.HostID); err != nil {
		return domain.SSHTunnel{}, fmt.Errorf("verify bastion host ownership: %w", err)
	}

	if err := s.tunnels.Create(ctx, tun); err != nil {
		return domain.SSHTunnel{}, fmt.Errorf("create ssh tunnel: %w", err)
	}
	return tun, nil
}

// ListTunnels returns all saved SSHTunnel definitions owned exclusively by the authenticated user.
func (s *TunnelService) ListTunnels(ctx context.Context) ([]domain.SSHTunnel, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}
	list, err := s.tunnels.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list ssh tunnels: %w", err)
	}
	return list, nil
}

// UpdateTunnel updates a stopped SSHTunnel definition owned by the authenticated user.
func (s *TunnelService) UpdateTunnel(ctx context.Context, input SaveTunnelInput) (domain.SSHTunnel, error) {
	if strings.TrimSpace(input.ID) == "" {
		return domain.SSHTunnel{}, fmt.Errorf("%w: tunnel id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.SSHTunnel{}, err
	}

	if s.isTunnelActive(user.ID, input.ID) {
		return domain.SSHTunnel{}, fmt.Errorf("%w: stop the tunnel before editing", domain.ErrTunnelAlreadyActive)
	}

	existing, err := s.tunnels.GetByID(ctx, user.ID, input.ID)
	if err != nil {
		return domain.SSHTunnel{}, fmt.Errorf("lookup ssh tunnel: %w", err)
	}

	updated := normalizeTunnelInput(input)
	updated.ID = existing.ID
	updated.OwnerUserID = user.ID
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = time.Now().UTC()

	if err := updated.Validate(); err != nil {
		return domain.SSHTunnel{}, fmt.Errorf("validate updated ssh tunnel: %w", err)
	}
	if _, err := s.hosts.GetByID(ctx, user.ID, updated.HostID); err != nil {
		return domain.SSHTunnel{}, fmt.Errorf("verify bastion host ownership: %w", err)
	}

	if err := s.tunnels.Update(ctx, updated); err != nil {
		return domain.SSHTunnel{}, fmt.Errorf("update ssh tunnel: %w", err)
	}
	return updated, nil
}

// DeleteTunnel deletes a stopped SSHTunnel owned by the authenticated user.
func (s *TunnelService) DeleteTunnel(ctx context.Context, tunnelID string) error {
	if strings.TrimSpace(tunnelID) == "" {
		return fmt.Errorf("%w: tunnel id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	if s.isTunnelActive(user.ID, tunnelID) {
		return fmt.Errorf("%w: stop the tunnel before deleting", domain.ErrTunnelAlreadyActive)
	}

	if err := s.tunnels.Delete(ctx, user.ID, tunnelID); err != nil {
		return fmt.Errorf("delete ssh tunnel: %w", err)
	}
	return nil
}

// StartTunnel establishes the SSH bastion connection with TOFU verification and opens the local port forwarder.
func (s *TunnelService) StartTunnel(
	ctx context.Context,
	tunnelID string,
	passwordFallback string,
	onTrust TOFUTrustPromptFunc,
) (domain.TunnelRuntimeStatus, error) {
	if strings.TrimSpace(tunnelID) == "" {
		return domain.TunnelRuntimeStatus{}, fmt.Errorf("%w: tunnel id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.TunnelRuntimeStatus{}, err
	}

	tun, err := s.tunnels.GetByID(ctx, user.ID, tunnelID)
	if err != nil {
		return domain.TunnelRuntimeStatus{}, fmt.Errorf("lookup tunnel to start: %w", err)
	}

	if s.isTunnelActive(user.ID, tun.ID) {
		return domain.TunnelRuntimeStatus{}, fmt.Errorf("%w", domain.ErrTunnelAlreadyActive)
	}

	host, err := s.hosts.GetByID(ctx, user.ID, tun.HostID)
	if err != nil {
		return domain.TunnelRuntimeStatus{}, fmt.Errorf("lookup tunnel bastion host: %w", err)
	}

	var rawPEM, passphrase []byte
	if host.SSHKeyID != "" {
		rawPEM, passphrase, err = s.vault.DecryptKeyForSession(ctx, host.SSHKeyID)
		if err != nil {
			return domain.TunnelRuntimeStatus{}, fmt.Errorf("unlock bastion ssh key: %w", err)
		}
		defer wipeBytes(rawPEM)
		defer wipeBytes(passphrase)
	}

	localEndpoint := net.JoinHostPort(tun.LocalHost, strconv.Itoa(tun.LocalPort))
	s.emitStatus(domain.TunnelRuntimeStatus{
		TunnelID:      tun.ID,
		OwnerUserID:   user.ID,
		State:         domain.TunnelStateConnecting,
		LocalEndpoint: localEndpoint,
	})

	var lastEmitMu sync.Mutex
	var lastEmit time.Time

	handle, err := s.sshManager.StartLocalForward(ctx, ports.TunnelForwardRequest{
		Host:          host,
		PrivateKeyPEM: rawPEM,
		Passphrase:    passphrase,
		Password:      passwordFallback,
		LocalHost:     tun.LocalHost,
		LocalPort:     tun.LocalPort,
		RemoteHost:    tun.RemoteHost,
		RemotePort:    tun.RemotePort,
		VerifyHostKey: newTOFUHostKeyVerifier(s.knownHosts, user.ID, onTrust),
		OnTraffic: func(activeConns int, bytesIn, bytesOut int64) {
			s.mu.Lock()
			rt, ok := s.active[tun.ID]
			var snapshot domain.TunnelRuntimeStatus
			shouldEmit := false
			if ok {
				rt.status.ActiveConnections = activeConns
				rt.status.BytesIn = bytesIn
				rt.status.BytesOut = bytesOut
				snapshot = rt.status

				lastEmitMu.Lock()
				now := time.Now()
				if lastEmit.IsZero() || now.Sub(lastEmit) >= tunnelTrafficEmitInterval || activeConns == 0 {
					lastEmit = now
					shouldEmit = true
				}
				lastEmitMu.Unlock()
			}
			s.mu.Unlock()

			if shouldEmit {
				s.emitStatus(snapshot)
			}
		},
	})
	if err != nil {
		errStatus := domain.TunnelRuntimeStatus{
			TunnelID:      tun.ID,
			OwnerUserID:   user.ID,
			State:         domain.TunnelStateError,
			LocalEndpoint: localEndpoint,
			ErrorMessage:  err.Error(),
		}
		s.emitStatus(errStatus)
		return domain.TunnelRuntimeStatus{}, fmt.Errorf("start local port forward: %w", err)
	}

	startedAt := time.Now().UTC()
	status := domain.TunnelRuntimeStatus{
		TunnelID:      tun.ID,
		OwnerUserID:   user.ID,
		State:         domain.TunnelStateActive,
		LocalEndpoint: handle.LocalAddr(),
		StartedAt:     &startedAt,
	}

	s.mu.Lock()
	s.active[tun.ID] = &runningTunnel{
		tunnel: tun,
		handle: handle,
		status: status,
	}
	s.mu.Unlock()

	s.emitStatus(status)
	go s.watchTunnelExit(tun.ID, user.ID, localEndpoint, handle)
	return status, nil
}

// StopTunnel stops an active SSH tunnel owned by the authenticated user.
func (s *TunnelService) StopTunnel(tunnelID string) error {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	s.mu.Lock()
	rt, ok := s.active[tunnelID]
	if !ok || rt.tunnel.OwnerUserID != user.ID {
		s.mu.Unlock()
		return fmt.Errorf("%w: active ssh tunnel", domain.ErrNotFound)
	}
	delete(s.active, tunnelID)
	localEndpoint := rt.status.LocalEndpoint
	s.mu.Unlock()

	closeErr := rt.handle.Close()
	s.emitStatus(domain.TunnelRuntimeStatus{
		TunnelID:      tunnelID,
		OwnerUserID:   user.ID,
		State:         domain.TunnelStateStopped,
		LocalEndpoint: localEndpoint,
	})
	if closeErr != nil {
		return fmt.Errorf("close ssh tunnel: %w", closeErr)
	}
	return nil
}

// CloseAll stops all active SSH tunnels across the service (typically on lock or shutdown).
func (s *TunnelService) CloseAll() {
	s.mu.Lock()
	items := make([]*runningTunnel, 0, len(s.active))
	for id, rt := range s.active {
		items = append(items, rt)
		delete(s.active, id)
	}
	s.mu.Unlock()

	for _, rt := range items {
		_ = rt.handle.Close()
		s.emitStatus(domain.TunnelRuntimeStatus{
			TunnelID:      rt.tunnel.ID,
			OwnerUserID:   rt.tunnel.OwnerUserID,
			State:         domain.TunnelStateStopped,
			LocalEndpoint: rt.status.LocalEndpoint,
		})
	}
}

// ListStatuses returns live runtime statuses for active tunnels owned by the authenticated user.
func (s *TunnelService) ListStatuses() ([]domain.TunnelRuntimeStatus, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]domain.TunnelRuntimeStatus, 0)
	for _, rt := range s.active {
		if rt.tunnel.OwnerUserID == user.ID {
			statuses = append(statuses, rt.status)
		}
	}
	return statuses, nil
}

func (s *TunnelService) isTunnelActive(ownerUserID, tunnelID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rt, ok := s.active[tunnelID]
	return ok && rt.tunnel.OwnerUserID == ownerUserID
}

func (s *TunnelService) watchTunnelExit(tunnelID, ownerUserID, localEndpoint string, handle ports.TunnelHandle) {
	<-handle.Done()

	s.mu.Lock()
	_, stillTracked := s.active[tunnelID]
	if stillTracked {
		delete(s.active, tunnelID)
	}
	s.mu.Unlock()

	if !stillTracked {
		return
	}

	state := domain.TunnelStateStopped
	errMsg := ""
	if err := handle.Err(); err != nil {
		state = domain.TunnelStateError
		errMsg = err.Error()
	}

	s.emitStatus(domain.TunnelRuntimeStatus{
		TunnelID:      tunnelID,
		OwnerUserID:   ownerUserID,
		State:         state,
		LocalEndpoint: localEndpoint,
		ErrorMessage:  errMsg,
	})
}

func (s *TunnelService) emitStatus(status domain.TunnelRuntimeStatus) {
	if s.sink != nil {
		s.sink.OnTunnelStatusChanged(status)
	}
}

func normalizeTunnelInput(input SaveTunnelInput) domain.SSHTunnel {
	localHost := strings.TrimSpace(input.LocalHost)
	if localHost == "" {
		localHost = domain.DefaultTunnelLocalHost
	}
	preset := input.Preset
	if preset == "" {
		preset = domain.TunnelPresetCustom
	}
	return domain.SSHTunnel{
		ID:         strings.TrimSpace(input.ID),
		HostID:     strings.TrimSpace(input.HostID),
		Name:       strings.TrimSpace(input.Name),
		Preset:     preset,
		LocalHost:  localHost,
		LocalPort:  input.LocalPort,
		RemoteHost: strings.TrimSpace(input.RemoteHost),
		RemotePort: input.RemotePort,
		Notes:      strings.TrimSpace(input.Notes),
	}
}
