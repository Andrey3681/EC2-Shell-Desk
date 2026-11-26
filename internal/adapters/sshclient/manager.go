package sshclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const (
	defaultDialTimeout       = 15 * time.Second
	defaultKeepaliveInterval = 30 * time.Second
	defaultPTYCols           = 80
	defaultPTYRows           = 24
)

// SFTPOpener opens an SFTP subsystem client over an active *ssh.Client connection.
type SFTPOpener func(client *ssh.Client) (ports.SFTPClient, error)

// Option configures optional Manager behavior such as dial timeouts and keepalive frequency.
type Option func(*Manager)

// WithDialTimeout overrides the TCP and SSH handshake timeout duration.
func WithDialTimeout(timeout time.Duration) Option {
	return func(m *Manager) {
		if timeout > 0 {
			m.dialTimeout = timeout
		}
	}
}

// WithKeepaliveInterval overrides the periodic SSH keepalive request interval.
func WithKeepaliveInterval(interval time.Duration) Option {
	return func(m *Manager) {
		if interval > 0 {
			m.keepaliveInterval = interval
		}
	}
}

// Manager implements ports.SSHSessionManager using golang.org/x/crypto/ssh.
type Manager struct {
	dialTimeout       time.Duration
	keepaliveInterval time.Duration
	sftpOpener        SFTPOpener
}

// NewManager constructs an SSH session manager with optional SFTP opener and timing options.
func NewManager(sftpOpener SFTPOpener, opts ...Option) *Manager {
	m := &Manager{
		dialTimeout:       defaultDialTimeout,
		keepaliveInterval: defaultKeepaliveInterval,
		sftpOpener:        sftpOpener,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

// ConnectPTY establishes an authenticated SSH connection with TOFU verification and starts an interactive PTY shell.
func (m *Manager) ConnectPTY(ctx context.Context, req ports.SSHConnectRequest) (ports.PTYSession, error) {
	client, err := m.dialSSHClient(ctx, req.Host, req.PrivateKeyPEM, req.Passphrase, req.Password, req.VerifyHostKey)
	if err != nil {
		return nil, err
	}

	cols := req.Cols
	if cols <= 0 {
		cols = defaultPTYCols
	}
	rows := req.Rows
	if rows <= 0 {
		rows = defaultPTYRows
	}

	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("open ssh session: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("attach pty stdin: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("attach pty stdout: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("attach pty stderr: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}
	if err := session.Shell(); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("start remote shell: %w", err)
	}

	sessionCtx, cancelKeepalive := context.WithCancel(context.WithoutCancel(ctx))
	pty := &ptySession{
		id:         uuid.NewString(),
		client:     client,
		session:    session,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		sftpOpener: m.sftpOpener,
		cancel:     cancelKeepalive,
	}

	go runSSHKeepalive(sessionCtx, client, m.keepaliveInterval)
	return pty, nil
}

func (m *Manager) dialSSHClient(
	ctx context.Context,
	host domain.HostInstance,
	privateKeyPEM, passphrase []byte,
	password string,
	verifyHostKey ports.HostKeyVerifier,
) (*ssh.Client, error) {
	if verifyHostKey == nil {
		return nil, fmt.Errorf("%w: host key verifier callback is required", domain.ErrInvalidInput)
	}

	authMethods, err := buildAuthMethods(privateKeyPEM, passphrase, password)
	if err != nil {
		return nil, err
	}

	hostPort := net.JoinHostPort(host.HostAddress, strconv.Itoa(host.Port))
	sshCfg := &ssh.ClientConfig{
		User:    host.SSHUsername,
		Auth:    authMethods,
		Timeout: m.dialTimeout,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			challenge := ports.HostKeyChallenge{
				HostAddress:   host.HostAddress,
				Port:          host.Port,
				KeyType:       key.Type(),
				Fingerprint:   ssh.FingerprintSHA256(key),
				PublicKeyBlob: key.Marshal(),
			}
			return verifyHostKey(ctx, challenge)
		},
	}

	dialCtx, cancelDial := context.WithTimeout(ctx, m.dialTimeout)
	defer cancelDial()

	dialer := &net.Dialer{Timeout: m.dialTimeout}
	netConn, err := dialer.DialContext(dialCtx, "tcp", hostPort)
	if err != nil {
		return nil, fmt.Errorf("dial ssh host %s: %w", hostPort, err)
	}

	// Abort handshake immediately if caller cancels context mid-handshake.
	handshakeDone := make(chan struct{})
	go func() {
		select {
		case <-dialCtx.Done():
			_ = netConn.Close()
		case <-handshakeDone:
		}
	}()

	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, hostPort, sshCfg)
	close(handshakeDone)
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("ssh handshake with %s: %w", hostPort, err)
	}

	return ssh.NewClient(sshConn, chans, reqs), nil
}

