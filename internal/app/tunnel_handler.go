package app

import (
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

// TunnelHandler exposes SSH local port-forwarding tunnel management bindings to Wails.
type TunnelHandler struct {
	bridge  *EventBridge
	tunnels *services.TunnelService
}

// NewTunnelHandler constructs a TunnelHandler wired to TunnelService.
func NewTunnelHandler(bridge *EventBridge, tunnels *services.TunnelService) *TunnelHandler {
	return &TunnelHandler{
		bridge:  bridge,
		tunnels: tunnels,
	}
}

// ListTunnels returns saved SSH port-forwarding tunnels owned by the active user.
func (h *TunnelHandler) ListTunnels() ([]domain.SSHTunnel, error) {
	return h.tunnels.ListTunnels(h.bridge.Context())
}

// CreateTunnel validates and stores a new SSH tunnel linked to a user-owned host instance.
func (h *TunnelHandler) CreateTunnel(input services.SaveTunnelInput) (domain.SSHTunnel, error) {
	return h.tunnels.CreateTunnel(h.bridge.Context(), input)
}

// UpdateTunnel updates a stopped SSH tunnel owned by the active user.
func (h *TunnelHandler) UpdateTunnel(input services.SaveTunnelInput) (domain.SSHTunnel, error) {
	return h.tunnels.UpdateTunnel(h.bridge.Context(), input)
}

// DeleteTunnel deletes a stopped SSH tunnel owned by the active user.
func (h *TunnelHandler) DeleteTunnel(tunnelID string) error {
	return h.tunnels.DeleteTunnel(h.bridge.Context(), tunnelID)
}

// StartTunnel dials the bastion host with TOFU verification and opens the local TCP tunnel listener.
func (h *TunnelHandler) StartTunnel(tunnelID, passwordFallback string) (domain.TunnelRuntimeStatus, error) {
	return h.tunnels.StartTunnel(
		h.bridge.Context(),
		tunnelID,
		passwordFallback,
		h.bridge.RequestTOFUTrust,
	)
}

// StopTunnel stops an active SSH tunnel owned by the active user.
func (h *TunnelHandler) StopTunnel(tunnelID string) error {
	return h.tunnels.StopTunnel(tunnelID)
}

// ListStatuses returns runtime status snapshots for active tunnels owned by the active user.
func (h *TunnelHandler) ListStatuses() ([]domain.TunnelRuntimeStatus, error) {
	return h.tunnels.ListStatuses()
}
