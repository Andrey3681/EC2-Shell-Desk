package sshclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

// StartLocalForward establishes an SSH bastion connection and listens on a local TCP port,
// forwarding each connection to the target remote endpoint over the SSH tunnel.
func (m *Manager) StartLocalForward(ctx context.Context, req ports.TunnelForwardRequest) (ports.TunnelHandle, error) {
	if strings.TrimSpace(req.RemoteHost) == "" || req.RemotePort < 1 || req.RemotePort > 65535 {
		return nil, fmt.Errorf("%w: valid remote host and port are required", domain.ErrInvalidInput)
	}

	localHost := strings.TrimSpace(req.LocalHost)
	if localHost == "" {
		localHost = domain.DefaultTunnelLocalHost
	}
	if req.LocalPort < 0 || req.LocalPort > 65535 {
		return nil, fmt.Errorf("%w: local port must be between 0 and 65535", domain.ErrInvalidInput)
	}

	client, err := m.dialSSHClient(ctx, req.Host, req.PrivateKeyPEM, req.Passphrase, req.Password, req.VerifyHostKey)
	if err != nil {
		return nil, err
	}

	bindAddr := net.JoinHostPort(localHost, strconv.Itoa(req.LocalPort))
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		_ = client.Close()
		if isAddrInUseError(err) {
			return nil, fmt.Errorf("%w: %s", domain.ErrLocalPortInUse, bindAddr)
		}
		return nil, fmt.Errorf("listen on local tunnel address %s: %w", bindAddr, err)
	}

	tunnelCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	handle := &tunnelHandle{
		id:         uuid.NewString(),
		localAddr:  listener.Addr().String(),
		remoteAddr: net.JoinHostPort(strings.TrimSpace(req.RemoteHost), strconv.Itoa(req.RemotePort)),
		client:     client,
		listener:   listener,
		onTraffic:  req.OnTraffic,
		cancel:     cancel,
		done:       make(chan struct{}),
		conns:      make(map[net.Conn]struct{}),
	}

	go runSSHKeepalive(tunnelCtx, client, m.keepaliveInterval)
	go handle.watchSSHClose(tunnelCtx)
	go handle.acceptLoop(tunnelCtx)

	return handle, nil
}

type tunnelHandle struct {
	id         string
	localAddr  string
	remoteAddr string
	client     *ssh.Client
	listener   net.Listener
	onTraffic  func(activeConns int, bytesIn, bytesOut int64)

	cancel    context.CancelFunc
	closeOnce sync.Once
	done      chan struct{}

	activeConns atomic.Int32
	bytesIn     atomic.Int64
	bytesOut    atomic.Int64

	errMu   sync.Mutex
	lastErr error

	connsMu sync.Mutex
	conns   map[net.Conn]struct{}
	wg      sync.WaitGroup
}

// ID returns the unique identifier of this active port-forwarding tunnel.
func (t *tunnelHandle) ID() string {
	return t.id
}

// LocalAddr returns the bound local host:port listener address.
func (t *tunnelHandle) LocalAddr() string {
	return t.localAddr
}

// ActiveConnections returns the number of currently open forwarded TCP connections.
func (t *tunnelHandle) ActiveConnections() int {
	return int(t.activeConns.Load())
}

// BytesTransferred returns cumulative inbound and outbound bytes forwarded through this tunnel.
func (t *tunnelHandle) BytesTransferred() (int64, int64) {
	return t.bytesIn.Load(), t.bytesOut.Load()
}

// Done returns a channel that is closed once the tunnel listener and SSH client have shut down.
func (t *tunnelHandle) Done() <-chan struct{} {
	return t.done
}

// Err returns the first terminal error encountered by the tunnel, if any.
func (t *tunnelHandle) Err() error {
	t.errMu.Lock()
	defer t.errMu.Unlock()
	return t.lastErr
}

// Close stops accepting connections, closes active forwarded sockets and the SSH client, and waits for goroutines.
func (t *tunnelHandle) Close() error {
	var closeErr error
	t.closeOnce.Do(func() {
		t.cancel()
		if t.listener != nil {
			_ = t.listener.Close()
		}

		t.connsMu.Lock()
		for conn := range t.conns {
			_ = conn.Close()
		}
		t.connsMu.Unlock()

		if t.client != nil {
			err := t.client.Close()
			if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
				closeErr = fmt.Errorf("close tunnel ssh client: %w", err)
			}
		}

		t.wg.Wait()
		close(t.done)
	})
	return closeErr
}

