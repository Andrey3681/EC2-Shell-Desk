// Package main launches EC2 Shell Desk for Wails v2 root-level build and dev commands.
package main

import (
	"log/slog"
	"os"

	"ec2-shell-desk/cmd/desktop/bootstrap"
	"ec2-shell-desk/frontend"
)

func main() {
	if err := bootstrap.Run(frontend.Assets); err != nil {
		slog.Error("desktop application exited with error", "error", err)
		os.Exit(1)
	}
}
