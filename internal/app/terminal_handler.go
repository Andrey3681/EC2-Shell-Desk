package app

import (
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

// TerminalHandler exposes interactive SSH PTY session and TOFU verification bindings to Wails.
type TerminalHandler struct {
	bridge   *EventBridge
	sessions *services.SessionService
}

// NewTerminalHandler constructs a TerminalHandler wired to SessionService.
func NewTerminalHandler(bridge *EventBridge, sessions *services.SessionService) *TerminalHandler {
	return &TerminalHandler{
		bridge:   bridge,
		sessions: sessions,
	}
}

// OpenSession dials the selected host, resolves TOFU host verification interactively, and starts a PTY shell.
func (h *TerminalHandler) OpenSession(hostID string, cols, rows int, passwordFallback string) (domain.SessionInfo, error) {
	return h.sessions.OpenSession(
		h.bridge.Context(),
		hostID,
		cols,
		rows,
		passwordFallback,
		h.bridge.RequestTOFUTrust,
	)
}

// RespondTOFU submits the user's trust decision for a pending host key prompt.
func (h *TerminalHandler) RespondTOFU(requestID string, trusted bool) error {
	return h.bridge.ResolveTOFU(requestID, trusted)
}

// WriteTerminal sends terminal input text from xterm.js to the active PTY session.
func (h *TerminalHandler) WriteTerminal(sessionID, data string) error {
	return h.sessions.WriteTerminal(sessionID, []byte(data))
}

// ResizeTerminal updates the remote PTY window size when xterm.js resizes.
func (h *TerminalHandler) ResizeTerminal(sessionID string, cols, rows int) error {
	return h.sessions.ResizeTerminal(sessionID, cols, rows)
}

// CloseSession closes an active SSH session tab and cleans up its SFTP subsystem.
func (h *TerminalHandler) CloseSession(sessionID string) error {
	return h.sessions.CloseSession(sessionID)
}

// ListSessions returns active SSH sessions belonging to the authenticated user.
func (h *TerminalHandler) ListSessions() ([]domain.SessionInfo, error) {
	return h.sessions.ListSessions()
}

// ListKnownHosts returns TOFU-verified hosts belonging to the authenticated user.
func (h *TerminalHandler) ListKnownHosts() ([]domain.KnownHost, error) {
	return h.sessions.ListKnownHosts(h.bridge.Context())
}

// RemoveKnownHost deletes a trusted host fingerprint belonging to the authenticated user.
func (h *TerminalHandler) RemoveKnownHost(knownHostID string) error {
	return h.sessions.RemoveKnownHost(h.bridge.Context(), knownHostID)
}
