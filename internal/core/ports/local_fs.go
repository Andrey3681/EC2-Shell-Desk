package ports

import (
	"context"

	"ec2-shell-desk/internal/core/domain"
)

// LocalFileSystem provides safe browsing of local Windows volumes and directories.
type LocalFileSystem interface {
	HomeDir() (string, error)
	ListRoots(ctx context.Context) ([]string, error)
	ListDir(ctx context.Context, localPath string) ([]domain.FileEntry, error)
	Mkdir(ctx context.Context, localPath string) error
	Rename(ctx context.Context, oldPath, newPath string) error
	Remove(ctx context.Context, localPath string) error
}
