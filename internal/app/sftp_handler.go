package app

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

// SFTPHandler exposes Xftp-style dual-pane local/remote browsing and transfer queue bindings to Wails.
type SFTPHandler struct {
	bridge    *EventBridge
	transfers *services.TransferService
}

// NewSFTPHandler constructs an SFTPHandler wired to TransferService.
func NewSFTPHandler(bridge *EventBridge, transfers *services.TransferService) *SFTPHandler {
	return &SFTPHandler{
		bridge:    bridge,
		transfers: transfers,
	}
}

// LocalHomeDir returns the Windows user's home directory path.
func (h *SFTPHandler) LocalHomeDir() (string, error) {
	return h.transfers.LocalHomeDir()
}

// ListLocalRoots returns mounted local drive roots (e.g. C:\, D:\).
func (h *SFTPHandler) ListLocalRoots() ([]string, error) {
	return h.transfers.ListLocalRoots(h.bridge.Context())
}

// ListLocalDir returns local files and directories at localPath.
func (h *SFTPHandler) ListLocalDir(localPath string) ([]domain.FileEntry, error) {
	return h.transfers.ListLocalDir(h.bridge.Context(), localPath)
}

// MakeLocalDir creates a directory at localPath.
func (h *SFTPHandler) MakeLocalDir(localPath string) error {
	return h.transfers.MakeLocalDir(h.bridge.Context(), localPath)
}

// RenameLocal renames or moves a local file or directory.
func (h *SFTPHandler) RenameLocal(oldPath, newPath string) error {
	return h.transfers.RenameLocal(h.bridge.Context(), oldPath, newPath)
}

// DeleteLocal deletes a local file or directory.
func (h *SFTPHandler) DeleteLocal(localPath string) error {
	return h.transfers.DeleteLocal(h.bridge.Context(), localPath)
}

// PickLocalFilesToUpload opens a native Windows multi-file selection dialog for SFTP upload.
func (h *SFTPHandler) PickLocalFilesToUpload() ([]string, error) {
	selected, err := runtime.OpenMultipleFilesDialog(h.bridge.Context(), runtime.OpenDialogOptions{
		Title: "Select Files to Upload",
	})
	if err != nil {
		return nil, fmt.Errorf("open upload file dialog: %w", err)
	}
	return selected, nil
}

// PickLocalDirectory opens a native Windows directory picker for local browsing or download destination.
func (h *SFTPHandler) PickLocalDirectory() (string, error) {
	selected, err := runtime.OpenDirectoryDialog(h.bridge.Context(), runtime.OpenDialogOptions{
		Title: "Select Local Folder",
	})
	if err != nil {
		return "", fmt.Errorf("open directory dialog: %w", err)
	}
	return selected, nil
}

// RemoteWorkingDir returns the initial remote working directory for an active SSH session.
func (h *SFTPHandler) RemoteWorkingDir(sessionID string) (string, error) {
	return h.transfers.RemoteWorkingDir(h.bridge.Context(), sessionID)
}

// ListRemoteDir lists remote files and directories over SFTP.
func (h *SFTPHandler) ListRemoteDir(sessionID, remotePath string) ([]domain.FileEntry, error) {
	return h.transfers.ListRemoteDir(h.bridge.Context(), sessionID, remotePath)
}

// MakeRemoteDir creates a remote directory over SFTP.
func (h *SFTPHandler) MakeRemoteDir(sessionID, remotePath string) error {
	return h.transfers.MakeRemoteDir(h.bridge.Context(), sessionID, remotePath)
}

// RenameRemote renames or moves a remote file or directory over SFTP.
func (h *SFTPHandler) RenameRemote(sessionID, oldPath, newPath string) error {
	return h.transfers.RenameRemote(h.bridge.Context(), sessionID, oldPath, newPath)
}

// DeleteRemote deletes a remote file or directory over SFTP.
func (h *SFTPHandler) DeleteRemote(sessionID, remotePath string, recursive bool) error {
	return h.transfers.DeleteRemote(h.bridge.Context(), sessionID, remotePath, recursive)
}

// StartUpload queues an SFTP upload of localFilePath into remoteDirPath.
func (h *SFTPHandler) StartUpload(sessionID, localFilePath, remoteDirPath string) (domain.TransferTask, error) {
	remoteTarget := path.Join(remoteDirPath, filepath.Base(localFilePath))
	return h.transfers.StartUpload(sessionID, localFilePath, remoteTarget)
}

// StartDownload queues an SFTP download of remoteFilePath into localDirPath.
func (h *SFTPHandler) StartDownload(sessionID, remoteFilePath, localDirPath string) (domain.TransferTask, error) {
	localTarget := filepath.Join(localDirPath, path.Base(remoteFilePath))
	return h.transfers.StartDownload(sessionID, remoteFilePath, localTarget)
}

// CancelTransfer cancels an active SFTP transfer owned by the active user.
func (h *SFTPHandler) CancelTransfer(transferID string) error {
	return h.transfers.CancelTransfer(transferID)
}

// ListTransfers returns tracked transfer tasks belonging to the active user.
func (h *SFTPHandler) ListTransfers() ([]domain.TransferTask, error) {
	return h.transfers.ListTransfers()
}
