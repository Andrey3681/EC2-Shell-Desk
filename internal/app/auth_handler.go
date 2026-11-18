package app

import (
	"errors"
	"fmt"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

// AuthStatus summarizes whether initial setup is completed and whether a user vault is unlocked.
type AuthStatus struct {
	HasUsers bool                `json:"hasUsers"`
	Unlocked bool                `json:"unlocked"`
	User     *domain.UserProfile `json:"user,omitempty"`
}

// AuthHandler exposes local authentication and multi-user session lifecycle bindings to Wails.
type AuthHandler struct {
	bridge        *EventBridge
	auth          *services.AuthService
	sessions      *services.SessionService
	transfers     *services.TransferService
	tunnels       *services.TunnelService
	observability *services.ObservabilityService
}

// NewAuthHandler constructs an AuthHandler wired to authentication and active session lifecycles.
func NewAuthHandler(
	bridge *EventBridge,
	auth *services.AuthService,
	sessions *services.SessionService,
	transfers *services.TransferService,
	tunnels *services.TunnelService,
	observability *services.ObservabilityService,
) *AuthHandler {
	return &AuthHandler{
		bridge:        bridge,
		auth:          auth,
		sessions:      sessions,
		transfers:     transfers,
		tunnels:       tunnels,
		observability: observability,
	}
}

// Status returns whether local users exist and the currently unlocked user profile if any.
func (h *AuthHandler) Status() (AuthStatus, error) {
	hasUsers, err := h.auth.HasUsers(h.bridge.Context())
	if err != nil {
		return AuthStatus{}, fmt.Errorf("check user initialization status: %w", err)
	}

	profile, curErr := h.auth.CurrentUser()
	if curErr != nil {
		if errors.Is(curErr, domain.ErrVaultLocked) {
			return AuthStatus{HasUsers: hasUsers, Unlocked: false}, nil
		}
		return AuthStatus{}, curErr
	}
	return AuthStatus{
		HasUsers: hasUsers,
		Unlocked: true,
		User:     &profile,
	}, nil
}

// ListUsers returns safe public profiles for all configured local users.
func (h *AuthHandler) ListUsers() ([]domain.UserProfile, error) {
	return h.auth.ListUsers(h.bridge.Context())
}

// BootstrapInitialUser creates the first local user and unlocks their encrypted vault.
func (h *AuthHandler) BootstrapInitialUser(username, password string) (domain.UserProfile, error) {
	return h.auth.BootstrapInitialUser(h.bridge.Context(), username, password)
}

// CreateUser registers an additional isolated local user account.
func (h *AuthHandler) CreateUser(username, password string) (domain.UserProfile, error) {
	return h.auth.CreateUser(h.bridge.Context(), username, password)
}

// Login authenticates a user and switches the unlocked vault session, closing prior user sessions first.
func (h *AuthHandler) Login(username, password string) (domain.UserProfile, error) {
	if h.transfers != nil {
		h.transfers.CloseAll()
	}
	if h.tunnels != nil {
		h.tunnels.CloseAll()
	}
	if h.observability != nil {
		h.observability.CloseAll()
	}
	if h.sessions != nil {
		h.sessions.CloseAll()
	}
	return h.auth.Login(h.bridge.Context(), username, password)
}

// Lock terminates active SSH/SFTP sessions and tunnels and wipes the unlocked master key from memory.
func (h *AuthHandler) Lock() {
	if h.transfers != nil {
		h.transfers.CloseAll()
	}
	if h.tunnels != nil {
		h.tunnels.CloseAll()
	}
	if h.observability != nil {
		h.observability.CloseAll()
	}
	if h.sessions != nil {
		h.sessions.CloseAll()
	}
	h.auth.Lock()
}
