package sshclient_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/adapters/sshclient"
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

type testSSHServer struct {
	host        string
	port        int
	fingerprint string
	listener    net.Listener
	mu          sync.Mutex
	lastResize  [2]int // cols, rows
}

func startEmbeddedSSHServer(t *testing.T, authorizedPub ssh.PublicKey) *testSSHServer {
	t.Helper()

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("new host signer: %v", err)
	}

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), authorizedPub.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unauthorized public key")
		},
	}
	cfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}

	tcpAddr := ln.Addr().(*net.TCPAddr)
	srv := &testSSHServer{
		host:        tcpAddr.IP.String(),
		port:        tcpAddr.Port,
		fingerprint: ssh.FingerprintSHA256(hostSigner.PublicKey()),
		listener:    ln,
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
			go srv.handleConn(conn, cfg)
		}
	}()

	return srv
}

func (s *testSSHServer) handleConn(netConn net.Conn, cfg *ssh.ServerConfig) {
	defer func() { _ = netConn.Close() }()

	_, chans, reqs, err := ssh.NewServerConn(netConn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		switch newCh.ChannelType() {
		case "session":
			ch, requests, acceptErr := newCh.Accept()
			if acceptErr != nil {
				continue
			}
			go s.handleSession(ch, requests)
		case "direct-tcpip":
			go s.handleDirectTCPIP(newCh)
		default:
			_ = newCh.Reject(ssh.UnknownChannelType, "unknown channel type")
		}
	}
}

func (s *testSSHServer) handleDirectTCPIP(newCh ssh.NewChannel) {
	var payload struct {
		DestAddr   string
		DestPort   uint32
		OriginAddr string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(newCh.ExtraData(), &payload); err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, "invalid payload")
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

func (s *testSSHServer) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer func() { _ = ch.Close() }()

	for req := range reqs {
		switch req.Type {
		case "pty-req":
			_ = req.Reply(true, nil)
		case "window-change":
			if len(req.Payload) >= 8 {
				cols := int(binary.BigEndian.Uint32(req.Payload[0:4]))
				rows := int(binary.BigEndian.Uint32(req.Payload[4:8]))
				s.mu.Lock()
				s.lastResize = [2]int{cols, rows}
				s.mu.Unlock()
			}
			_ = req.Reply(true, nil)
		case "shell":
			_ = req.Reply(true, nil)
			go func() {
				_, _ = io.Copy(ch, ch)
				_ = ch.CloseWrite()
			}()
		default:
			_ = req.Reply(false, nil)
		}
	}
}

func generateClientKey(t *testing.T, passphrase []byte) ([]byte, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh.NewPublicKey: %v", err)
	}
	var block *pem.Block
	if len(passphrase) > 0 {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "test", passphrase)
	} else {
		block, err = ssh.MarshalPrivateKey(priv, "test")
	}
	if err != nil {
		t.Fatalf("MarshalPrivateKey: %v", err)
	}
	return pem.EncodeToMemory(block), sshPub
}

func TestConnectPTYResizeAndHostKeyVerification(t *testing.T) {
	rawKey, sshPub := generateClientKey(t, []byte("secret-pass"))
	srv := startEmbeddedSSHServer(t, sshPub)

	mgr := sshclient.NewManager(nil, sshclient.WithDialTimeout(5*time.Second), sshclient.WithKeepaliveInterval(100*time.Millisecond))
	host := domain.HostInstance{
		ID:          "h1",
		OwnerUserID: "u1",
		Alias:       "Embedded",
		HostAddress: srv.host,
		Port:        srv.port,
		SSHUsername: "ec2-user",
	}

	// 1. Rejecting host key aborts connection.
	_, err := mgr.ConnectPTY(context.Background(), ports.SSHConnectRequest{
		Host:          host,
		PrivateKeyPEM: rawKey,
		Passphrase:    []byte("secret-pass"),
		VerifyHostKey: func(_ context.Context, ch ports.HostKeyChallenge) error {
			if ch.Fingerprint != srv.fingerprint {
				t.Fatalf("unexpected fingerprint %q vs %q", ch.Fingerprint, srv.fingerprint)
			}
			return domain.ErrHostKeyUnknown
		},
	})
	if !errors.Is(err, domain.ErrHostKeyUnknown) {
		t.Fatalf("expected ErrHostKeyUnknown, got %v", err)
	}

	// 2. Accepting host key opens PTY, supports echo I/O and WindowChange resize.
	pty, err := mgr.ConnectPTY(context.Background(), ports.SSHConnectRequest{
		Host:          host,
		PrivateKeyPEM: rawKey,
		Passphrase:    []byte("secret-pass"),
		Cols:          100,
		Rows:          30,
		VerifyHostKey: func(_ context.Context, _ ports.HostKeyChallenge) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ConnectPTY() error = %v", err)
	}
	defer func() { _ = pty.Close() }()

	if _, err := pty.Stdin().Write([]byte("echo hello-ec2\n")); err != nil {
		t.Fatalf("write stdin: %v", err)
	}

	buf := make([]byte, 64)
	n, err := pty.Stdout().Read(buf)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "hello-ec2") {
		t.Fatalf("unexpected stdout %q", string(buf[:n]))
	}

	if err := pty.Resize(132, 42); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}
	// Give server goroutine a moment to process the window-change request.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		srv.mu.Lock()
		got := srv.lastResize
		srv.mu.Unlock()
		if got == [2]int{132, 42} {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	srv.mu.Lock()
	gotResize := srv.lastResize
	srv.mu.Unlock()
	if gotResize != [2]int{132, 42} {
		t.Fatalf("server lastResize = %v, want [132 42]", gotResize)
	}

	if err := pty.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := pty.Resize(80, 24); !errors.Is(err, domain.ErrSessionClosed) {
		t.Fatalf("Resize after Close = %v, want ErrSessionClosed", err)
	}
}