func (t *tunnelHandle) watchSSHClose(ctx context.Context) {
	waitErr := t.client.Wait()
	select {
	case <-ctx.Done():
		return
	default:
		if waitErr != nil {
			t.setErr(fmt.Errorf("ssh tunnel connection closed: %w", waitErr))
		}
		_ = t.Close()
	}
}

func (t *tunnelHandle) acceptLoop(ctx context.Context) {
	for {
		localConn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
			default:
				if !errors.Is(err, net.ErrClosed) {
					t.setErr(fmt.Errorf("accept local tunnel connection: %w", err))
				}
			}
			return
		}

		t.wg.Add(1)
		go t.forwardConn(ctx, localConn)
	}
}

func (t *tunnelHandle) forwardConn(ctx context.Context, localConn net.Conn) {
	defer t.wg.Done()
	t.trackConn(localConn)
	defer t.untrackConn(localConn)
	defer func() { _ = localConn.Close() }()

	remoteConn, err := t.client.Dial("tcp", t.remoteAddr)
	if err != nil {
		return
	}
	t.trackConn(remoteConn)
	defer t.untrackConn(remoteConn)
	defer func() { _ = remoteConn.Close() }()

	t.activeConns.Add(1)
	t.notifyTraffic()
	defer func() {
		t.activeConns.Add(-1)
		t.notifyTraffic()
	}()

	var copyWG sync.WaitGroup
	copyWG.Add(2)

	// Local -> Remote (bytesOut)
	go func() {
		defer copyWG.Done()
		_, _ = io.Copy(&countingWriter{dst: remoteConn, counter: &t.bytesOut, onChunk: t.notifyTraffic}, localConn)
		closeWrite(remoteConn)
	}()

	// Remote -> Local (bytesIn)
	go func() {
		defer copyWG.Done()
		_, _ = io.Copy(&countingWriter{dst: localConn, counter: &t.bytesIn, onChunk: t.notifyTraffic}, remoteConn)
		closeWrite(localConn)
	}()

	doneCh := make(chan struct{})
	go func() {
		copyWG.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-ctx.Done():
		_ = localConn.Close()
		_ = remoteConn.Close()
		<-doneCh
	}
}

func (t *tunnelHandle) trackConn(conn net.Conn) {
	t.connsMu.Lock()
	t.conns[conn] = struct{}{}
	t.connsMu.Unlock()
}

func (t *tunnelHandle) untrackConn(conn net.Conn) {
	t.connsMu.Lock()
	delete(t.conns, conn)
	t.connsMu.Unlock()
}

func (t *tunnelHandle) notifyTraffic() {
	if t.onTraffic != nil {
		t.onTraffic(int(t.activeConns.Load()), t.bytesIn.Load(), t.bytesOut.Load())
	}
}

func (t *tunnelHandle) setErr(err error) {
	t.errMu.Lock()
	if t.lastErr == nil {
		t.lastErr = err
	}
	t.errMu.Unlock()
}

type countingWriter struct {
	dst     io.Writer
	counter *atomic.Int64
	onChunk func()
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if n > 0 {
		w.counter.Add(int64(n))
		if w.onChunk != nil {
			w.onChunk()
		}
	}
	if err != nil {
		return n, fmt.Errorf("forward tunnel bytes: %w", err)
	}
	return n, nil
}

func closeWrite(conn net.Conn) {
	type closeWriter interface {
		CloseWrite() error
	}
	if cw, ok := conn.(closeWriter); ok {
		_ = cw.CloseWrite()
	}
}

func isAddrInUseError(err error) bool {
	if err == nil {
		return false
	}
	const windowsWSAEAddrInUse = syscall.Errno(10048)
	if errors.Is(err, syscall.EADDRINUSE) || errors.Is(err, windowsWSAEAddrInUse) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") || strings.Contains(msg, "only one usage of each socket address")
}
