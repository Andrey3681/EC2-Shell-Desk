package winfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"ec2-shell-desk/internal/core/domain"
)

// WinFS implements ports.LocalFileSystem for browsing Windows volumes and local directories safely.
type WinFS struct{}

// NewWinFS constructs a local Windows filesystem adapter.
func NewWinFS() *WinFS {
	return &WinFS{}
}

// HomeDir returns the current operating system user's home directory.
func (w *WinFS) HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Clean(home), nil
}

// ListRoots returns available mounted filesystem roots (e.g. C:\, D:\ on Windows).
func (w *WinFS) ListRoots(ctx context.Context) ([]string, error) {
	if err := checkCtx(ctx); err != nil {
		return nil, err
	}

	if runtime.GOOS != "windows" {
		return []string{"/"}, nil
	}

	roots := make([]string, 0)
	for letter := 'A'; letter <= 'Z'; letter++ {
		if err := checkCtx(ctx); err != nil {
			return nil, err
		}
		drive := string(letter) + `:\`
		if info, err := os.Stat(drive); err == nil && info.IsDir() {
			roots = append(roots, drive)
		}
	}
	return roots, nil
}

// ListDir reads localPath and returns entries sorted directories-first then alphabetically.
func (w *WinFS) ListDir(ctx context.Context, localPath string) ([]domain.FileEntry, error) {
	if err := checkCtx(ctx); err != nil {
		return nil, err
	}
	cleanPath, err := validateLocalPath(localPath)
	if err != nil {
		return nil, err
	}

	dirEntries, err := os.ReadDir(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read local directory %q: %w", cleanPath, err)
	}

	entries := make([]domain.FileEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		entries = append(entries, domain.FileEntry{
			Name:       entry.Name(),
			Path:       filepath.Join(cleanPath, entry.Name()),
			Size:       info.Size(),
			IsDir:      entry.IsDir(),
			Permission: info.Mode().String(),
			ModifiedAt: info.ModTime().UTC(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

// Mkdir creates a local directory at localPath.
func (w *WinFS) Mkdir(ctx context.Context, localPath string) error {
	if err := checkCtx(ctx); err != nil {
		return err
	}
	cleanPath, err := validateLocalPath(localPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cleanPath, 0o750); err != nil {
		return fmt.Errorf("create local directory %q: %w", cleanPath, err)
	}
	return nil
}

// Rename moves or renames a local file or directory.
func (w *WinFS) Rename(ctx context.Context, oldPath, newPath string) error {
	if err := checkCtx(ctx); err != nil {
		return err
	}
	cleanOld, err := validateLocalPath(oldPath)
	if err != nil {
		return err
	}
	cleanNew, err := validateLocalPath(newPath)
	if err != nil {
		return err
	}
	if isRootPath(cleanOld) {
		return fmt.Errorf("%w: refusing to rename filesystem root", domain.ErrPathTraversal)
	}
	if err := os.Rename(cleanOld, cleanNew); err != nil {
		return fmt.Errorf("rename local path %q -> %q: %w", cleanOld, cleanNew, err)
	}
	return nil
}

// Remove deletes a local file or directory safely, refusing to delete filesystem roots.
func (w *WinFS) Remove(ctx context.Context, localPath string) error {
	if err := checkCtx(ctx); err != nil {
		return err
	}
	cleanPath, err := validateLocalPath(localPath)
	if err != nil {
		return err
	}
	if isRootPath(cleanPath) {
		return fmt.Errorf("%w: refusing to remove filesystem root", domain.ErrPathTraversal)
	}
	if err := os.RemoveAll(cleanPath); err != nil {
		return fmt.Errorf("remove local path %q: %w", cleanPath, err)
	}
	return nil
}

func validateLocalPath(raw string) (string, error) {
	if strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("%w: null byte in local path", domain.ErrPathTraversal)
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: local path is required", domain.ErrInvalidInput)
	}
	return filepath.Clean(trimmed), nil
}

func isRootPath(cleanPath string) bool {
	vol := filepath.VolumeName(cleanPath)
	rest := strings.TrimPrefix(cleanPath, vol)
	return rest == "" || rest == string(filepath.Separator) || rest == "/"
}

func checkCtx(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("local filesystem operation cancelled: %w", err)
	}
	return nil
}
