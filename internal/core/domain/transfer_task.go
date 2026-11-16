package domain

import "time"

// TransferDirection indicates whether a file transfer moves data to or from the remote host.
type TransferDirection string

const (
	// TransferUpload transfers a local Windows file to a remote SFTP directory.
	TransferUpload TransferDirection = "upload"
	// TransferDownload transfers a remote SFTP file to a local Windows directory.
	TransferDownload TransferDirection = "download"
)

// TransferState describes the execution lifecycle state of a TransferTask.
type TransferState string

const (
	// TransferPending indicates the task is queued and waiting to start.
	TransferPending TransferState = "pending"
	// TransferRunning indicates bytes are actively streaming.
	TransferRunning TransferState = "running"
	// TransferCompleted indicates the file finished transferring successfully.
	TransferCompleted TransferState = "completed"
	// TransferFailed indicates the transfer aborted with an error.
	TransferFailed TransferState = "failed"
	// TransferCancelled indicates the transfer was cancelled by the user or application shutdown.
	TransferCancelled TransferState = "cancelled"
)

// TransferTask tracks progress and metadata for a single SFTP file transfer operation.
type TransferTask struct {
	ID               string            `json:"id"`
	SessionID        string            `json:"sessionId"`
	OwnerUserID      string            `json:"ownerUserId"`
	Direction        TransferDirection `json:"direction"`
	LocalPath        string            `json:"localPath"`
	RemotePath       string            `json:"remotePath"`
	TotalBytes       int64             `json:"totalBytes"`
	BytesTransferred int64             `json:"bytesTransferred"`
	State            TransferState     `json:"state"`
	ErrorMessage     string            `json:"errorMessage,omitempty"`
	StartedAt        time.Time         `json:"startedAt"`
	CompletedAt      *time.Time        `json:"completedAt,omitempty"`
}
