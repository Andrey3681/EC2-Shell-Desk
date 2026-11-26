package sftpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const transferChunkSize = 64 * 1024

// Client implements ports.SFTPClient backed by github.com/pkg/sftp.
type Client struct {
	sftp *sftp.Client
}

// NewFromSSHClient opens a new SFTP subsystem client over an active SSH connection.
func NewFromSSHClient(sshClient *ssh.Client) (ports.SFTPClient, error) {
	if sshClient == nil {
		return nil, fmt.Errorf("%w: ssh client is nil", domain.ErrInvalidInput)
	}
	sc, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("create sftp client: %w", err)
	}
	return &Client{sftp: sc}, nil
}

// WorkingDir returns the remote user's current working directory.
func (c *Client) WorkingDir(ctx context.Context) (string, error) {
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	wd, err := c.sftp.Getwd()
	if err != nil {
		return "", fmt.Errorf("get remote working directory: %w", err)
	}
	return wd, nil
}

// ListDir lists files and subdirectories at remotePath sorted directories-first then alphabetically.
func (c *Client) ListDir(ctx context.Context, remotePath string) ([]domain.FileEntry, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	cleanPath, err := sanitizeRemotePath(remotePath)
	if err != nil {
		return nil, err
	}

	infos, err := c.sftp.ReadDir(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read remote directory %q: %w", cleanPath, err)
	}

	entries := make([]domain.FileEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, domain.FileEntry{
			Name:       info.Name(),
			Path:       path.Join(cleanPath, info.Name()),
			Size:       info.Size(),
			IsDir:      info.IsDir(),
			Permission: info.Mode().String(),
			ModifiedAt: info.ModTime().UTC(),
		})
	}

	sortFileEntries(entries)
	return entries, nil
}

// Upload streams a local Windows file to remotePath while reporting progress and honoring cancellation.
func (c *Client) Upload(ctx context.Context, localPath, remotePath string, onProgress ports.TransferProgressFunc) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	cleanLocal, err := sanitizeLocalPath(localPath)
	if err != nil {
		return err
	}
	cleanRemote, err := sanitizeRemotePath(remotePath)
	if err != nil {
		return err
	}

	// #nosec G304 -- cleanLocal is sanitized and chosen by the authenticated local desktop user.
	src, err := os.Open(cleanLocal)
	if err != nil {
		return fmt.Errorf("open local file for upload: %w", err)
	}
	defer func() {
		_ = src.Close()
	}()

	stat, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}
	if stat.IsDir() {
		return fmt.Errorf("%w: upload source is a directory", domain.ErrInvalidInput)
	}

	dst, err := c.sftp.OpenFile(cleanRemote, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("open remote destination %q: %w", cleanRemote, err)
	}

	copyErr := streamWithProgress(ctx, dst, src, stat.Size(), onProgress)
	closeErr := dst.Close()

	if copyErr != nil {
		_ = c.sftp.Remove(cleanRemote)
		return copyErr
	}
	if closeErr != nil {
		return fmt.Errorf("close remote upload file: %w", closeErr)
	}
	return nil
}

type queuedUploadFile struct {
	localPath  string
	remotePath string
	size       int64
}

// UploadDir recursively uploads a local directory and all its nested files to remoteTargetDir while reporting aggregated byte progress.
func (c *Client) UploadDir(ctx context.Context, localDirPath, remoteTargetDir string, onProgress ports.TransferProgressFunc) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	cleanLocal, err := sanitizeLocalPath(localDirPath)
	if err != nil {
		return err
	}
	cleanRemote, err := sanitizeRemotePath(remoteTargetDir)
	if err != nil {
		return err
	}

	rootInfo, err := os.Stat(cleanLocal)
	if err != nil {
		return fmt.Errorf("stat local directory %q: %w", cleanLocal, err)
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("%w: upload directory source is not a directory", domain.ErrInvalidInput)
	}

	dirs := make([]string, 0)
	files := make([]queuedUploadFile, 0)
	var totalBytes int64

	walkErr := filepath.WalkDir(cleanLocal, func(currentPath string, d os.DirEntry, wErr error) error {
		if wErr != nil {
			return wErr
		}
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return ctxErr
		}
		rel, relErr := filepath.Rel(cleanLocal, currentPath)
		if relErr != nil {
			return fmt.Errorf("compute relative path for %q: %w", currentPath, relErr)
		}
		if rel == "." {
			return nil
		}
		remoteEntryPath := path.Join(cleanRemote, filepath.ToSlash(rel))
		if d.IsDir() {
			dirs = append(dirs, remoteEntryPath)
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return fmt.Errorf("stat local entry %q: %w", currentPath, infoErr)
		}
		totalBytes += info.Size()
		files = append(files, queuedUploadFile{
			localPath:  currentPath,
			remotePath: remoteEntryPath,
			size:       info.Size(),
		})
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("walk local directory %q: %w", cleanLocal, walkErr)
	}

	if onProgress != nil {
		onProgress(0, totalBytes)
	}

	if err := c.sftp.MkdirAll(cleanRemote); err != nil {
		return fmt.Errorf("create remote root directory %q: %w", cleanRemote, err)
	}
	for _, dirPath := range dirs {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if err := c.sftp.MkdirAll(dirPath); err != nil {
			return fmt.Errorf("create remote subdirectory %q: %w", dirPath, err)
		}
	}

	var uploadedBefore int64
	for _, f := range files {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if err := c.uploadSingleInDir(ctx, f, uploadedBefore, totalBytes, onProgress); err != nil {
			return err
		}
		uploadedBefore += f.size
	}

	if onProgress != nil {
		onProgress(totalBytes, totalBytes)
	}
	return nil
}

