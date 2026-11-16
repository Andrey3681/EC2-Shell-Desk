package app

import (
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/services"
)

// KeyHandler exposes SSH key vault bindings and native Windows file selection to Wails.
type KeyHandler struct {
	bridge *EventBridge
	vault  *services.VaultService
}

// NewKeyHandler constructs a KeyHandler wired to VaultService.
func NewKeyHandler(bridge *EventBridge, vault *services.VaultService) *KeyHandler {
	return &KeyHandler{
		bridge: bridge,
		vault:  vault,
	}
}

// PickKeyFile opens a native Windows file dialog to select an SSH private key file.
func (h *KeyHandler) PickKeyFile() (string, error) {
	selected, err := runtime.OpenFileDialog(h.bridge.Context(), runtime.OpenDialogOptions{
		Title: "Select SSH Private Key",
		Filters: []runtime.FileFilter{
			{DisplayName: "SSH Private Keys (*.pem, *.key)", Pattern: "*.pem;*.key"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("open key file dialog: %w", err)
	}
	return selected, nil
}

// ImportKeyFromFile reads an SSH key from disk, encrypts it into the active user's vault, and wipes memory buffers.
func (h *KeyHandler) ImportKeyFromFile(name, filePath, passphrase string) (domain.SSHKeySummary, error) {
	if filePath == "" {
		return domain.SSHKeySummary{}, fmt.Errorf("%w: file path is required", domain.ErrInvalidInput)
	}

	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return domain.SSHKeySummary{}, fmt.Errorf("read key file: %w", err)
	}
	defer wipeBytes(rawBytes)

	var passBytes []byte
	if passphrase != "" {
		passBytes = []byte(passphrase)
		defer wipeBytes(passBytes)
	}

	return h.vault.ImportKey(h.bridge.Context(), name, rawBytes, passBytes)
}

// ImportKeyContent validates and encrypts pasted PEM/OpenSSH key content into the active user's vault.
func (h *KeyHandler) ImportKeyContent(name, rawPEM, passphrase string) (domain.SSHKeySummary, error) {
	rawBytes := []byte(rawPEM)
	defer wipeBytes(rawBytes)

	var passBytes []byte
	if passphrase != "" {
		passBytes = []byte(passphrase)
		defer wipeBytes(passBytes)
	}

	return h.vault.ImportKey(h.bridge.Context(), name, rawBytes, passBytes)
}

// ListKeys returns safe SSH key metadata summaries owned by the active user.
func (h *KeyHandler) ListKeys() ([]domain.SSHKeySummary, error) {
	return h.vault.ListKeys(h.bridge.Context())
}

// DeleteKey deletes an unreferenced SSH key owned by the active user.
func (h *KeyHandler) DeleteKey(keyID string) error {
	return h.vault.DeleteKey(h.bridge.Context(), keyID)
}

func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
