package app

import (
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

// SnippetHandler exposes reusable command snippet CRUD and terminal execution bindings to Wails.
type SnippetHandler struct {
	bridge   *EventBridge
	snippets *services.SnippetService
}

// NewSnippetHandler constructs a SnippetHandler wired to SnippetService.
func NewSnippetHandler(bridge *EventBridge, snippets *services.SnippetService) *SnippetHandler {
	return &SnippetHandler{
		bridge:   bridge,
		snippets: snippets,
	}
}

// ListSnippets returns saved command snippets owned by the active user.
func (h *SnippetHandler) ListSnippets() ([]domain.CommandSnippet, error) {
	return h.snippets.ListSnippets(h.bridge.Context())
}

// CreateSnippet validates and persists a new command snippet owned by the active user.
func (h *SnippetHandler) CreateSnippet(input services.SaveSnippetInput) (domain.CommandSnippet, error) {
	return h.snippets.CreateSnippet(h.bridge.Context(), input)
}

// UpdateSnippet updates an existing command snippet owned by the active user.
func (h *SnippetHandler) UpdateSnippet(input services.SaveSnippetInput) (domain.CommandSnippet, error) {
	return h.snippets.UpdateSnippet(h.bridge.Context(), input)
}

// DeleteSnippet deletes a command snippet owned by the active user.
func (h *SnippetHandler) DeleteSnippet(snippetID string) error {
	return h.snippets.DeleteSnippet(h.bridge.Context(), snippetID)
}

// RunSnippetInSession executes or pastes an owned snippet into the active SSH terminal session.
func (h *SnippetHandler) RunSnippetInSession(sessionID, snippetID string) error {
	return h.snippets.RunSnippetInSession(h.bridge.Context(), sessionID, snippetID)
}

// BroadcastSnippet executes or pastes an owned snippet across multiple active SSH sessions.
func (h *SnippetHandler) BroadcastSnippet(sessionIDs []string, snippetID string) error {
	return h.snippets.BroadcastSnippet(h.bridge.Context(), sessionIDs, snippetID)
}
