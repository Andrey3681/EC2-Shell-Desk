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
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		ch, requests, acceptErr := newCh.Accept()
		if acceptErr != nil {
			continue
		}
		go s.handleSession(ch, requests)
	}
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
