package ports

import (
	"context"

	"ec2-shell-desk/internal/core/domain"
)

// TransferProgressFunc receives incremental byte counts during an SFTP upload or download.
type TransferProgressFunc func(bytesTransferred, totalBytes int64)

// SFTPClient defines remote filesystem and cancelable file transfer operations over SFTP.
type SFTPClient interface {
	WorkingDir(ctx context.Context) (string, error)
	ListDir(ctx context.Context, remotePath string) ([]domain.FileEntry, error)
	Upload(ctx context.Context, localPath, remotePath string, onProgress TransferProgressFunc) error
	UploadDir(ctx context.Context, localDirPath, remoteTargetDir string, onProgress TransferProgressFunc) error
	Download(ctx context.Context, remotePath, localPath string, onProgress TransferProgressFunc) error
	Mkdir(ctx context.Context, remotePath string) error
	MkdirAll(ctx context.Context, remotePath string) error
	Rename(ctx context.Context, oldPath, newPath string) error
	Remove(ctx context.Context, remotePath string, recursive bool) error
	Close() error
}
