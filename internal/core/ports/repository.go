package ports

import (
	"context"
	"time"

	"ec2-shell-desk/internal/core/domain"
)

// UserRepository persists and retrieves local application users.
type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	GetByID(ctx context.Context, userID string) (domain.User, error)
	GetByUsername(ctx context.Context, username string) (domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	Count(ctx context.Context) (int, error)
}

// KeyRepository persists and retrieves encrypted SSH keys scoped strictly by owner user ID.
type KeyRepository interface {
	Create(ctx context.Context, key domain.SSHKey) error
	GetByID(ctx context.Context, ownerUserID, keyID string) (domain.SSHKey, error)
	ListByUser(ctx context.Context, ownerUserID string) ([]domain.SSHKey, error)
	Delete(ctx context.Context, ownerUserID, keyID string) error
	CountHostReferences(ctx context.Context, ownerUserID, keyID string) (int, error)
}

// HostRepository persists and retrieves EC2/Linux host instances scoped strictly by owner user ID.
type HostRepository interface {
	Create(ctx context.Context, host domain.HostInstance) error
	GetByID(ctx context.Context, ownerUserID, hostID string) (domain.HostInstance, error)
	ListByUser(ctx context.Context, ownerUserID string) ([]domain.HostInstance, error)
	Update(ctx context.Context, host domain.HostInstance) error
	Delete(ctx context.Context, ownerUserID, hostID string) error
	TouchLastConnected(ctx context.Context, ownerUserID, hostID string, connectedAt time.Time) error
}

// KnownHostRepository persists TOFU-verified SSH host keys scoped strictly by owner user ID.
type KnownHostRepository interface {
	GetByHostPort(ctx context.Context, ownerUserID, hostAddress string, port int) (domain.KnownHost, error)
	Upsert(ctx context.Context, knownHost domain.KnownHost) error
	ListByUser(ctx context.Context, ownerUserID string) ([]domain.KnownHost, error)
	Delete(ctx context.Context, ownerUserID, knownHostID string) error
}

// TunnelRepository persists SSH port-forwarding tunnel definitions scoped strictly by owner user ID.
type TunnelRepository interface {
	Create(ctx context.Context, tunnel domain.SSHTunnel) error
	GetByID(ctx context.Context, ownerUserID, tunnelID string) (domain.SSHTunnel, error)
	ListByUser(ctx context.Context, ownerUserID string) ([]domain.SSHTunnel, error)
	Update(ctx context.Context, tunnel domain.SSHTunnel) error
	Delete(ctx context.Context, ownerUserID, tunnelID string) error
	CountByHost(ctx context.Context, ownerUserID, hostID string) (int, error)
}

// HostGroupRepository persists user-defined host instance groups scoped strictly by owner user ID.
type HostGroupRepository interface {
	Create(ctx context.Context, group domain.HostGroup) error
	GetByID(ctx context.Context, ownerUserID, groupID string) (domain.HostGroup, error)
	ListByUser(ctx context.Context, ownerUserID string) ([]domain.HostGroup, error)
	Update(ctx context.Context, group domain.HostGroup) error
	Delete(ctx context.Context, ownerUserID, groupID string) error
}
