package services_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/adapters/sftpclient"
	"ec2-shell-desk/internal/adapters/sqlite"
	"ec2-shell-desk/internal/adapters/sshclient"
	"ec2-shell-desk/internal/adapters/winfs"
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
	"ec2-shell-desk/internal/services"
)

type spyTerminalSink struct {
	mu     sync.Mutex
	output bytes.Buffer
	closed map[string]string
}

func newSpyTerminalSink() *spyTerminalSink {
	return &spyTerminalSink{closed: make(map[string]string)}
}

func (s *spyTerminalSink) OnTerminalData(_ string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.output.Write(data)
}

func (s *spyTerminalSink) OnTerminalClosed(sessionID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed[sessionID] = reason
}

func (s *spyTerminalSink) OutputString() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.output.String()
}

type spyTransferSink struct {
	mu     sync.Mutex
	events map[string][]domain.TransferTask
}

func newSpyTransferSink() *spyTransferSink {
	return &spyTransferSink{events: make(map[string][]domain.TransferTask)}
}

func (s *spyTransferSink) OnTransferProgress(task domain.TransferTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[task.ID] = append(s.events[task.ID], task)
}

func (s *spyTransferSink) FinalState(transferID string) (domain.TransferState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev := s.events[transferID]
	if len(ev) == 0 {
		return "", false
	}
	last := ev[len(ev)-1]
	if last.State == domain.TransferCompleted || last.State == domain.TransferFailed || last.State == domain.TransferCancelled {
		return last.State, true
	}
	return last.State, false
}

type fullTestSSHServer struct {
	host     string
	port     int
	mu       sync.Mutex
	hostKey  ssh.Signer
	listener net.Listener
}

func startFullTestSSHServer(t *testing.T, authorizedPub ssh.PublicKey, sftpRoot string) *fullTestSSHServer {
	t.Helper()

	hostKey := generateSigner(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	srv := &fullTestSSHServer{
		host:     tcpAddr.IP.String(),
		port:     tcpAddr.Port,
		hostKey:  hostKey,
		listener: ln,
	}
	t.Cleanup(func() {
		_ = ln.Close()
	})

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go srv.serveConn(conn, authorizedPub, sftpRoot)
		}
	}()

	return srv
}

func (s *fullTestSSHServer) RotateHostKey(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostKey = generateSigner(t)
}

func (s *fullTestSSHServer) serveConn(netConn net.Conn, authorizedPub ssh.PublicKey, sftpRoot string) {
	defer func() { _ = netConn.Close() }()

	s.mu.Lock()
	signer := s.hostKey
	s.mu.Unlock()

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), authorizedPub.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unauthorized")
		},
	}
	cfg.AddHostKey(signer)

	_, chans, reqs, err := ssh.NewServerConn(netConn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() == "direct-tcpip" {
			go serveDirectTCPIP(newCh)
			continue
		}
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "unknown")
			continue
		}
		ch, chReqs, aErr := newCh.Accept()
		if aErr != nil {
			continue
		}
		go func(channel ssh.Channel, requests <-chan *ssh.Request) {
			defer func() { _ = channel.Close() }()
			for req := range requests {
				switch req.Type {
				case "pty-req", "window-change":
					_ = req.Reply(true, nil)
				case "shell":
					_ = req.Reply(true, nil)
					go func() {
						_, _ = io.Copy(channel, channel)
						_ = channel.CloseWrite()
					}()
				case "subsystem":
					if len(req.Payload) >= 4 && string(req.Payload[4:]) == "sftp" {
						_ = req.Reply(true, nil)
						sftpSrv, sErr := sftp.NewServer(channel, sftp.WithServerWorkingDirectory(sftpRoot))
						if sErr != nil {
							return
						}
						_ = sftpSrv.Serve()
						return
					}
					_ = req.Reply(false, nil)
				default:
					_ = req.Reply(false, nil)
				}
			}
		}(ch, chReqs)
	}
}

func serveDirectTCPIP(newCh ssh.NewChannel) {
	var payload struct {
		DestAddr   string
		DestPort   uint32
		OriginAddr string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(newCh.ExtraData(), &payload); err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, "invalid direct-tcpip payload")
		return
	}
	targetConn, err := net.DialTimeout("tcp", net.JoinHostPort(payload.DestAddr, strconv.Itoa(int(payload.DestPort))), 2*time.Second)
	if err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	ch, reqs, err := newCh.Accept()
	if err != nil {
		_ = targetConn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(targetConn, ch)
		_ = targetConn.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(ch, targetConn)
		_ = ch.CloseWrite()
	}()
	wg.Wait()
	_ = ch.Close()
}

func generateSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("NewSignerFromKey: %v", err)
	}
	return signer
}

func generateClientPEMAndPub(t *testing.T) ([]byte, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("NewPublicKey: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "ec2-test")
	if err != nil {
		t.Fatalf("MarshalPrivateKey: %v", err)
	}
	return pem.EncodeToMemory(block), sshPub
}

func TestSessionTOFUSSHAndSFTPEndToEnd(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	store, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	vc := newFastCrypto(t)
	userRepo := sqlite.NewUserRepository(store)
	keyRepo := sqlite.NewKeyRepository(store)
	hostRepo := sqlite.NewHostRepository(store)
	knownRepo := sqlite.NewKnownHostRepository(store)
	tunnelRepo := sqlite.NewTunnelRepository(store)

	authSvc := services.NewAuthService(userRepo, vc, vc)
	vaultSvc := services.NewVaultService(authSvc, keyRepo, vc, vc)
	hostSvc := services.NewHostService(authSvc, hostRepo, keyRepo, nil, tunnelRepo, nil)

	termSink := newSpyTerminalSink()
	transferSink := newSpyTransferSink()

	sshMgr := sshclient.NewManager(sftpclient.NewFromSSHClient, sshclient.WithDialTimeout(5*time.Second))
	sessionSvc := services.NewSessionService(authSvc, hostRepo, knownRepo, vaultSvc, sshMgr, termSink)
	transferSvc := services.NewTransferService(authSvc, sessionSvc, winfs.NewWinFS(), transferSink)
	t.Cleanup(func() {
		transferSvc.CloseAll()
		sessionSvc.CloseAll()
	})

	// Prepare Alice and Bob.
	if _, err := authSvc.BootstrapInitialUser(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("BootstrapInitialUser: %v", err)
	}
	if _, err := authSvc.CreateUser(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("CreateUser(bob): %v", err)
	}

	rawPEM, clientPub := generateClientPEMAndPub(t)
	remoteRoot := t.TempDir()
	localRoot := t.TempDir()
	srv := startFullTestSSHServer(t, clientPub, remoteRoot)

	keySummary, err := vaultSvc.ImportKey(ctx, "ec2-demo.pem", rawPEM, nil)
	if err != nil {
		t.Fatalf("ImportKey: %v", err)
	}
	host, err := hostSvc.CreateHost(ctx, services.SaveHostInput{
		Alias:       "Demo EC2",
		HostAddress: srv.host,
		Port:        srv.port,
		SSHUsername: "ec2-user",
		SSHKeyID:    keySummary.ID,
		Environment: domain.EnvDev,
	})
	if err != nil {
		t.Fatalf("CreateHost: %v", err)
	}

	// 1. First connection without trust prompt fails with ErrHostKeyUnknown.
	if _, err := sessionSvc.OpenSession(ctx, host.ID, 80, 24, "", nil); !errors.Is(err, domain.ErrHostKeyUnknown) {
		t.Fatalf("OpenSession without TOFU callback = %v, want ErrHostKeyUnknown", err)
	}

	// 2. User accepts TOFU prompt -> session opens and persists known_host.
	promptCalls := 0
	sess, err := sessionSvc.OpenSession(ctx, host.ID, 100, 30, "", func(_ context.Context, ch ports.HostKeyChallenge) (bool, error) {
		promptCalls++
		return strings.HasPrefix(ch.Fingerprint, "SHA256:"), nil
	})
	if err != nil {
		t.Fatalf("OpenSession(TOFU accept) error = %v", err)
	}
	if promptCalls != 1 {
		t.Fatalf("expected 1 TOFU prompt, got %d", promptCalls)
	}

	// Terminal input/output + resize.
	if err := sessionSvc.WriteTerminal(sess.SessionID, []byte("uname -a\n")); err != nil {
		t.Fatalf("WriteTerminal: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(termSink.OutputString(), "uname -a") {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if !strings.Contains(termSink.OutputString(), "uname -a") {
		t.Fatalf("expected echoed terminal output, got %q", termSink.OutputString())
	}
	if err := sessionSvc.ResizeTerminal(sess.SessionID, 120, 40); err != nil {
		t.Fatalf("ResizeTerminal: %v", err)
	}

	// 3. SFTP upload & download via TransferService.
	uploadPayload := bytes.Repeat([]byte("XFTP-PAYLOAD-"), 4096)
	localUploadFile := filepath.Join(localRoot, "report.txt")
	if err := os.WriteFile(localUploadFile, uploadPayload, 0o600); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	remoteUploadFile := "report.txt"

	upTask, err := transferSvc.StartUpload(sess.SessionID, localUploadFile, remoteUploadFile)
	if err != nil {
		t.Fatalf("StartUpload: %v", err)
	}
	waitTransferDone(t, transferSink, upTask.ID, domain.TransferCompleted)

	localDownloadFile := filepath.Join(localRoot, "report-copy.txt")
	downTask, err := transferSvc.StartDownload(sess.SessionID, remoteUploadFile, localDownloadFile)
	if err != nil {
		t.Fatalf("StartDownload: %v", err)
	}
	waitTransferDone(t, transferSink, downTask.ID, domain.TransferCompleted)

	gotDownload, err := os.ReadFile(localDownloadFile)
	if err != nil || !bytes.Equal(gotDownload, uploadPayload) {
		t.Fatalf("downloaded file mismatch: err=%v", err)
	}

	// 3b. Recursive directory upload via TransferService.
	localBundleDir := filepath.Join(localRoot, "bundle")
	if err := os.MkdirAll(filepath.Join(localBundleDir, "nested"), 0o750); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localBundleDir, "nested", "app.conf"), []byte("port=8080"), 0o600); err != nil {
		t.Fatalf("write nested conf: %v", err)
	}
	dirTask, err := transferSvc.StartUploadDirectory(sess.SessionID, localBundleDir, "bundle")
	if err != nil {
		t.Fatalf("StartUploadDirectory: %v", err)
	}
	waitTransferDone(t, transferSink, dirTask.ID, domain.TransferCompleted)
	if gotConf, err := os.ReadFile(filepath.Join(remoteRoot, "bundle", "nested", "app.conf")); err != nil || string(gotConf) != "port=8080" {
		t.Fatalf("remote nested conf = (%q, %v)", string(gotConf), err)
	}

	// 4. Multi-user isolation: Bob logs in and cannot access Alice's session or transfer.
	if _, err := authSvc.Login(ctx, "bob", "BobPass!2026"); err != nil {
		t.Fatalf("Login(bob): %v", err)
	}
	if bobSessions, err := sessionSvc.ListSessions(); err != nil || len(bobSessions) != 0 {
		t.Fatalf("Bob ListSessions = (%d, %v), want (0, nil)", len(bobSessions), err)
	}
	if err := sessionSvc.WriteTerminal(sess.SessionID, []byte("rm -rf /\n")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob WriteTerminal = %v, want ErrNotFound", err)
	}
	if _, err := transferSvc.ListRemoteDir(ctx, sess.SessionID, "."); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob ListRemoteDir = %v, want ErrNotFound", err)
	}
	if err := transferSvc.CancelTransfer(upTask.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Bob CancelTransfer = %v, want ErrNotFound", err)
	}

	// Switch back to Alice -> second connection auto-trusts known_host without prompting.
	if _, err := authSvc.Login(ctx, "alice", "AlicePass!2026"); err != nil {
		t.Fatalf("Login(alice): %v", err)
	}
	sess2, err := sessionSvc.OpenSession(ctx, host.ID, 80, 24, "", func(_ context.Context, _ ports.HostKeyChallenge) (bool, error) {
		t.Fatal("TOFU prompt should not be invoked for already-trusted host")
		return false, nil
	})
	if err != nil {
		t.Fatalf("OpenSession(trusted) error = %v", err)
	}
	_ = sessionSvc.CloseSession(sess.SessionID)
	_ = sessionSvc.CloseSession(sess2.SessionID)

	// 5. Host key rotation triggers ErrHostKeyMismatch!
	srv.RotateHostKey(t)
	if _, err := sessionSvc.OpenSession(ctx, host.ID, 80, 24, "", nil); !errors.Is(err, domain.ErrHostKeyMismatch) {
		t.Fatalf("OpenSession(rotated host key) = %v, want ErrHostKeyMismatch", err)
	}
}

func waitTransferDone(t *testing.T, sink *spyTransferSink, transferID string, want domain.TransferState) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if state, done := sink.FinalState(transferID); done {
			if state != want {
				t.Fatalf("transfer %s finished with state %q, want %q", transferID, state, want)
			}
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for transfer %s", transferID)
}
