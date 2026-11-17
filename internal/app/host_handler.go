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

// ListGroups returns all user-defined host groups owned by the active user.
func (h *HostHandler) ListGroups() ([]domain.HostGroup, error) {
	return h.hosts.ListGroups(h.bridge.Context())
}

// CreateGroup creates a new host group owned by the active user.
func (h *HostHandler) CreateGroup(name string) (domain.HostGroup, error) {
	return h.hosts.CreateGroup(h.bridge.Context(), name)
}

// RenameGroup renames an existing host group owned by the active user.
func (h *HostHandler) RenameGroup(groupID, name string) (domain.HostGroup, error) {
	return h.hosts.RenameGroup(h.bridge.Context(), groupID, name)
}

// DeleteGroup removes a host group owned by the active user without deleting its member instances.
func (h *HostHandler) DeleteGroup(groupID string) error {
	return h.hosts.DeleteGroup(h.bridge.Context(), groupID)
}
