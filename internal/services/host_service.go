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

// SaveHostInput captures fields required to create or update an EC2/Linux HostInstance.
type SaveHostInput struct {
	ID          string             `json:"id,omitempty"`
	GroupID     string             `json:"groupId"`
	Alias       string             `json:"alias"`
	HostAddress string             `json:"hostAddress"`
	Port        int                `json:"port"`
	SSHUsername string             `json:"sshUsername"`
	SSHKeyID    string             `json:"sshKeyId"`
	Environment domain.Environment `json:"environment"`
	Notes       string             `json:"notes"`
}

// HostService manages EC2 and Linux host instances and user-defined groups isolated per authenticated user.
type HostService struct {
	auth   *AuthService
	hosts  ports.HostRepository
	keys   ports.KeyRepository
	groups ports.HostGroupRepository
}

// NewHostService constructs a HostService with active session, host, key, and group repositories.
func NewHostService(
	auth *AuthService,
	hosts ports.HostRepository,
	keys ports.KeyRepository,
	groups ports.HostGroupRepository,
) *HostService {
	return &HostService{
		auth:   auth,
		hosts:  hosts,
		keys:   keys,
		groups: groups,
	}
}

// CreateHost validates and persists a new HostInstance owned by the currently authenticated user.
func (s *HostService) CreateHost(ctx context.Context, input SaveHostInput) (domain.HostInstance, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.HostInstance{}, err
	}

	host := normalizeHostInput(input)
	host.ID = uuid.NewString()
	host.OwnerUserID = user.ID
	now := time.Now().UTC()
	host.CreatedAt = now
	host.UpdatedAt = now

	if err := host.Validate(); err != nil {
		return domain.HostInstance{}, fmt.Errorf("validate host instance: %w", err)
	}
	if err := s.verifyOwnedKey(ctx, user.ID, host.SSHKeyID); err != nil {
		return domain.HostInstance{}, err
	}
	if err := s.verifyOwnedGroup(ctx, user.ID, host.GroupID); err != nil {
		return domain.HostInstance{}, err
	}

	if err := s.hosts.Create(ctx, host); err != nil {
		return domain.HostInstance{}, fmt.Errorf("create host instance: %w", err)
	}
	return host, nil
}

// ListHosts returns all HostInstance records belonging to the currently authenticated user.
func (s *HostService) ListHosts(ctx context.Context) ([]domain.HostInstance, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}

	hosts, err := s.hosts.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list user hosts: %w", err)
	}
	return hosts, nil
}

// GetHost retrieves a single HostInstance belonging to the currently authenticated user.
func (s *HostService) GetHost(ctx context.Context, hostID string) (domain.HostInstance, error) {
	if strings.TrimSpace(hostID) == "" {
		return domain.HostInstance{}, fmt.Errorf("%w: host id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.HostInstance{}, err
	}

	host, err := s.hosts.GetByID(ctx, user.ID, hostID)
	if err != nil {
		return domain.HostInstance{}, fmt.Errorf("get host instance: %w", err)
	}
	return host, nil
}

// UpdateHost modifies an existing HostInstance owned by the currently authenticated user.
func (s *HostService) UpdateHost(ctx context.Context, input SaveHostInput) (domain.HostInstance, error) {
	if strings.TrimSpace(input.ID) == "" {
		return domain.HostInstance{}, fmt.Errorf("%w: host id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.HostInstance{}, err
	}

	existing, err := s.hosts.GetByID(ctx, user.ID, input.ID)
	if err != nil {
		return domain.HostInstance{}, fmt.Errorf("lookup host for update: %w", err)
	}

	updated := normalizeHostInput(input)
	updated.ID = existing.ID
	updated.OwnerUserID = user.ID
	updated.LastConnectedAt = existing.LastConnectedAt
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = time.Now().UTC()

	if err := updated.Validate(); err != nil {
		return domain.HostInstance{}, fmt.Errorf("validate updated host instance: %w", err)
	}
	if err := s.verifyOwnedKey(ctx, user.ID, updated.SSHKeyID); err != nil {
		return domain.HostInstance{}, err
	}
	if err := s.verifyOwnedGroup(ctx, user.ID, updated.GroupID); err != nil {
		return domain.HostInstance{}, err
	}

	if err := s.hosts.Update(ctx, updated); err != nil {
		return domain.HostInstance{}, fmt.Errorf("update host instance: %w", err)
	}
	return updated, nil
}

// DeleteHost deletes a HostInstance owned by the currently authenticated user.
func (s *HostService) DeleteHost(ctx context.Context, hostID string) error {
	if strings.TrimSpace(hostID) == "" {
		return fmt.Errorf("%w: host id is required", domain.ErrInvalidInput)
	}

	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	if err := s.hosts.Delete(ctx, user.ID, hostID); err != nil {
		return fmt.Errorf("delete host instance: %w", err)
	}
	return nil
}

// ListGroups returns all user-defined host groups belonging to the active user.
func (s *HostService) ListGroups(ctx context.Context) ([]domain.HostGroup, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}
	if s.groups == nil {
		return make([]domain.HostGroup, 0), nil
	}
	groups, err := s.groups.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list host groups: %w", err)
	}
	return groups, nil
}

