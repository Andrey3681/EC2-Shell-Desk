//go:build bindings

package bootstrap

func resolveVaultDBPath() (string, error) {
	// Use in-memory SQLite during Wails binding generation so builds never touch user AppData.
	return ":memory:", nil
}
