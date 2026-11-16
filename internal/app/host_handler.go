package app

import (
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

// HostHandler exposes EC2 and Linux host instance CRUD bindings to Wails.
type HostHandler struct {
	bridge *EventBridge
	hosts  *services.HostService
}

// NewHostHandler constructs a HostHandler wired to HostService.
func NewHostHandler(bridge *EventBridge, hosts *services.HostService) *HostHandler {
	return &HostHandler{
		bridge: bridge,
		hosts:  hosts,
	}
}

// ListHosts returns all host instances owned by the active user.
func (h *HostHandler) ListHosts() ([]domain.HostInstance, error) {
	return h.hosts.ListHosts(h.bridge.Context())
}

// CreateHost validates and persists a new host instance owned by the active user.
func (h *HostHandler) CreateHost(input services.SaveHostInput) (domain.HostInstance, error) {
	return h.hosts.CreateHost(h.bridge.Context(), input)
}

// UpdateHost updates an existing host instance owned by the active user.
func (h *HostHandler) UpdateHost(input services.SaveHostInput) (domain.HostInstance, error) {
	return h.hosts.UpdateHost(h.bridge.Context(), input)
}

// DeleteHost removes a host instance owned by the active user.
func (h *HostHandler) DeleteHost(hostID string) error {
	return h.hosts.DeleteHost(h.bridge.Context(), hostID)
}