func TestConnectPTYRejectsPPKAndMissingPassphrase(t *testing.T) {
	encKey, _ := generateClientKey(t, []byte("pass"))
	mgr := sshclient.NewManager(nil)
	host := domain.HostInstance{
		HostAddress: "127.0.0.1",
		Port:        22,
		SSHUsername: "ec2-user",
	}

	tests := []struct {
		name    string
		pem     []byte
		pass    []byte
		wantErr error
	}{
		{
			name:    "ppk rejected",
			pem:     []byte("PuTTY-User-Key-File-2: ssh-rsa\n"),
			wantErr: domain.ErrUnsupportedKeyFormat,
		},
		{
			name:    "missing passphrase",
			pem:     encKey,
			wantErr: domain.ErrPassphraseRequired,
		},
		{
			name:    "no credentials",
			wantErr: domain.ErrInvalidInput,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mgr.ConnectPTY(context.Background(), ports.SSHConnectRequest{
				Host:          host,
				PrivateKeyPEM: tc.pem,
				Passphrase:    tc.pass,
				VerifyHostKey: func(_ context.Context, _ ports.HostKeyChallenge) error { return nil },
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ConnectPTY() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
	_ = strconv.Itoa(0)
}

func TestStartLocalForwardEndToEndAndBusyPort(t *testing.T) {
	// Spin up a mock private DB TCP server that echoes requests back.
	dbLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock db: %v", err)
	}
	t.Cleanup(func() { _ = dbLn.Close() })
	go func() {
		for {
			c, aErr := dbLn.Accept()
			if aErr != nil {
				return
			}
			go func(conn net.Conn) {
				defer func() { _ = conn.Close() }()
				_, _ = io.Copy(conn, conn)
			}(c)
		}
	}()
	dbAddr := dbLn.Addr().(*net.TCPAddr)

	rawKey, sshPub := generateClientKey(t, nil)
	srv := startEmbeddedSSHServer(t, sshPub)
	mgr := sshclient.NewManager(nil, sshclient.WithDialTimeout(5*time.Second))

	bastion := domain.HostInstance{
		ID:          "h-bastion",
		OwnerUserID: "u1",
		Alias:       "Bastion",
		HostAddress: srv.host,
		Port:        srv.port,
		SSHUsername: "ec2-user",
	}

	handle, err := mgr.StartLocalForward(context.Background(), ports.TunnelForwardRequest{
		Host:          bastion,
		PrivateKeyPEM: rawKey,
		LocalHost:     "127.0.0.1",
		LocalPort:     0,
		RemoteHost:    dbAddr.IP.String(),
		RemotePort:    dbAddr.Port,
		VerifyHostKey: func(_ context.Context, _ ports.HostKeyChallenge) error { return nil },
	})
	if err != nil {
		t.Fatalf("StartLocalForward() error = %v", err)
	}
	defer func() { _ = handle.Close() }()

	// Connect as a local DB client (like pgAdmin / Compass) to the forwarded port.
	clientConn, err := net.DialTimeout("tcp", handle.LocalAddr(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial local tunnel addr %s: %v", handle.LocalAddr(), err)
	}
	msg := []byte("SELECT 1;")
	if _, err := clientConn.Write(msg); err != nil {
		t.Fatalf("write through tunnel: %v", err)
	}
	resp := make([]byte, len(msg))
	if _, err := io.ReadFull(clientConn, resp); err != nil {
		t.Fatalf("read from tunnel: %v", err)
	}
	if !bytes.Equal(resp, msg) {
		t.Fatalf("tunnel response = %q, want %q", resp, msg)
	}
	_ = clientConn.Close()

	inBytes, outBytes := handle.BytesTransferred()
	if inBytes < int64(len(msg)) || outBytes < int64(len(msg)) {
		t.Fatalf("expected >=%d bytes transferred, got in=%d out=%d", len(msg), inBytes, outBytes)
	}

	// Attempt to bind the same busy local port must return ErrLocalPortInUse.
	_, busyPortStr, _ := net.SplitHostPort(handle.LocalAddr())
	busyPort, _ := strconv.Atoi(busyPortStr)
	_, err = mgr.StartLocalForward(context.Background(), ports.TunnelForwardRequest{
		Host:          bastion,
		PrivateKeyPEM: rawKey,
		LocalHost:     "127.0.0.1",
		LocalPort:     busyPort,
		RemoteHost:    dbAddr.IP.String(),
		RemotePort:    dbAddr.Port,
		VerifyHostKey: func(_ context.Context, _ ports.HostKeyChallenge) error { return nil },
	})
	if !errors.Is(err, domain.ErrLocalPortInUse) {
		t.Fatalf("busy port error = %v, want ErrLocalPortInUse", err)
	}

	if err := handle.Close(); err != nil {
		t.Fatalf("handle.Close() error = %v", err)
	}
	select {
	case <-handle.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tunnel Done channel")
	}
}
