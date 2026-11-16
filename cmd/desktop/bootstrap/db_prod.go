//go:build !bindings

package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveVaultDBPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(configDir, appFolderName, vaultFileName), nil
}
