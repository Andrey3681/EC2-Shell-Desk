// Package bootstrap wires hexagonal adapters, application services, Wails handlers, and desktop lifecycle hooks.
package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"ec2-shell-desk/internal/adapters/crypto"
	"ec2-shell-desk/internal/adapters/sftpclient"
	"ec2-shell-desk/internal/adapters/sqlite"
	"ec2-shell-desk/internal/adapters/sshclient"
	"ec2-shell-desk/internal/adapters/winfs"
	"ec2-shell-desk/internal/app"
	"ec2-shell-desk/internal/services"
)

const (
	appFolderName = "EC2ShellDesk"
	vaultFileName = "vault.db"
)

// Run composes all application dependencies and launches the Wails v2 Windows desktop window.
func Run(assets fs.FS) error {
	dbPath, err := resolveVaultDBPath()
	if err != nil {
		return fmt.Errorf("resolve vault database path: %w", err)
	}

	store, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		return fmt.Errorf("open vault sqlite store: %w", err)
	}

	vaultCrypto, err := crypto.NewVaultCrypto(crypto.DefaultArgon2Params())
	if err != nil {
		_ = store.Close()
		return fmt.Errorf("initialize vault cryptography: %w", err)
	}

	userRepo := sqlite.NewUserRepository(store)
	keyRepo := sqlite.NewKeyRepository(store)
	hostRepo := sqlite.NewHostRepository(store)
	knownRepo := sqlite.NewKnownHostRepository(store)

	sshManager := sshclient.NewManager(sftpclient.NewFromSSHClient)
	localFS := winfs.NewWinFS()
	bridge := app.NewEventBridge()

	authSvc := services.NewAuthService(userRepo, vaultCrypto, vaultCrypto)
	vaultSvc := services.NewVaultService(authSvc, keyRepo, vaultCrypto, vaultCrypto)
	hostSvc := services.NewHostService(authSvc, hostRepo, keyRepo)
	sessionSvc := services.NewSessionService(authSvc, hostRepo, knownRepo, vaultSvc, sshManager, bridge)
	transferSvc := services.NewTransferService(authSvc, sessionSvc, localFS, bridge)

	authHandler := app.NewAuthHandler(bridge, authSvc, sessionSvc, transferSvc)
	keyHandler := app.NewKeyHandler(bridge, vaultSvc)
	hostHandler := app.NewHostHandler(bridge, hostSvc)
	termHandler := app.NewTerminalHandler(bridge, sessionSvc)
	sftpHandler := app.NewSFTPHandler(bridge, transferSvc)

	runErr := wails.Run(&options.App{
		Title:     "EC2 Shell Desk",
		Width:     1360,
		Height:    840,
		MinWidth:  1024,
		MinHeight: 680,
		BackgroundColour: &options.RGBA{
			R: 15,
			G: 23,
			B: 42,
			A: 1,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.Dark,
		},
		OnStartup: func(ctx context.Context) {
			bridge.SetContext(ctx)
			slog.Info("ec2-shell-desk desktop started")
		},
		OnShutdown: func(_ context.Context) {
			transferSvc.CloseAll()
			sessionSvc.CloseAll()
			authSvc.Lock()
			if closeErr := store.Close(); closeErr != nil {
				slog.Error("failed closing sqlite store", "error", closeErr)
			}
			slog.Info("ec2-shell-desk desktop shutdown complete")
		},
		Bind: []interface{}{
			authHandler,
			keyHandler,
			hostHandler,
			termHandler,
			sftpHandler,
		},
	})
	if runErr != nil {
		_ = store.Close()
		return fmt.Errorf("run wails desktop application: %w", runErr)
	}
	return nil
}
