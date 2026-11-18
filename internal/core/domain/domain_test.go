package domain_test

import (
	"errors"
	"testing"
	"time"

	"ec2-shell-desk/internal/core/domain"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  error
	}{
		{name: "valid username", username: "admin", wantErr: nil},
		{name: "minimum length", username: "abc", wantErr: nil},
		{name: "too short", username: "ab", wantErr: domain.ErrInvalidInput},
		{name: "leading whitespace", username: " admin", wantErr: domain.ErrInvalidInput},
		{name: "trailing whitespace", username: "admin ", wantErr: domain.ErrInvalidInput},
		{name: "empty", username: "", wantErr: domain.ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateUsername(tc.username)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateUsername(%q) error = %v, want %v", tc.username, err, tc.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "valid password", password: "correct-horse-battery", wantErr: nil},
		{name: "minimum 8 chars", password: "12345678", wantErr: nil},
		{name: "too short", password: "1234567", wantErr: domain.ErrInvalidInput},
		{name: "empty", password: "", wantErr: domain.ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidatePassword(tc.password)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidatePassword() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestHostInstanceValidate(t *testing.T) {
	valid := domain.HostInstance{
		ID:          "host-1",
		OwnerUserID: "user-1",
		Alias:       "Backend API Prod",
		HostAddress: "54.123.45.67",
		Port:        22,
		SSHUsername: "ec2-user",
		Environment: domain.EnvProd,
	}

	tests := []struct {
		name    string
		mutate  func(h *domain.HostInstance)
		wantErr error
	}{
		{name: "valid host", mutate: func(_ *domain.HostInstance) {}, wantErr: nil},
		{name: "missing owner", mutate: func(h *domain.HostInstance) { h.OwnerUserID = "" }, wantErr: domain.ErrInvalidInput},
		{name: "empty alias", mutate: func(h *domain.HostInstance) { h.Alias = "   " }, wantErr: domain.ErrInvalidInput},
		{name: "host with spaces", mutate: func(h *domain.HostInstance) { h.HostAddress = "54.1.2.3 extra" }, wantErr: domain.ErrInvalidInput},
		{name: "port zero", mutate: func(h *domain.HostInstance) { h.Port = 0 }, wantErr: domain.ErrInvalidInput},
		{name: "port out of range", mutate: func(h *domain.HostInstance) { h.Port = 70000 }, wantErr: domain.ErrInvalidInput},
		{name: "empty ssh user", mutate: func(h *domain.HostInstance) { h.SSHUsername = "" }, wantErr: domain.ErrInvalidInput},
		{name: "invalid environment", mutate: func(h *domain.HostInstance) { h.Environment = "QA" }, wantErr: domain.ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := valid
			tc.mutate(&h)
			err := h.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestProjectionsStripSecrets(t *testing.T) {
	now := time.Now().UTC()
	u := domain.User{
		ID:           "u1",
		Username:     "alice",
		PasswordHash: []byte("secret-hash"),
		AuthSalt:     []byte("auth-salt"),
		VaultSalt:    []byte("vault-salt"),
		IsDefault:    true,
		CreatedAt:    now,
	}
	profile := u.Profile()
	if profile.ID != u.ID || profile.Username != u.Username || !profile.IsDefault {
		t.Fatalf("unexpected profile: %+v", profile)
	}

	k := domain.SSHKey{
		ID:                  "k1",
		OwnerUserID:         "u1",
		Name:                "aws-prod.pem",
		KeyType:             domain.KeyTypeEd25519,
		Fingerprint:         "SHA256:abc",
		EncryptedPEM:        []byte("ciphertext"),
		EncryptedPassphrase: []byte("cipher-pass"),
		HasPassphrase:       true,
		CreatedAt:           now,
	}
	summary := k.Summary()
	if summary.ID != k.ID || summary.OwnerUserID != k.OwnerUserID || !summary.HasPassphrase {
		t.Fatalf("unexpected key summary: %+v", summary)
	}
}

func TestSSHTunnelValidate(t *testing.T) {
	valid := domain.SSHTunnel{
		ID:          "tun-1",
		OwnerUserID: "user-1",
		HostID:      "host-1",
		Name:        "RDS Prod Postgres",
		Preset:      domain.TunnelPresetPostgreSQL,
		LocalHost:   domain.DefaultTunnelLocalHost,
		LocalPort:   15432,
		RemoteHost:  "mydb.abc123.us-east-1.rds.amazonaws.com",
		RemotePort:  5432,
	}

	tests := []struct {
		name    string
		mutate  func(tun *domain.SSHTunnel)
		wantErr error
	}{
		{name: "valid tunnel", mutate: func(_ *domain.SSHTunnel) {}, wantErr: nil},
		{name: "missing owner", mutate: func(tun *domain.SSHTunnel) { tun.OwnerUserID = "" }, wantErr: domain.ErrInvalidInput},
		{name: "missing host", mutate: func(tun *domain.SSHTunnel) { tun.HostID = "" }, wantErr: domain.ErrInvalidInput},
		{name: "empty name", mutate: func(tun *domain.SSHTunnel) { tun.Name = " " }, wantErr: domain.ErrInvalidInput},
		{name: "unsupported preset", mutate: func(tun *domain.SSHTunnel) { tun.Preset = "oracle" }, wantErr: domain.ErrInvalidInput},
		{name: "local port zero", mutate: func(tun *domain.SSHTunnel) { tun.LocalPort = 0 }, wantErr: domain.ErrInvalidInput},
		{name: "remote host whitespace", mutate: func(tun *domain.SSHTunnel) { tun.RemoteHost = "bad host" }, wantErr: domain.ErrInvalidInput},
		{name: "remote port out of range", mutate: func(tun *domain.SSHTunnel) { tun.RemotePort = 70000 }, wantErr: domain.ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tun := valid
			tc.mutate(&tun)
			err := tun.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateGroupName(t *testing.T) {
	tests := []struct {
		name      string
		groupName string
		wantErr   error
	}{
		{name: "valid group name", groupName: "Universidad", wantErr: nil},
		{name: "with spaces", groupName: "Trabajo AWS", wantErr: nil},
		{name: "empty", groupName: "   ", wantErr: domain.ErrInvalidInput},
		{name: "too long", groupName: string(make([]byte, domain.MaxGroupNameLength+1)), wantErr: domain.ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateGroupName(tc.groupName)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateGroupName(%q) error = %v, want %v", tc.groupName, err, tc.wantErr)
			}
		})
	}
}

func TestCommandSnippetValidate(t *testing.T) {
	valid := domain.CommandSnippet{
		ID:          "snip-1",
		OwnerUserID: "user-1",
		Name:        "Deploy API",
		Command:     "git pull && docker compose up -d --build",
		AutoExecute: true,
	}

	tests := []struct {
		name    string
		mutate  func(s *domain.CommandSnippet)
		wantErr error
	}{
		{name: "valid snippet", mutate: func(_ *domain.CommandSnippet) {}, wantErr: nil},
		{name: "missing owner", mutate: func(s *domain.CommandSnippet) { s.OwnerUserID = "" }, wantErr: domain.ErrInvalidInput},
		{name: "empty name", mutate: func(s *domain.CommandSnippet) { s.Name = "   " }, wantErr: domain.ErrInvalidInput},
		{name: "empty command", mutate: func(s *domain.CommandSnippet) { s.Command = "  \n " }, wantErr: domain.ErrInvalidInput},
		{name: "command too large", mutate: func(s *domain.CommandSnippet) { s.Command = string(make([]byte, domain.MaxSnippetCommandLength+1)) }, wantErr: domain.ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := valid
			tc.mutate(&s)
			err := s.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestPrivateServiceValidateAndURL(t *testing.T) {
	valid := domain.PrivateService{
		ID:          "svc-1",
		OwnerUserID: "user-1",
		HostID:      "host-1",
		Name:        "Grafana Metrics",
		Preset:      domain.ServicePresetGrafana,
		Protocol:    domain.ServiceProtocolHTTP,
		RemoteHost:  "127.0.0.1",
		RemotePort:  3001,
		BasePath:    "/grafana",
		LocalPort:   13001,
		HealthPath:  "/api/health",
		OpenMode:    domain.ServiceOpenModeBrowser,
	}

	tests := []struct {
		name    string
		mutate  func(s *domain.PrivateService)
		wantErr error
	}{
		{name: "valid service", mutate: func(_ *domain.PrivateService) {}, wantErr: nil},
		{name: "missing owner", mutate: func(s *domain.PrivateService) { s.OwnerUserID = "" }, wantErr: domain.ErrInvalidInput},
		{name: "missing host", mutate: func(s *domain.PrivateService) { s.HostID = "" }, wantErr: domain.ErrInvalidInput},
		{name: "empty name", mutate: func(s *domain.PrivateService) { s.Name = "   " }, wantErr: domain.ErrInvalidInput},
		{name: "name too long", mutate: func(s *domain.PrivateService) { s.Name = string(make([]byte, domain.MaxServiceNameLength+1)) }, wantErr: domain.ErrInvalidInput},
		{name: "unsupported preset", mutate: func(s *domain.PrivateService) { s.Preset = "invalid" }, wantErr: domain.ErrInvalidInput},
		{name: "unsupported protocol", mutate: func(s *domain.PrivateService) { s.Protocol = "ftp" }, wantErr: domain.ErrInvalidInput},
		{name: "empty remote host", mutate: func(s *domain.PrivateService) { s.RemoteHost = "   " }, wantErr: domain.ErrInvalidInput},
		{name: "remote host whitespace", mutate: func(s *domain.PrivateService) { s.RemoteHost = "127.0. 0.1" }, wantErr: domain.ErrInvalidInput},
		{name: "invalid remote port zero", mutate: func(s *domain.PrivateService) { s.RemotePort = 0 }, wantErr: domain.ErrInvalidInput},
		{name: "invalid remote port too large", mutate: func(s *domain.PrivateService) { s.RemotePort = 70000 }, wantErr: domain.ErrInvalidInput},
		{name: "invalid local port negative", mutate: func(s *domain.PrivateService) { s.LocalPort = -1 }, wantErr: domain.ErrInvalidInput},
		{name: "base path whitespace", mutate: func(s *domain.PrivateService) { s.BasePath = "/api path" }, wantErr: domain.ErrInvalidInput},
		{name: "health path whitespace", mutate: func(s *domain.PrivateService) { s.HealthPath = "/api /health" }, wantErr: domain.ErrInvalidInput},
		{name: "unsupported open mode", mutate: func(s *domain.PrivateService) { s.OpenMode = "terminal" }, wantErr: domain.ErrInvalidInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := valid
			tc.mutate(&s)
			err := s.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}

	// Test URL helper
	u := valid.URL(13001)
	if u != "http://127.0.0.1:13001/grafana" {
		t.Errorf("URL() = %q, want http://127.0.0.1:13001/grafana", u)
	}

	rootSvc := valid
	rootSvc.BasePath = "/"
	rootSvc.Protocol = domain.ServiceProtocolHTTPS
	if rootSvc.URL(8443) != "https://127.0.0.1:8443" {
		t.Errorf("URL() root = %q, want https://127.0.0.1:8443", rootSvc.URL(8443))
	}
}
