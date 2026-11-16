package sftpclient_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/adapters/sftpclient"
	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

func startEmbeddedSFTPServer(t *testing.T, rootDir string) ports.SFTPClient {
	t.Helper()

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("new host signer: %v", err)
	}

	srvCfg := &ssh.ServerConfig{NoClientAuth: true}
	srvCfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
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
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_, chans, reqs, sErr := ssh.NewServerConn(c, srvCfg)
				if sErr != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for newCh := range chans {
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
							if req.Type == "subsystem" && len(req.Payload) >= 4 && string(req.Payload[4:]) == "sftp" {
								_ = req.Reply(true, nil)
								srv, err := sftp.NewServer(channel, sftp.WithServerWorkingDirectory(rootDir))
								if err != nil {
									return
								}
								if serveErr := srv.Serve(); serveErr != nil && !errors.Is(serveErr, io.EOF) {
									return
								}
								return
							}
							_ = req.Reply(false, nil)
						}
					}(ch, chReqs)
				}
			}(conn)
		}
	}()

	clientCfg := &ssh.ClientConfig{
		User:            "ec2-user",
		HostKeyCallback: ssh.FixedHostKey(hostSigner.PublicKey()),
		Timeout:         5 * time.Second,
	}
	sshConn, err := ssh.Dial("tcp", ln.Addr().String(), clientCfg)
	if err != nil {
		t.Fatalf("ssh.Dial: %v", err)
	}
	t.Cleanup(func() {
		_ = sshConn.Close()
	})

	client, err := sftpclient.NewFromSSHClient(sshConn)
	if err != nil {
		t.Fatalf("NewFromSSHClient: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func TestSFTPDirectoryAndTransferLifecycle(t *testing.T) {
	ctx := context.Background()
	remoteRoot := t.TempDir()
	localRoot := t.TempDir()
	client := startEmbeddedSFTPServer(t, remoteRoot)

	wd, err := client.WorkingDir(ctx)
	if err != nil || wd == "" {
		t.Fatalf("WorkingDir() = (%q, %v)", wd, err)
	}

	remoteSubdir := "uploads"
	if err := client.Mkdir(ctx, remoteSubdir); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	// Upload a 256KB payload and assert progress callbacks fire.
	payload := bytes.Repeat([]byte("EC2-SHELL-DESK-SFTP-"), 13*1024)
	localUpload := filepath.Join(localRoot, "artifact.bin")
	if err := os.WriteFile(localUpload, payload, 0o600); err != nil {
		t.Fatalf("write local upload file: %v", err)
	}

	remoteFile := remoteSubdir + "/artifact.bin"
	var lastTransferred, lastTotal int64
	err = client.Upload(ctx, localUpload, remoteFile, func(transferred, total int64) {
		lastTransferred = transferred
		lastTotal = total
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if lastTransferred != int64(len(payload)) || lastTotal != int64(len(payload)) {
		t.Fatalf("progress = (%d, %d), want (%d, %d)", lastTransferred, lastTotal, len(payload), len(payload))
	}

	// List directory and confirm entry metadata.
	entries, err := client.ListDir(ctx, remoteSubdir)
	if err != nil || len(entries) != 1 || entries[0].Name != "artifact.bin" {
		t.Fatalf("ListDir() = (%+v, %v)", entries, err)
	}

	// Rename remote file.
	renamedRemote := remoteSubdir + "/artifact-v2.bin"
	if err := client.Rename(ctx, remoteFile, renamedRemote); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Download and verify byte equality.
	localDownload := filepath.Join(localRoot, "downloaded.bin")
	if err := client.Download(ctx, renamedRemote, localDownload, nil); err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	gotBytes, err := os.ReadFile(localDownload)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if !bytes.Equal(gotBytes, payload) {
		t.Fatal("downloaded payload does not match uploaded payload")
	}

	// Remove recursively.
	if err := client.Remove(ctx, remoteSubdir, true); err != nil {
		t.Fatalf("Remove(recursive) error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(remoteRoot, "uploads")); !os.IsNotExist(statErr) {
		t.Fatalf("expected remote directory to be deleted, statErr=%v", statErr)
	}
}

func TestSFTPCancellationCleansUpPartialDownload(t *testing.T) {
	remoteRoot := t.TempDir()
	localRoot := t.TempDir()
	client := startEmbeddedSFTPServer(t, remoteRoot)

	// Create a ~2MB remote file so mid-transfer cancellation triggers cleanly.
	payload := bytes.Repeat([]byte("A"), 2*1024*1024)
	remoteFile := "large.bin"
	if err := os.WriteFile(filepath.Join(remoteRoot, "large.bin"), payload, 0o600); err != nil {
		t.Fatalf("write remote large file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	localTarget := filepath.Join(localRoot, "partial.bin")
	err := client.Download(ctx, remoteFile, localTarget, func(transferred, _ int64) {
		if transferred > 0 {
			cancel()
		}
	})
	if !errors.Is(err, domain.ErrTransferCancelled) {
		t.Fatalf("Download(cancelled) error = %v, want ErrTransferCancelled", err)
	}
	if _, statErr := os.Stat(localTarget); !os.IsNotExist(statErr) {
		t.Fatalf("expected partial local file %q to be deleted on cancel", localTarget)
	}
}
