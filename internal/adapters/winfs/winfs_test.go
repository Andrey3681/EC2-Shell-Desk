package winfs_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ec2-shell-desk/internal/adapters/winfs"
	"ec2-shell-desk/internal/core/domain"
)

func TestWinFSOperations(t *testing.T) {
	ctx := context.Background()
	fs := winfs.NewWinFS()

	home, err := fs.HomeDir()
	if err != nil || home == "" {
		t.Fatalf("HomeDir() = (%q, %v)", home, err)
	}

	roots, err := fs.ListRoots(ctx)
	if err != nil || len(roots) == 0 {
		t.Fatalf("ListRoots() = (%v, %v)", roots, err)
	}

	base := t.TempDir()
	subDir := filepath.Join(base, "docs")
	if err := fs.Mkdir(ctx, subDir); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	filePath := filepath.Join(base, "a-notes.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	entries, err := fs.ListDir(ctx, base)
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListDir() count = %d, want 2", len(entries))
	}
	// Directories must sort before files.
	if !entries[0].IsDir || entries[0].Name != "docs" {
		t.Fatalf("expected first entry to be directory 'docs', got %+v", entries[0])
	}
	if entries[1].IsDir || entries[1].Name != "a-notes.txt" {
		t.Fatalf("expected second entry to be file 'a-notes.txt', got %+v", entries[1])
	}

	renamed := filepath.Join(base, "b-notes.txt")
	if err := fs.Rename(ctx, filePath, renamed); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if err := fs.Remove(ctx, renamed); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, statErr := os.Stat(renamed); !os.IsNotExist(statErr) {
		t.Fatalf("expected removed file to not exist, statErr=%v", statErr)
	}

	// Refuse root removal and null-byte paths.
	if err := fs.Remove(ctx, roots[0]); !errors.Is(err, domain.ErrPathTraversal) {
		t.Fatalf("Remove(root) = %v, want ErrPathTraversal", err)
	}
	if _, err := fs.ListDir(ctx, "bad\x00path"); !errors.Is(err, domain.ErrPathTraversal) {
		t.Fatalf("ListDir(null byte) = %v, want ErrPathTraversal", err)
	}
}
