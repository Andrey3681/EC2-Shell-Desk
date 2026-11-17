package domain

import (
	"fmt"
	"strings"
	"time"
)

// TunnelPreset identifies a database or service template used to configure an SSH tunnel.
type TunnelPreset string

const (
	// TunnelPresetPostgreSQL identifies a PostgreSQL / AWS RDS Postgres tunnel (default remote port 5432).
	TunnelPresetPostgreSQL TunnelPreset = "postgresql"
	// TunnelPresetMySQL identifies a MySQL / MariaDB / Aurora tunnel (default remote port 3306).
	TunnelPresetMySQL TunnelPreset = "mysql"
	// TunnelPresetMongoDB identifies a MongoDB / AWS DocumentDB tunnel (default remote port 27017).
	TunnelPresetMongoDB TunnelPreset = "mongodb"
	// TunnelPresetRedis identifies a Redis / AWS ElastiCache tunnel (default remote port 6379).
	TunnelPresetRedis TunnelPreset = "redis"
	// TunnelPresetSQLServer identifies a Microsoft SQL Server / RDS SQL Server tunnel (default remote port 1433).
	TunnelPresetSQLServer TunnelPreset = "sqlserver"
	// TunnelPresetCustom identifies a user-defined TCP port-forwarding tunnel.
	TunnelPresetCustom TunnelPreset = "custom"
)

// TunnelState describes the live runtime status of a local SSH port-forwarding tunnel.
type TunnelState string

const (
	// TunnelStateStopped indicates the tunnel listener is inactive.
	TunnelStateStopped TunnelState = "stopped"
	// TunnelStateConnecting indicates the SSH bastion handshake is in progress.
	TunnelStateConnecting TunnelState = "connecting"
	// TunnelStateActive indicates the local TCP port is listening and forwarding over SSH.
	TunnelStateActive TunnelState = "active"
	// TunnelStateError indicates the tunnel stopped due to a connection or bind failure.
	TunnelStateError TunnelState = "error"
)

// DefaultTunnelLocalHost restricts local tunnel listeners to the loopback interface by default.
const DefaultTunnelLocalHost = "127.0.0.1"

// SSHTunnel defines a saved local port-forwarding tunnel routed through a user-owned HostInstance bastion.
type SSHTunnel struct {
	ID          string       `json:"id"`
	OwnerUserID string       `json:"ownerUserId"`
	HostID      string       `json:"hostId"`
	Name        string       `json:"name"`
	Preset      TunnelPreset `json:"preset"`
	LocalHost   string       `json:"localHost"`
	LocalPort   int          `json:"localPort"`
	RemoteHost  string       `json:"remoteHost"`
	RemotePort  int          `json:"remotePort"`
	Notes       string       `json:"notes"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// Validate checks that required SSHTunnel fields, ports, and endpoints are valid.
func (t SSHTunnel) Validate() error {
	if strings.TrimSpace(t.OwnerUserID) == "" {
		return fmt.Errorf("%w: owner user id is required", ErrInvalidInput)
	}
	if strings.TrimSpace(t.HostID) == "" {
		return fmt.Errorf("%w: bastion host id is required", ErrInvalidInput)
	}
	name := strings.TrimSpace(t.Name)
	if name == "" || len(name) > 128 {
		return fmt.Errorf("%w: tunnel name must be between 1 and 128 characters", ErrInvalidInput)
	}
	switch t.Preset {
	case "", TunnelPresetPostgreSQL, TunnelPresetMySQL, TunnelPresetMongoDB, TunnelPresetRedis, TunnelPresetSQLServer, TunnelPresetCustom:
	default:
		return fmt.Errorf("%w: unsupported tunnel preset %q", ErrInvalidInput, t.Preset)
	}
	localHost := strings.TrimSpace(t.LocalHost)
	if localHost == "" || strings.ContainsAny(localHost, " \t\r\n") {
		return fmt.Errorf("%w: local bind host must be non-empty without whitespace", ErrInvalidInput)
	}
	if t.LocalPort < 1 || t.LocalPort > 65535 {
		return fmt.Errorf("%w: local port must be between 1 and 65535", ErrInvalidInput)
	}
	remoteHost := strings.TrimSpace(t.RemoteHost)
	if remoteHost == "" || strings.ContainsAny(remoteHost, " \t\r\n") {
		return fmt.Errorf("%w: remote target host must be non-empty without whitespace", ErrInvalidInput)
	}
	if t.RemotePort < 1 || t.RemotePort > 65535 {
		return fmt.Errorf("%w: remote port must be between 1 and 65535", ErrInvalidInput)
	}
	return nil
}

// TunnelRuntimeStatus reports real-time state and traffic counters for an SSHTunnel.
type TunnelRuntimeStatus struct {
	TunnelID          string      `json:"tunnelId"`
	OwnerUserID       string      `json:"ownerUserId"`
	State             TunnelState `json:"state"`
	LocalEndpoint     string      `json:"localEndpoint"`
	ActiveConnections int         `json:"activeConnections"`
	BytesIn           int64       `json:"bytesIn"`
	BytesOut          int64       `json:"bytesOut"`
	ErrorMessage      string      `json:"errorMessage,omitempty"`
	StartedAt         *time.Time  `json:"startedAt,omitempty"`
}