func buildAuthMethods(rawPEM, passphrase []byte, password string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	trimmedPEM := bytes.TrimSpace(rawPEM)
	if len(trimmedPEM) > 0 {
		if bytes.HasPrefix(trimmedPEM, []byte("PuTTY-User-Key-File")) {
			return nil, fmt.Errorf("%w: PuTTY .ppk keys must be exported to OpenSSH PEM format", domain.ErrUnsupportedKeyFormat)
		}

		var signer ssh.Signer
		var err error
		if len(passphrase) > 0 {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(trimmedPEM, passphrase)
		} else {
			signer, err = ssh.ParsePrivateKey(trimmedPEM)
		}
		if err != nil {
			var missingErr *ssh.PassphraseMissingError
			if errors.As(err, &missingErr) {
				return nil, fmt.Errorf("%w", domain.ErrPassphraseRequired)
			}
			return nil, fmt.Errorf("%w: parse private key: %w", domain.ErrUnsupportedKeyFormat, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if password != "" {
		methods = append(methods, ssh.Password(password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("%w: either private key or password is required", domain.ErrInvalidInput)
	}
	return methods, nil
}

type ptySession struct {
	id         string
	client     *ssh.Client
	session    *ssh.Session
	stdin      io.WriteCloser
	stdout     io.Reader
	stderr     io.Reader
	sftpOpener SFTPOpener
	cancel     context.CancelFunc
	closeOnce  sync.Once
	mu         sync.Mutex
	closed     bool
}

// ID returns the unique identifier of this PTY session.
func (s *ptySession) ID() string {
	return s.id
}

// Stdin returns the writable pipe feeding remote PTY standard input.
func (s *ptySession) Stdin() io.WriteCloser {
	return s.stdin
}

// Stdout returns the readable pipe streaming remote PTY standard output.
func (s *ptySession) Stdout() io.Reader {
	return s.stdout
}

// Stderr returns the readable pipe streaming remote PTY standard error.
func (s *ptySession) Stderr() io.Reader {
	return s.stderr
}

// Resize updates the remote PTY window dimensions in columns and rows.
func (s *ptySession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("%w: terminal cols and rows must be positive", domain.ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("%w", domain.ErrSessionClosed)
	}
	if err := s.session.WindowChange(rows, cols); err != nil {
		return fmt.Errorf("resize remote pty: %w", err)
	}
	return nil
}

// OpenSFTP opens an SFTP subsystem client over the underlying authenticated SSH connection.
func (s *ptySession) OpenSFTP() (ports.SFTPClient, error) {
	if s.sftpOpener == nil {
		return nil, fmt.Errorf("%w: sftp opener is not configured", domain.ErrInvalidInput)
	}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil, fmt.Errorf("%w", domain.ErrSessionClosed)
	}
	client, err := s.sftpOpener(s.client)
	if err != nil {
		return nil, fmt.Errorf("open sftp subsystem: %w", err)
	}
	return client, nil
}

// Wait blocks until the remote shell exits.
func (s *ptySession) Wait() error {
	if err := s.session.Wait(); err != nil {
		return fmt.Errorf("wait ssh session: %w", err)
	}
	return nil
}

// Close cancels keepalives and closes the SSH session and underlying client connection.
func (s *ptySession) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()

		s.cancel()
		if s.stdin != nil {
			_ = s.stdin.Close()
		}
		if s.session != nil {
			_ = s.session.Close()
		}
		if s.client != nil {
			closeErr = s.client.Close()
		}
	})
	if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) && !errors.Is(closeErr, io.EOF) {
		return fmt.Errorf("close ssh client: %w", closeErr)
	}
	return nil
}

func runSSHKeepalive(ctx context.Context, client *ssh.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				return
			}
		}
	}
}
