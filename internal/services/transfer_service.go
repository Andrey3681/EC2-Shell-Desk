package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const (
	progressEmitInterval      = 50 * time.Millisecond
	defaultMaxTransferHistory = 100
)

// TransferEventSink receives throttled SFTP transfer progress and completion notifications.
type TransferEventSink interface {
	OnTransferProgress(task domain.TransferTask)
}

type managedTransfer struct {
	task   domain.TransferTask
	isDir  bool
	cancel context.CancelFunc
}

// TransferService manages local Windows browsing, remote SFTP browsing, and cancelable file transfers.
type TransferService struct {
	auth     *AuthService
	sessions *SessionService
	localFS  ports.LocalFileSystem
	sink     TransferEventSink

	mu         sync.RWMutex
	maxHistory int
	transfers  map[string]*managedTransfer
}

// NewTransferService constructs a TransferService wired to auth, active sessions, local FS, and event sink.
func NewTransferService(
	auth *AuthService,
	sessions *SessionService,
	localFS ports.LocalFileSystem,
	sink TransferEventSink,
) *TransferService {
	return &TransferService{
		auth:       auth,
		sessions:   sessions,
		localFS:    localFS,
		sink:       sink,
		maxHistory: defaultMaxTransferHistory,
		transfers:  make(map[string]*managedTransfer),
	}
}

// SetMaxHistory overrides the maximum number of finished transfer tasks retained in memory.
func (s *TransferService) SetMaxHistory(limit int) {
	if limit <= 0 {
		return
	}
	s.mu.Lock()
	s.maxHistory = limit
	s.pruneFinishedLocked()
	s.mu.Unlock()
}

// LocalHomeDir returns the local user's home directory when authenticated.
func (s *TransferService) LocalHomeDir() (string, error) {
	if _, err := s.auth.CurrentUser(); err != nil {
		return "", err
	}
	home, err := s.localFS.HomeDir()
	if err != nil {
		return "", fmt.Errorf("get local home dir: %w", err)
	}
	return home, nil
}

// ListLocalRoots lists available local filesystem drive roots (e.g. C:\, D:\) for the active user.
func (s *TransferService) ListLocalRoots(ctx context.Context) ([]string, error) {
	if _, err := s.auth.CurrentUser(); err != nil {
		return nil, err
	}
	roots, err := s.localFS.ListRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local roots: %w", err)
	}
	return roots, nil
}

// ListLocalDir lists files and subdirectories at localPath for the active user.
func (s *TransferService) ListLocalDir(ctx context.Context, localPath string) ([]domain.FileEntry, error) {
	if _, err := s.auth.CurrentUser(); err != nil {
		return nil, err
	}
	entries, err := s.localFS.ListDir(ctx, localPath)
	if err != nil {
		return nil, fmt.Errorf("list local dir: %w", err)
	}
	return entries, nil
}

// MakeLocalDir creates a local directory at localPath.
func (s *TransferService) MakeLocalDir(ctx context.Context, localPath string) error {
	if _, err := s.auth.CurrentUser(); err != nil {
		return err
	}
	if err := s.localFS.Mkdir(ctx, localPath); err != nil {
		return fmt.Errorf("create local dir: %w", err)
	}
	return nil
}

// RenameLocal renames or moves a local file or directory.
func (s *TransferService) RenameLocal(ctx context.Context, oldPath, newPath string) error {
	if _, err := s.auth.CurrentUser(); err != nil {
		return err
	}
	if err := s.localFS.Rename(ctx, oldPath, newPath); err != nil {
		return fmt.Errorf("rename local path: %w", err)
	}
	return nil
}

// DeleteLocal removes a local file or directory.
func (s *TransferService) DeleteLocal(ctx context.Context, localPath string) error {
	if _, err := s.auth.CurrentUser(); err != nil {
		return err
	}
	if err := s.localFS.Remove(ctx, localPath); err != nil {
		return fmt.Errorf("remove local path: %w", err)
	}
	return nil
}

// RemoteWorkingDir returns the initial remote directory of the session's SFTP channel.
func (s *TransferService) RemoteWorkingDir(ctx context.Context, sessionID string) (string, error) {
	sftpCli, err := s.sessions.GetSFTPClient(sessionID)
	if err != nil {
		return "", err
	}
	wd, err := sftpCli.WorkingDir(ctx)
	if err != nil {
		return "", fmt.Errorf("get remote working dir: %w", err)
	}
	return wd, nil
}

