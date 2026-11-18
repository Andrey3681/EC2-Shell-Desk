package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

// SaveSnippetInput captures fields required to create or update a reusable CommandSnippet.
type SaveSnippetInput struct {
	ID          string `json:"id,omitempty"`
	HostID      string `json:"hostId"`
	GroupID     string `json:"groupId"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	AutoExecute bool   `json:"autoExecute"`
	Notes       string `json:"notes"`
}

// SnippetService manages user-isolated command snippets and executes them in active SSH PTY sessions.
type SnippetService struct {
	auth     *AuthService
	snippets ports.SnippetRepository
	hosts    ports.HostRepository
	groups   ports.HostGroupRepository
	sessions *SessionService
}

// NewSnippetService constructs a SnippetService bound to auth, snippet/host/group repositories, and active sessions.
func NewSnippetService(
	auth *AuthService,
	snippets ports.SnippetRepository,
	hosts ports.HostRepository,
	groups ports.HostGroupRepository,
	sessions *SessionService,
) *SnippetService {
	return &SnippetService{
		auth:     auth,
		snippets: snippets,
		hosts:    hosts,
		groups:   groups,
		sessions: sessions,
	}
}

// CreateSnippet validates and stores a new CommandSnippet owned by the authenticated user.
func (s *SnippetService) CreateSnippet(ctx context.Context, input SaveSnippetInput) (domain.CommandSnippet, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.CommandSnippet{}, err
	}

	snip := normalizeSnippetInput(input)
	snip.ID = uuid.NewString()
	snip.OwnerUserID = user.ID
	now := time.Now().UTC()
	snip.CreatedAt = now
	snip.UpdatedAt = now

	if err := snip.Validate(); err != nil {
		return domain.CommandSnippet{}, fmt.Errorf("validate command snippet: %w", err)
	}
	if err := s.verifyOwnedHost(ctx, user.ID, snip.HostID); err != nil {
		return domain.CommandSnippet{}, err
	}
	if err := s.verifyOwnedGroup(ctx, user.ID, snip.GroupID); err != nil {
		return domain.CommandSnippet{}, err
	}

	if err := s.snippets.Create(ctx, snip); err != nil {
		return domain.CommandSnippet{}, fmt.Errorf("create command snippet: %w", err)
	}
	return snip, nil
}

// ListSnippets returns all CommandSnippet records belonging strictly to the authenticated user.
func (s *SnippetService) ListSnippets(ctx context.Context) ([]domain.CommandSnippet, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}
	list, err := s.snippets.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list command snippets: %w", err)
	}
	return list, nil
}

// UpdateSnippet modifies an existing CommandSnippet owned by the authenticated user.
func (s *SnippetService) UpdateSnippet(ctx context.Context, input SaveSnippetInput) (domain.CommandSnippet, error) {
	if strings.TrimSpace(input.ID) == "" {
		return domain.CommandSnippet{}, fmt.Errorf("%w: snippet id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.CommandSnippet{}, err
	}

	existing, err := s.snippets.GetByID(ctx, user.ID, input.ID)
	if err != nil {
		return domain.CommandSnippet{}, fmt.Errorf("lookup command snippet: %w", err)
	}

	updated := normalizeSnippetInput(input)
	updated.ID = existing.ID
	updated.OwnerUserID = user.ID
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = time.Now().UTC()

	if err := updated.Validate(); err != nil {
		return domain.CommandSnippet{}, fmt.Errorf("validate updated command snippet: %w", err)
	}
	if err := s.verifyOwnedHost(ctx, user.ID, updated.HostID); err != nil {
		return domain.CommandSnippet{}, err
	}
	if err := s.verifyOwnedGroup(ctx, user.ID, updated.GroupID); err != nil {
		return domain.CommandSnippet{}, err
	}

	if err := s.snippets.Update(ctx, updated); err != nil {
		return domain.CommandSnippet{}, fmt.Errorf("update command snippet: %w", err)
	}
	return updated, nil
}

// DeleteSnippet removes a CommandSnippet owned by the authenticated user.
func (s *SnippetService) DeleteSnippet(ctx context.Context, snippetID string) error {
	if strings.TrimSpace(snippetID) == "" {
		return fmt.Errorf("%w: snippet id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	if err := s.snippets.Delete(ctx, user.ID, snippetID); err != nil {
		return fmt.Errorf("delete command snippet: %w", err)
	}
	return nil
}

// RunSnippetInSession writes an owned snippet's command into an active owned SSH PTY session.
func (s *SnippetService) RunSnippetInSession(ctx context.Context, sessionID, snippetID string) error {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(snippetID) == "" {
		return fmt.Errorf("%w: session id and snippet id are required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	snip, err := s.snippets.GetByID(ctx, user.ID, snippetID)
	if err != nil {
		return fmt.Errorf("lookup snippet to run: %w", err)
	}

	payload := formatSnippetPayload(snip)
	if err := s.sessions.WriteTerminal(sessionID, payload); err != nil {
		return fmt.Errorf("send snippet to terminal session: %w", err)
	}
	return nil
}

// BroadcastSnippet executes an owned snippet across all specified active SSH sessions owned by the user.
func (s *SnippetService) BroadcastSnippet(ctx context.Context, sessionIDs []string, snippetID string) error {
	if len(sessionIDs) == 0 {
		return fmt.Errorf("%w: at least one active session id is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(snippetID) == "" {
		return fmt.Errorf("%w: snippet id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	snip, err := s.snippets.GetByID(ctx, user.ID, snippetID)
	if err != nil {
		return fmt.Errorf("lookup snippet to broadcast: %w", err)
	}

	payload := formatSnippetPayload(snip)
	for _, sid := range sessionIDs {
		if err := s.sessions.WriteTerminal(sid, payload); err != nil {
			return fmt.Errorf("broadcast snippet to session %s: %w", sid, err)
		}
	}
	return nil
}

func (s *SnippetService) verifyOwnedHost(ctx context.Context, ownerUserID, hostID string) error {
	if hostID == "" || s.hosts == nil {
		return nil
	}
	if _, err := s.hosts.GetByID(ctx, ownerUserID, hostID); err != nil {
		return fmt.Errorf("verify snippet host ownership: %w", err)
	}
	return nil
}

func (s *SnippetService) verifyOwnedGroup(ctx context.Context, ownerUserID, groupID string) error {
	if groupID == "" || s.groups == nil {
		return nil
	}
	if _, err := s.groups.GetByID(ctx, ownerUserID, groupID); err != nil {
		return fmt.Errorf("verify snippet group ownership: %w", err)
	}
	return nil
}

func normalizeSnippetInput(input SaveSnippetInput) domain.CommandSnippet {
	normalizedCmd := strings.ReplaceAll(input.Command, "\r\n", "\n")
	normalizedCmd = strings.TrimRight(normalizedCmd, "\r\n")
	return domain.CommandSnippet{
		ID:          strings.TrimSpace(input.ID),
		HostID:      strings.TrimSpace(input.HostID),
		GroupID:     strings.TrimSpace(input.GroupID),
		Name:        strings.TrimSpace(input.Name),
		Command:     normalizedCmd,
		AutoExecute: input.AutoExecute,
		Notes:       strings.TrimSpace(input.Notes),
	}
}

func formatSnippetPayload(snip domain.CommandSnippet) []byte {
	cmd := strings.ReplaceAll(snip.Command, "\r\n", "\n")
	cmd = strings.TrimRight(cmd, "\r\n")
	if snip.AutoExecute {
		cmd += "\r"
	}
	return []byte(cmd)
}