func (c *Client) uploadSingleInDir(
	ctx context.Context,
	f queuedUploadFile,
	uploadedBefore int64,
	totalBytes int64,
	onProgress ports.TransferProgressFunc,
) error {
	// #nosec G304 -- f.localPath comes from sanitized filepath.WalkDir under a user-selected directory.
	src, err := os.Open(f.localPath)
	if err != nil {
		return fmt.Errorf("open local file %q: %w", f.localPath, err)
	}
	defer func() {
		_ = src.Close()
	}()

	dst, err := c.sftp.OpenFile(f.remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("open remote file %q: %w", f.remotePath, err)
	}

	copyErr := streamWithProgress(ctx, dst, src, f.size, func(fileBytes, _ int64) {
		if onProgress != nil {
			onProgress(uploadedBefore+fileBytes, totalBytes)
		}
	})
	closeErr := dst.Close()

	if copyErr != nil {
		_ = c.sftp.Remove(f.remotePath)
		return copyErr
	}
	if closeErr != nil {
		return fmt.Errorf("close remote upload file %q: %w", f.remotePath, closeErr)
	}
	return nil
}

// Download streams a remote file to localPath while reporting progress and cleaning up partial files on cancellation.
func (c *Client) Download(ctx context.Context, remotePath, localPath string, onProgress ports.TransferProgressFunc) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	cleanRemote, err := sanitizeRemotePath(remotePath)
	if err != nil {
		return err
	}
	cleanLocal, err := sanitizeLocalPath(localPath)
	if err != nil {
		return err
	}

	src, err := c.sftp.Open(cleanRemote)
	if err != nil {
		return fmt.Errorf("open remote file %q: %w", cleanRemote, err)
	}
	defer func() {
		_ = src.Close()
	}()

	stat, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat remote file: %w", err)
	}
	if stat.IsDir() {
		return fmt.Errorf("%w: download source is a directory", domain.ErrInvalidInput)
	}

	if mkErr := os.MkdirAll(filepath.Dir(cleanLocal), 0o750); mkErr != nil {
		return fmt.Errorf("create local parent directory: %w", mkErr)
	}

	// #nosec G304 -- cleanLocal is sanitized and chosen by the authenticated local desktop user.
	dst, err := os.Create(cleanLocal)
	if err != nil {
		return fmt.Errorf("create local file %q: %w", cleanLocal, err)
	}

	copyErr := streamWithProgress(ctx, dst, src, stat.Size(), onProgress)
	closeErr := dst.Close()

	if copyErr != nil {
		_ = os.Remove(cleanLocal)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(cleanLocal)
		return fmt.Errorf("close local download file: %w", closeErr)
	}
	return nil
}

// Mkdir creates a directory at remotePath.
func (c *Client) Mkdir(ctx context.Context, remotePath string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	cleanPath, err := sanitizeRemotePath(remotePath)
	if err != nil {
		return err
	}
	if err := c.sftp.Mkdir(cleanPath); err != nil {
		return fmt.Errorf("create remote directory %q: %w", cleanPath, err)
	}
	return nil
}