// CreateGroup validates and creates a new host group owned by the active user.
func (s *HostService) CreateGroup(ctx context.Context, name string) (domain.HostGroup, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.HostGroup{}, err
	}
	if s.groups == nil {
		return domain.HostGroup{}, fmt.Errorf("%w: group repository is not configured", domain.ErrInvalidInput)
	}
	trimmed := strings.TrimSpace(name)
	if err := domain.ValidateGroupName(trimmed); err != nil {
		return domain.HostGroup{}, fmt.Errorf("validate group name: %w", err)
	}

	now := time.Now().UTC()
	group := domain.HostGroup{
		ID:          uuid.NewString(),
		OwnerUserID: user.ID,
		Name:        trimmed,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.groups.Create(ctx, group); err != nil {
		return domain.HostGroup{}, fmt.Errorf("create host group: %w", err)
	}
	return group, nil
}

// RenameGroup updates the name of an existing host group owned by the active user.
func (s *HostService) RenameGroup(ctx context.Context, groupID, name string) (domain.HostGroup, error) {
	if strings.TrimSpace(groupID) == "" {
		return domain.HostGroup{}, fmt.Errorf("%w: group id is required", domain.ErrInvalidInput)
	}
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.HostGroup{}, err
	}
	if s.groups == nil {
		return domain.HostGroup{}, fmt.Errorf("%w: group repository is not configured", domain.ErrInvalidInput)
	}
	trimmed := strings.TrimSpace(name)
	if err := domain.ValidateGroupName(trimmed); err != nil {
		return domain.HostGroup{}, fmt.Errorf("validate group name: %w", err)
	}

	existing, err := s.groups.GetByID(ctx, user.ID, groupID)
	if err != nil {
		return domain.HostGroup{}, fmt.Errorf("lookup host group: %w", err)
	}
	existing.Name = trimmed
	existing.UpdatedAt = time.Now().UTC()
	if err := s.groups.Update(ctx, existing); err != nil {
		return domain.HostGroup{}, fmt.Errorf("rename host group: %w", err)
	}
	return existing, nil
}

// DeleteGroup removes a host group owned by the active user; member instances become ungrouped.
func (s *HostService) DeleteGroup(ctx context.Context, groupID string) error {
	if strings.TrimSpace(groupID) == "" {
		return fmt.Errorf("%w: group id is required", domain.ErrInvalidInput)
	}
	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}
	if s.groups == nil {
		return fmt.Errorf("%w: group repository is not configured", domain.ErrInvalidInput)
	}
	if err := s.groups.Delete(ctx, user.ID, groupID); err != nil {
		return fmt.Errorf("delete host group: %w", err)
	}
	return nil
}

func (s *HostService) verifyOwnedKey(ctx context.Context, ownerUserID, sshKeyID string) error {
	if sshKeyID == "" {
		return nil
	}
	if _, err := s.keys.GetByID(ctx, ownerUserID, sshKeyID); err != nil {
		return fmt.Errorf("verify ssh key ownership: %w", err)
	}
	return nil
}

func (s *HostService) verifyOwnedGroup(ctx context.Context, ownerUserID, groupID string) error {
	if groupID == "" || s.groups == nil {
		return nil
	}
	if _, err := s.groups.GetByID(ctx, ownerUserID, groupID); err != nil {
		return fmt.Errorf("verify host group ownership: %w", err)
	}
	return nil
}

func normalizeHostInput(input SaveHostInput) domain.HostInstance {
	port := input.Port
	if port == 0 {
		port = domain.DefaultSSHPort
	}
	env := input.Environment
	if env == "" {
		env = domain.EnvProd
	}
	return domain.HostInstance{
		ID:          strings.TrimSpace(input.ID),
		GroupID:     strings.TrimSpace(input.GroupID),
		Alias:       strings.TrimSpace(input.Alias),
		HostAddress: strings.TrimSpace(input.HostAddress),
		Port:        port,
		SSHUsername: strings.TrimSpace(input.SSHUsername),
		SSHKeyID:    strings.TrimSpace(input.SSHKeyID),
		Environment: env,
		Notes:       strings.TrimSpace(input.Notes),
	}
}
