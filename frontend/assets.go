// Package frontend embeds the compiled Vite desktop UI bundle for Wails v2.
package frontend

import "embed"

// Assets contains the built static assets under frontend/dist.
//
//go:embed all:dist
var Assets embed.FS