// ListRemoteDir lists files and directories on the remote SFTP host for an owned session.
func (s *TransferService) ListRemoteDir(ctx context.Context, sessionID, remotePath string) ([]domain.FileEntry, error) {
	sftpCli, err := s.sessions.GetSFTPClient(sessionID)
	if err != nil {
		return nil, err
	}
	entries, err := sftpCli.ListDir(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("list remote dir: %w", err)
	}
	return entries, nil
}

// MakeRemoteDir creates a directory on the remote SFTP host.
func (s *TransferService) MakeRemoteDir(ctx context.Context, sessionID, remotePath string) error {
	sftpCli, err := s.sessions.GetSFTPClient(sessionID)
	if err != nil {
		return err
	}
	if err := sftpCli.Mkdir(ctx, remotePath); err != nil {
		return fmt.Errorf("create remote dir: %w", err)
	}
	return nil
}

// RenameRemote renames or moves a remote file or directory over SFTP.
func (s *TransferService) RenameRemote(ctx context.Context, sessionID, oldPath, newPath string) error {
	sftpCli, err := s.sessions.GetSFTPClient(sessionID)
	if err != nil {
		return err
	}
	if err := sftpCli.Rename(ctx, oldPath, newPath); err != nil {
		return fmt.Errorf("rename remote path: %w", err)
	}
	return nil
}

// DeleteRemote deletes a remote file or directory over SFTP.
func (s *TransferService) DeleteRemote(ctx context.Context, sessionID, remotePath string, recursive bool) error {
	sftpCli, err := s.sessions.GetSFTPClient(sessionID)
	if err != nil {
		return err
	}
	if err := sftpCli.Remove(ctx, remotePath, recursive); err != nil {
		return fmt.Errorf("remove remote path: %w", err)
	}
	return nil
}

// StartUpload queues and executes an asynchronous cancelable file upload from localPath to remotePath.
func (s *TransferService) StartUpload(sessionID, localPath, remotePath string) (domain.TransferTask, error) {
	return s.startTransfer(sessionID, domain.TransferUpload, localPath, remotePath, false)
}

// StartUploadDirectory queues and executes an asynchronous cancelable recursive directory upload from localDirPath to remoteTargetDir.
func (s *TransferService) StartUploadDirectory(sessionID, localDirPath, remoteTargetDir string) (domain.TransferTask, error) {
	return s.startTransfer(sessionID, domain.TransferUpload, localDirPath, remoteTargetDir, true)
}

// StartDownload queues and executes an asynchronous cancelable file download from remotePath to localPath.
func (s *TransferService) StartDownload(sessionID, remotePath, localPath string) (domain.TransferTask, error) {
	return s.startTransfer(sessionID, domain.TransferDownload, localPath, remotePath, false)
}

// CancelTransfer cancels an active transfer task owned by the authenticated user.
func (s *TransferService) CancelTransfer(transferID string) error {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return err
	}

	s.mu.RLock()
	mt, ok := s.transfers[transferID]
	s.mu.RUnlock()

	if !ok || mt.task.OwnerUserID != user.ID {
		return fmt.Errorf("%w: transfer task", domain.ErrNotFound)
	}
	mt.cancel()
	return nil
}

