package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const ptyReadBufferSize = 32 * 1024

// TerminalEventSink receives asynchronous PTY output and closure notifications for the UI.
type TerminalEventSink interface {
	OnTerminalData(sessionID string, data []byte)
	OnTerminalClosed(sessionID string, reason string)
}

// TOFUTrustPromptFunc prompts the caller to accept or reject an unseen SSH server public key.
type TOFUTrustPromptFunc func(ctx context.Context, challenge ports.HostKeyChallenge) (bool, error)

type activeSession struct {
	info       domain.SessionInfo
	pty        ports.PTYSession
	sftpMu     sync.Mutex
	sftpClient ports.SFTPClient
	cancel     context.CancelFunc
}

// SessionService orchestrates authenticated SSH PTY connections, TOFU known_hosts checks, and SFTP channel access.
type SessionService struct {
	auth       *AuthService
	hosts      ports.HostRepository
	knownHosts ports.KnownHostRepository
	vault      *VaultService
	sshManager ports.SSHSessionManager
	sink       TerminalEventSink

	mu       sync.RWMutex
	sessions map[string]*activeSession
}

// NewSessionService constructs a SessionService orchestrating SSH sessions for the active user.
func NewSessionService(
	auth *AuthService,
	hosts ports.HostRepository,
	knownHosts ports.KnownHostRepository,
	vault *VaultService,
	sshManager ports.SSHSessionManager,
	sink TerminalEventSink,
) *SessionService {
	return &SessionService{
		auth:       auth,
		hosts:      hosts,
		knownHosts: knownHosts,
		vault:      vault,
		sshManager: sshManager,
		sink:       sink,
		sessions:   make(map[string]*activeSession),
	}
}

// OpenSession connects via SSH to a user-owned host, enforces TOFU host key verification, and opens a PTY shell.
func (s *SessionService) OpenSession(
	ctx context.Context,
	hostID string,
	cols, rows int,
	passwordFallback string,
	onTrust TOFUTrustPromptFunc,
) (domain.SessionInfo, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.SessionInfo{}, err
	}

	host, err := s.hosts.GetByID(ctx, user.ID, hostID)
	if err != nil {
		return domain.SessionInfo{}, fmt.Errorf("lookup host for ssh session: %w", err)
	}

	var rawPEM, passphrase []byte
	if host.SSHKeyID != "" {
		rawPEM, passphrase, err = s.vault.DecryptKeyForSession(ctx, host.SSHKeyID)
		if err != nil {
			return domain.SessionInfo{}, fmt.Errorf("unlock host ssh key: %w", err)
		}
		defer wipeBytes(rawPEM)
		defer wipeBytes(passphrase)
	}

	pty, err := s.sshManager.ConnectPTY(ctx, ports.SSHConnectRequest{
		Host:          host,
		PrivateKeyPEM: rawPEM,
		Passphrase:    passphrase,
		Password:      passwordFallback,
		Cols:          cols,
		Rows:          rows,
		VerifyHostKey: s.buildHostKeyVerifier(user.ID, onTrust),
	})
	if err != nil {
		return domain.SessionInfo{}, fmt.Errorf("connect ssh pty: %w", err)
	}

	now := time.Now().UTC()
	_ = s.hosts.TouchLastConnected(ctx, user.ID, host.ID, now)

	streamCtx, cancelStream := context.WithCancel(context.WithoutCancel(ctx))
	info := domain.SessionInfo{
		SessionID:   pty.ID(),
		OwnerUserID: user.ID,
		HostID:      host.ID,
		HostAlias:   host.Alias,
		HostAddress: host.HostAddress,
		Port:        host.Port,
		SSHUsername: host.SSHUsername,
		ConnectedAt: now,
	}

	entry := &activeSession{
		info:   info,
		pty:    pty,
		cancel: cancelStream,
	}

	s.mu.Lock()
	s.sessions[info.SessionID] = entry
	s.mu.Unlock()

	go s.streamOutput(streamCtx, entry)
	return info, nil
}

// WriteTerminal sends user input bytes to an active PTY session owned by the current user.
func (s *SessionService) WriteTerminal(sessionID string, data []byte) error {
	entry, err := s.ownedSession(sessionID)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if _, err := entry.pty.Stdin().Write(data); err != nil {
		return fmt.Errorf("write terminal stdin: %w", err)
	}
	return nil
}

// ResizeTerminal adjusts the remote PTY window size for an owned active session.
func (s *SessionService) ResizeTerminal(sessionID string, cols, rows int) error {
	entry, err := s.ownedSession(sessionID)
	if err != nil {
		return err
	}
	if err := entry.pty.Resize(cols, rows); err != nil {
		return fmt.Errorf("resize terminal: %w", err)
	}
	return nil
}

// CloseSession closes an active SSH/SFTP session owned by the current user.
func (s *SessionService) CloseSession(sessionID string) error {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	s.mu.Lock()
	entry, ok := s.sessions[sessionID]
	if !ok || entry.info.OwnerUserID != user.ID {
		s.mu.Unlock()
		return fmt.Errorf("%w: session", domain.ErrNotFound)
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()

	return s.teardownEntry(entry)
}

// CloseAll terminates all active SSH and SFTP sessions (typically on vault lock or app shutdown).
func (s *SessionService) CloseAll() {
	s.mu.Lock()
	entries := make([]*activeSession, 0, len(s.sessions))
	for id, entry := range s.sessions {
		entries = append(entries, entry)
		delete(s.sessions, id)
	}
	s.mu.Unlock()

	for _, entry := range entries {
		_ = s.teardownEntry(entry)
	}
}

// ListSessions returns active SSH session metadata belonging to the authenticated user.
func (s *SessionService) ListSessions() ([]domain.SessionInfo, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]domain.SessionInfo, 0)
	for _, entry := range s.sessions {
		if entry.info.OwnerUserID == user.ID {
			list = append(list, entry.info)
		}
	}
	return list, nil
}

