package domain

import "time"

// SessionInfo describes an active SSH terminal session owned by an authenticated user.
type SessionInfo struct {
	SessionID   string    `json:"sessionId"`
	OwnerUserID string    `json:"ownerUserId"`
	HostID      string    `json:"hostId"`
	HostAlias   string    `json:"hostAlias"`
	HostAddress string    `json:"hostAddress"`
	Port        int       `json:"port"`
	SSHUsername string    `json:"sshUsername"`
	ConnectedAt time.Time `json:"connectedAt"`
}