// MkdirAll creates a directory at remotePath along with any necessary parents.
func (c *Client) MkdirAll(ctx context.Context, remotePath string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	cleanPath, err := sanitizeRemotePath(remotePath)
	if err != nil {
		return err
	}
	if err := c.sftp.MkdirAll(cleanPath); err != nil {
		return fmt.Errorf("create remote directories %q: %w", cleanPath, err)
	}
	return nil
}

// Rename renames or moves a remote file or directory.
func (c *Client) Rename(ctx context.Context, oldPath, newPath string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	cleanOld, err := sanitizeRemotePath(oldPath)
	if err != nil {
		return err
	}
	cleanNew, err := sanitizeRemotePath(newPath)
	if err != nil {
		return err
	}
	if err := c.sftp.Rename(cleanOld, cleanNew); err != nil {
		return fmt.Errorf("rename remote path %q -> %q: %w", cleanOld, cleanNew, err)
	}
	return nil
}

// Remove deletes a remote file or directory (recursively when recursive is true).
func (c *Client) Remove(ctx context.Context, remotePath string, recursive bool) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	cleanPath, err := sanitizeRemotePath(remotePath)
	if err != nil {
		return err
	}
	if cleanPath == "/" || cleanPath == "." {
		return fmt.Errorf("%w: refusing to remove root remote directory", domain.ErrPathTraversal)
	}

	info, err := c.sftp.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("stat remote path %q: %w", cleanPath, err)
	}

	if !info.IsDir() {
		if err := c.sftp.Remove(cleanPath); err != nil {
			return fmt.Errorf("remove remote file %q: %w", cleanPath, err)
		}
		return nil
	}

	if !recursive {
		if err := c.sftp.RemoveDirectory(cleanPath); err != nil {
			return fmt.Errorf("remove remote directory %q: %w", cleanPath, err)
		}
		return nil
	}

	return c.removeDirRecursive(ctx, cleanPath)
}

func (c *Client) removeDirRecursive(ctx context.Context, dirPath string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	entries, err := c.sftp.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("read remote dir for removal %q: %w", dirPath, err)
	}
	for _, entry := range entries {
		child := path.Join(dirPath, entry.Name())
		if entry.IsDir() {
			if err := c.removeDirRecursive(ctx, child); err != nil {
				return err
			}
		} else if err := c.sftp.Remove(child); err != nil {
			return fmt.Errorf("remove remote child %q: %w", child, err)
		}
	}
	if err := c.sftp.RemoveDirectory(dirPath); err != nil {
		return fmt.Errorf("remove remote directory %q: %w", dirPath, err)
	}
	return nil
}

// Close closes the underlying SFTP client session.
func (c *Client) Close() error {
	if c == nil || c.sftp == nil {
		return nil
	}
	if err := c.sftp.Close(); err != nil {
		return fmt.Errorf("close sftp client: %w", err)
	}
	return nil
}

func streamWithProgress(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	totalBytes int64,
	onProgress ports.TransferProgressFunc,
) error {
	buf := make([]byte, transferChunkSize)
	var transferred int64

	if onProgress != nil {
		onProgress(0, totalBytes)
	}

	for {
		if err := checkContext(ctx); err != nil {
			return err
		}

		n, readErr := src.Read(buf)
		if n > 0 {
			written := 0
			for written < n {
				if err := checkContext(ctx); err != nil {
					return err
				}
				w, writeErr := dst.Write(buf[written:n])
				if writeErr != nil {
					return fmt.Errorf("write transfer chunk: %w", writeErr)
				}
				written += w
			}
			transferred += int64(n)
			if onProgress != nil {
				onProgress(transferred, totalBytes)
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return fmt.Errorf("read transfer chunk: %w", readErr)
		}
	}
	return nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", domain.ErrTransferCancelled, ctx.Err())
	default:
		return nil
	}
}

func sanitizeRemotePath(raw string) (string, error) {
	if strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("%w: null byte in remote path", domain.ErrPathTraversal)
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: remote path is required", domain.ErrInvalidInput)
	}
	cleaned := path.Clean(strings.ReplaceAll(trimmed, "\\", "/"))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%w: relative parent traversal is not allowed", domain.ErrPathTraversal)
	}
	return cleaned, nil
}

func sanitizeLocalPath(raw string) (string, error) {
	if strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("%w: null byte in local path", domain.ErrPathTraversal)
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: local path is required", domain.ErrInvalidInput)
	}
	return filepath.Clean(trimmed), nil
}

func sortFileEntries(entries []domain.FileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