// GetSFTPClient returns (lazily opening if needed) the SFTP client for an owned active session.
func (s *SessionService) GetSFTPClient(sessionID string) (ports.SFTPClient, error) {
	entry, err := s.ownedSession(sessionID)
	if err != nil {
		return nil, err
	}

	entry.sftpMu.Lock()
	defer entry.sftpMu.Unlock()

	if entry.sftpClient != nil {
		return entry.sftpClient, nil
	}
	client, err := entry.pty.OpenSFTP()
	if err != nil {
		return nil, fmt.Errorf("open session sftp client: %w", err)
	}
	entry.sftpClient = client
	return client, nil
}

// ListKnownHosts returns TOFU-verified remote hosts belonging to the authenticated user.
func (s *SessionService) ListKnownHosts(ctx context.Context) ([]domain.KnownHost, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}
	hosts, err := s.knownHosts.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list known hosts: %w", err)
	}
	return hosts, nil
}

// RemoveKnownHost deletes a trusted host fingerprint belonging to the authenticated user.
func (s *SessionService) RemoveKnownHost(ctx context.Context, knownHostID string) error {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}
	if err := s.knownHosts.Delete(ctx, user.ID, knownHostID); err != nil {
		return fmt.Errorf("delete known host: %w", err)
	}
	return nil
}

func (s *SessionService) ownedSession(sessionID string) (*activeSession, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok || entry.info.OwnerUserID != user.ID {
		return nil, fmt.Errorf("%w: session", domain.ErrNotFound)
	}
	return entry, nil
}

func (s *SessionService) buildHostKeyVerifier(ownerUserID string, onTrust TOFUTrustPromptFunc) ports.HostKeyVerifier {
	return newTOFUHostKeyVerifier(s.knownHosts, ownerUserID, onTrust)
}

func newTOFUHostKeyVerifier(
	knownHosts ports.KnownHostRepository,
	ownerUserID string,
	onTrust TOFUTrustPromptFunc,
) ports.HostKeyVerifier {
	return func(ctx context.Context, challenge ports.HostKeyChallenge) error {
		known, err := knownHosts.GetByHostPort(ctx, ownerUserID, challenge.HostAddress, challenge.Port)
		if err == nil {
			if bytes.Equal(known.PublicKeyBlob, challenge.PublicKeyBlob) {
				return nil
			}
			return fmt.Errorf("%w: expected %s, got %s", domain.ErrHostKeyMismatch, known.Fingerprint, challenge.Fingerprint)
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("lookup known host: %w", err)
		}

		if onTrust == nil {
			return fmt.Errorf("%w: %s", domain.ErrHostKeyUnknown, challenge.Fingerprint)
		}
		trusted, promptErr := onTrust(ctx, challenge)
		if promptErr != nil {
			return fmt.Errorf("host key trust prompt: %w", promptErr)
		}
		if !trusted {
			return fmt.Errorf("%w: user rejected fingerprint %s", domain.ErrHostKeyUnknown, challenge.Fingerprint)
		}

		record := domain.KnownHost{
			ID:            uuid.NewString(),
			OwnerUserID:   ownerUserID,
			HostAddress:   challenge.HostAddress,
			Port:          challenge.Port,
			KeyType:       challenge.KeyType,
			Fingerprint:   challenge.Fingerprint,
			PublicKeyBlob: append([]byte(nil), challenge.PublicKeyBlob...),
			TrustedAt:     time.Now().UTC(),
		}
		if upsertErr := knownHosts.Upsert(ctx, record); upsertErr != nil {
			return fmt.Errorf("persist trusted host key: %w", upsertErr)
		}
		return nil
	}
}

func (s *SessionService) streamOutput(ctx context.Context, entry *activeSession) {
	var wg sync.WaitGroup
	pipeReader := func(r io.Reader) {
		defer wg.Done()
		buf := make([]byte, ptyReadBufferSize)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := r.Read(buf)
			if n > 0 && s.sink != nil {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				s.sink.OnTerminalData(entry.info.SessionID, chunk)
			}
			if err != nil {
				return
			}
		}
	}

	wg.Add(2)
	go pipeReader(entry.pty.Stdout())
	go pipeReader(entry.pty.Stderr())

	waitErr := entry.pty.Wait()
	wg.Wait()

	s.mu.Lock()
	_, stillActive := s.sessions[entry.info.SessionID]
	if stillActive {
		delete(s.sessions, entry.info.SessionID)
	}
	s.mu.Unlock()

	_ = s.teardownEntry(entry)

	if stillActive && s.sink != nil {
		reason := "closed"
		if waitErr != nil {
			reason = waitErr.Error()
		}
		s.sink.OnTerminalClosed(entry.info.SessionID, reason)
	}
}

func (s *SessionService) teardownEntry(entry *activeSession) error {
	entry.cancel()

	entry.sftpMu.Lock()
	sftpCli := entry.sftpClient
	entry.sftpClient = nil
	entry.sftpMu.Unlock()

	if sftpCli != nil {
		_ = sftpCli.Close()
	}
	if err := entry.pty.Close(); err != nil {
		return fmt.Errorf("close pty session: %w", err)
	}
	return nil
}
