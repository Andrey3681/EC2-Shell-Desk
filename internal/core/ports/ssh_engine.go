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

// SSHSessionManager establishes authenticated SSH connections and interactive PTY channels.
type SSHSessionManager interface {
	ConnectPTY(ctx context.Context, req SSHConnectRequest) (PTYSession, error)
}
