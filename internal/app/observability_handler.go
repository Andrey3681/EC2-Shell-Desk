package app

import (
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

// ObservabilityHandler exposes private service management and browser launching bindings to Wails.
type ObservabilityHandler struct {
	bridge   *EventBridge
	services *services.ObservabilityService
}

// NewObservabilityHandler constructs an ObservabilityHandler wired to ObservabilityService.
func NewObservabilityHandler(bridge *EventBridge, services *services.ObservabilityService) *ObservabilityHandler {
	return &ObservabilityHandler{
		bridge:   bridge,
		services: services,
	}
}

// ListServices returns saved private services owned by the active user.
func (h *ObservabilityHandler) ListServices() ([]domain.PrivateService, error) {
	return h.services.ListServices(h.bridge.Context())
}

// ListServicesByHost returns saved private services for a specific host.
func (h *ObservabilityHandler) ListServicesByHost(hostID string) ([]domain.PrivateService, error) {
	return h.services.ListServicesByHost(h.bridge.Context(), hostID)
}

// CreateService validates and stores a new private service.
func (h *ObservabilityHandler) CreateService(input services.SaveServiceInput) (domain.PrivateService, error) {
	return h.services.CreateService(h.bridge.Context(), input)
}

// UpdateService modifies a stopped private service.
func (h *ObservabilityHandler) UpdateService(input services.SaveServiceInput) (domain.PrivateService, error) {
	return h.services.UpdateService(h.bridge.Context(), input)
}

// DeleteService removes a stopped private service.
func (h *ObservabilityHandler) DeleteService(serviceID string) error {
	return h.services.DeleteService(h.bridge.Context(), serviceID)
}

// StartService dials the host instance, opens the local port forward, checks health, and returns status.
func (h *ObservabilityHandler) StartService(serviceID, passwordFallback string) (domain.ServiceRuntimeStatus, error) {
	return h.services.StartService(
		h.bridge.Context(),
		serviceID,
		passwordFallback,
		h.bridge.RequestTOFUTrust,
	)
}

// StopService closes the local tunnel for the private service.
func (h *ObservabilityHandler) StopService(serviceID string) error {
	return h.services.StopService(serviceID)
}

// ProbeServiceHealth runs an immediate HTTP probe against the service.
func (h *ObservabilityHandler) ProbeServiceHealth(serviceID string) domain.ServiceRuntimeStatus {
	return h.services.ProbeServiceHealth(h.bridge.Context(), serviceID)
}

// ListServiceStatuses returns runtime statuses of active services for the current user.
func (h *ObservabilityHandler) ListServiceStatuses() ([]domain.ServiceRuntimeStatus, error) {
	return h.services.ListStatuses()
}

// OpenService ensures the service tunnel is active and launches it in the system browser.
func (h *ObservabilityHandler) OpenService(serviceID, passwordFallback string) (domain.ServiceRuntimeStatus, error) {
	status, err := h.StartService(serviceID, passwordFallback)
	if err != nil {
		return status, err
	}

	if status.LocalURL != "" {
		runtime.BrowserOpenURL(h.bridge.Context(), status.LocalURL)
	} else {
		return status, fmt.Errorf("service has no local URL")
	}

	return status, nil
}