// ListTransfers returns tracked transfer tasks belonging to the authenticated user sorted by start time.
func (s *TransferService) ListTransfers() ([]domain.TransferTask, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.TransferTask, 0)
	for _, mt := range s.transfers {
		if mt.task.OwnerUserID == user.ID {
			result = append(result, mt.task)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].StartedAt.Equal(result[j].StartedAt) {
			return result[i].StartedAt.Before(result[j].StartedAt)
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// CloseAll cancels all active file transfers and clears finished transfer history from memory.
func (s *TransferService) CloseAll() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.transfers))
	for id, mt := range s.transfers {
		cancels = append(cancels, mt.cancel)
		if mt.task.State != domain.TransferRunning && mt.task.State != domain.TransferPending {
			delete(s.transfers, id)
		}
	}
	s.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (s *TransferService) startTransfer(
	sessionID string,
	direction domain.TransferDirection,
	localPath, remotePath string,
	isDir bool,
) (domain.TransferTask, error) {
	user, err := s.auth.CurrentUser()
	if err != nil {
		return domain.TransferTask{}, err
	}

	sftpCli, err := s.sessions.GetSFTPClient(sessionID)
	if err != nil {
		return domain.TransferTask{}, err
	}

	transferCtx, cancel := context.WithCancel(context.Background())
	task := domain.TransferTask{
		ID:          uuid.NewString(),
		SessionID:   sessionID,
		OwnerUserID: user.ID,
		Direction:   direction,
		LocalPath:   localPath,
		RemotePath:  remotePath,
		State:       domain.TransferRunning,
		StartedAt:   time.Now().UTC(),
	}

	entry := &managedTransfer{
		task:   task,
		isDir:  isDir,
		cancel: cancel,
	}

	s.mu.Lock()
	s.transfers[task.ID] = entry
	s.mu.Unlock()

	s.emitProgress(task)

	go s.runTransfer(transferCtx, entry, sftpCli)
	return task, nil
}

func (s *TransferService) runTransfer(ctx context.Context, entry *managedTransfer, sftpCli ports.SFTPClient) {
	defer entry.cancel()

	var lastEmit time.Time
	onProgress := func(transferred, total int64) {
		s.mu.Lock()
		entry.task.BytesTransferred = transferred
		entry.task.TotalBytes = total
		snapshot := entry.task
		s.mu.Unlock()

		now := time.Now()
		if transferred == total || lastEmit.IsZero() || now.Sub(lastEmit) >= progressEmitInterval {
			lastEmit = now
			s.emitProgress(snapshot)
		}
	}

	var transferErr error
	if entry.task.Direction == domain.TransferUpload {
		if entry.isDir {
			transferErr = sftpCli.UploadDir(ctx, entry.task.LocalPath, entry.task.RemotePath, onProgress)
		} else {
			transferErr = sftpCli.Upload(ctx, entry.task.LocalPath, entry.task.RemotePath, onProgress)
		}
	} else {
		transferErr = sftpCli.Download(ctx, entry.task.RemotePath, entry.task.LocalPath, onProgress)
	}

	completedAt := time.Now().UTC()

	s.mu.Lock()
	entry.task.CompletedAt = &completedAt
	switch {
	case transferErr == nil:
		entry.task.State = domain.TransferCompleted
	case errors.Is(transferErr, domain.ErrTransferCancelled) || errors.Is(ctx.Err(), context.Canceled):
		entry.task.State = domain.TransferCancelled
		entry.task.ErrorMessage = "cancelled"
	default:
		entry.task.State = domain.TransferFailed
		entry.task.ErrorMessage = transferErr.Error()
	}
	finalSnapshot := entry.task
	s.pruneFinishedLocked()
	s.mu.Unlock()

	s.emitProgress(finalSnapshot)
}

func (s *TransferService) pruneFinishedLocked() {
	limit := s.maxHistory
	if limit <= 0 {
		limit = defaultMaxTransferHistory
	}

	type finishedItem struct {
		id         string
		finishedAt time.Time
	}

	finished := make([]finishedItem, 0)
	for id, mt := range s.transfers {
		if mt.task.State == domain.TransferRunning || mt.task.State == domain.TransferPending {
			continue
		}
		ts := mt.task.StartedAt
		if mt.task.CompletedAt != nil {
			ts = *mt.task.CompletedAt
		}
		finished = append(finished, finishedItem{id: id, finishedAt: ts})
	}

	if len(finished) <= limit {
		return
	}

	sort.Slice(finished, func(i, j int) bool {
		if !finished[i].finishedAt.Equal(finished[j].finishedAt) {
			return finished[i].finishedAt.Before(finished[j].finishedAt)
		}
		return finished[i].id < finished[j].id
	})

	excess := len(finished) - limit
	for i := 0; i < excess; i++ {
		delete(s.transfers, finished[i].id)
	}
}

func (s *TransferService) emitProgress(task domain.TransferTask) {
	if s.sink != nil {
		s.sink.OnTransferProgress(task)
	}
}
