package domain

import "time"

// FileEntry represents a file or directory entry from either a local Windows volume or a remote SFTP path.
type FileEntry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	IsDir      bool      `json:"isDir"`
	Permission string    `json:"permission"`
	ModifiedAt time.Time `json:"modifiedAt"`
}
