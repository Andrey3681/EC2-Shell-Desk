package domain

import "errors"

var (
	// ErrNotFound indicates that the requested entity does not exist or is not visible to the current user.
	ErrNotFound = errors.New("record not found")

	// ErrUnauthorized indicates that an operation requires an authenticated user or lacks ownership.
	ErrUnauthorized = errors.New("unauthorized access")

	// ErrInvalidInput indicates that one or more caller-supplied fields failed domain validation.
	ErrInvalidInput = errors.New("invalid input")

	// ErrDuplicateRecord indicates a uniqueness constraint violation such as an existing username or alias.
	ErrDuplicateRecord = errors.New("duplicate record")

	// ErrVaultLocked indicates that the user vault is locked and no master key is active in memory.
	ErrVaultLocked = errors.New("vault is locked")

	// ErrInvalidCredentials indicates an unrecognized username or incorrect password.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrUserAlreadyExists indicates that a user with the requested username already exists.
	ErrUserAlreadyExists = errors.New("user already exists")

	// ErrInitialUserAlreadySet indicates that initial user bootstrap was attempted after a user already exists.
	ErrInitialUserAlreadySet = errors.New("initial user is already configured")

	// ErrKeyInUse indicates that an SSH key cannot be deleted while referenced by one or more host instances.
	ErrKeyInUse = errors.New("ssh key is referenced by host instances")

	// ErrUnsupportedKeyFormat indicates an unrecognized or unsupported private key format (e.g. PuTTY .ppk).
	ErrUnsupportedKeyFormat = errors.New("unsupported private key format")

	// ErrPassphraseRequired indicates that an encrypted private key requires a passphrase to unlock.
	ErrPassphraseRequired = errors.New("passphrase required for encrypted private key")

	// ErrHostKeyUnknown indicates a first-time SSH connection requiring TOFU host key confirmation.
	ErrHostKeyUnknown = errors.New("host key is not yet trusted")

	// ErrHostKeyMismatch indicates that the remote host key differs from the stored known_hosts fingerprint.
	ErrHostKeyMismatch = errors.New("remote host key mismatch")

	// ErrSessionClosed indicates that the target SSH or SFTP session is no longer active.
	ErrSessionClosed = errors.New("session is closed")

	// ErrTransferCancelled indicates that a file transfer was cancelled via context cancellation.
	ErrTransferCancelled = errors.New("transfer cancelled")

	// ErrPathTraversal indicates an unsafe path attempting to escape allowed directory boundaries.
	ErrPathTraversal = errors.New("unsafe path traversal detected")

	// ErrHostInUse indicates that a host instance cannot be deleted while referenced by saved SSH tunnels.
	ErrHostInUse = errors.New("host instance is referenced by ssh tunnels")

	// ErrTunnelAlreadyActive indicates that the requested SSH tunnel is already running.
	ErrTunnelAlreadyActive = errors.New("ssh tunnel is already active")

	// ErrLocalPortInUse indicates that the requested local TCP port is already bound by another process or tunnel.
	ErrLocalPortInUse = errors.New("local port is already in use")
)
