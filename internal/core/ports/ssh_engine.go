package ports

import (
	"context"
	"io"

	"ec2-shell-desk/internal/core/domain"
)

// HostKeyChallenge carries remote server host key details during a TOFU verification check.
type HostKeyChallenge struct {
	HostAddress   string
	Port          int
	KeyType       string
	Fingerprint   string
	PublicKeyBlob []byte
	KnownMismatch bool
}

// HostKeyVerifier evaluates whether a remote server public key should be trusted for a connection.
type HostKeyVerifier func(ctx context.Context, challenge HostKeyChallenge) error

// SSHConnectRequest bundles credentials and terminal sizing needed to open an SSH session.
type SSHConnectRequest struct {
	Host          domain.HostInstance
	PrivateKeyPEM []byte
	Passphrase    []byte
	Password      string
	Cols          int
	Rows          int
	VerifyHostKey HostKeyVerifier
}

// PTYSession represents an active interactive SSH terminal session.
type PTYSession interface {
	ID() string
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
	Resize(cols, rows int) error
	OpenSFTP() (SFTPClient, error)
	Wait() error
	Close() error
}

// TunnelForwardRequest bundles SSH bastion credentials and local-to-remote port forwarding targets.
type TunnelForwardRequest struct {
	Host          domain.HostInstance
	PrivateKeyPEM []byte
	Passphrase    []byte
	Password      string
	LocalHost     string
	LocalPort     int
	RemoteHost    string
	RemotePort    int
	VerifyHostKey HostKeyVerifier
	OnTraffic     func(activeConns int, bytesIn, bytesOut int64)
}

// TunnelHandle represents an active local TCP port forwarder routed over an authenticated SSH client.
type TunnelHandle interface {
	ID() string
	LocalAddr() string
	ActiveConnections() int
	BytesTransferred() (bytesIn, bytesOut int64)
	Done() <-chan struct{}
	Err() error
	Close() error
}

// SSHSessionManager establishes authenticated SSH connections, interactive PTY channels, and local port forwards.
type SSHSessionManager interface {
	ConnectPTY(ctx context.Context, req SSHConnectRequest) (PTYSession, error)
	StartLocalForward(ctx context.Context, req TunnelForwardRequest) (TunnelHandle, error)
}
