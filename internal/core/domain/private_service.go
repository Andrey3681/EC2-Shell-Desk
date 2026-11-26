package domain

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ServicePreset defines predefined templates for observability and web services.
type ServicePreset string

const (
	// ServicePresetGrafana identifies Grafana dashboard service.
	ServicePresetGrafana ServicePreset = "grafana"
	// ServicePresetPrometheus identifies Prometheus metrics server.
	ServicePresetPrometheus ServicePreset = "prometheus"
	// ServicePresetLoki identifies Grafana Loki log aggregation API.
	ServicePresetLoki ServicePreset = "loki"
	// ServicePresetMailpit identifies Mailpit / MailHog email testing UI.
	ServicePresetMailpit ServicePreset = "mailpit"
	// ServicePresetPortainer identifies Portainer container management UI.
	ServicePresetPortainer ServicePreset = "portainer"
	// ServicePresetNetdata identifies Netdata real-time monitoring agent.
	ServicePresetNetdata ServicePreset = "netdata"
	// ServicePresetUptimeKuma identifies Uptime Kuma monitoring tool.
	ServicePresetUptimeKuma ServicePreset = "uptime_kuma"
	// ServicePresetCustom identifies user-defined web services.
	ServicePresetCustom ServicePreset = "custom"
)

// ServiceProtocol defines the communication scheme (http or https).
type ServiceProtocol string

const (
	// ServiceProtocolHTTP represents cleartext HTTP.
	ServiceProtocolHTTP ServiceProtocol = "http"
	// ServiceProtocolHTTPS represents secure HTTP.
	ServiceProtocolHTTPS ServiceProtocol = "https"
)

// ServiceOpenMode defines where the service is displayed.
type ServiceOpenMode string

const (
	// ServiceOpenModeBrowser opens the service in the default external system browser.
	ServiceOpenModeBrowser ServiceOpenMode = "browser"
	// ServiceOpenModeEmbedded opens the service in an internal desktop application tab.
	ServiceOpenModeEmbedded ServiceOpenMode = "embedded"
)

// ServiceHealthStatus reflects the HTTP probe result.
type ServiceHealthStatus string

const (
	// ServiceHealthUnknown indicates the service has not been probed yet.
	ServiceHealthUnknown ServiceHealthStatus = "unknown"
	// ServiceHealthHealthy indicates the probe returned an expected successful HTTP response (e.g. 2xx/3xx).
	ServiceHealthHealthy ServiceHealthStatus = "healthy"
	// ServiceHealthDegraded indicates non-terminal status or unexpected response code.
	ServiceHealthDegraded ServiceHealthStatus = "degraded"
	// ServiceHealthUnhealthy indicates connection refused or error.
	ServiceHealthUnhealthy ServiceHealthStatus = "unhealthy"
)

// MaxServiceNameLength defines max character length for private service names.
const MaxServiceNameLength = 128

// PrivateService represents a user-configured internal web service running on an EC2 instance / VM.
type PrivateService struct {
	ID               string          `json:"id"`
	OwnerUserID      string          `json:"ownerUserId"`
	HostID           string          `json:"hostId"`
	Name             string          `json:"name"`
	Preset           ServicePreset   `json:"preset"`
	Protocol         ServiceProtocol `json:"protocol"`
	RemoteHost       string          `json:"remoteHost"`
	RemotePort       int             `json:"remotePort"`
	BasePath         string          `json:"basePath"`
	LocalPort        int             `json:"localPort"`
	HealthPath       string          `json:"healthPath"`
	OpenMode         ServiceOpenMode `json:"openMode"`
	AllowInsecureTLS bool            `json:"allowInsecureTls"`
	Notes            string          `json:"notes"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

// Validate verifies constraints for PrivateService fields.
func (s PrivateService) Validate() error {
	if strings.TrimSpace(s.OwnerUserID) == "" {
		return fmt.Errorf("%w: owner user id is required", ErrInvalidInput)
	}
	if strings.TrimSpace(s.HostID) == "" {
		return fmt.Errorf("%w: host id is required", ErrInvalidInput)
	}
	name := strings.TrimSpace(s.Name)
	if name == "" || len(name) > MaxServiceNameLength {
		return fmt.Errorf("%w: service name must be between 1 and %d characters", ErrInvalidInput, MaxServiceNameLength)
	}
	switch s.Preset {
	case "", ServicePresetGrafana, ServicePresetPrometheus, ServicePresetLoki,
		ServicePresetMailpit, ServicePresetPortainer, ServicePresetNetdata,
		ServicePresetUptimeKuma, ServicePresetCustom:
	default:
		return fmt.Errorf("%w: unsupported service preset %q", ErrInvalidInput, s.Preset)
	}
	switch s.Protocol {
	case "", ServiceProtocolHTTP, ServiceProtocolHTTPS:
	default:
		return fmt.Errorf("%w: unsupported service protocol %q", ErrInvalidInput, s.Protocol)
	}
	remoteHost := strings.TrimSpace(s.RemoteHost)
	if remoteHost == "" || strings.ContainsAny(remoteHost, " \t\r\n") {
		return fmt.Errorf("%w: remote host must be non-empty and without whitespace", ErrInvalidInput)
	}
	if s.RemotePort < 1 || s.RemotePort > 65535 {
		return fmt.Errorf("%w: remote port must be between 1 and 65535", ErrInvalidInput)
	}
	if s.LocalPort < 0 || s.LocalPort > 65535 {
		return fmt.Errorf("%w: local port must be between 0 and 65535", ErrInvalidInput)
	}
	if strings.ContainsAny(s.BasePath, " \t\r\n") {
		return fmt.Errorf("%w: base path cannot contain whitespace", ErrInvalidInput)
	}
	if strings.ContainsAny(s.HealthPath, " \t\r\n") {
		return fmt.Errorf("%w: health path cannot contain whitespace", ErrInvalidInput)
	}
	switch s.OpenMode {
	case "", ServiceOpenModeBrowser, ServiceOpenModeEmbedded:
	default:
		return fmt.Errorf("%w: unsupported service open mode %q", ErrInvalidInput, s.OpenMode)
	}
	return nil
}

// NormalizedBasePath returns sanitized base path starting with / or empty string if root.
func (s PrivateService) NormalizedBasePath() string {
	raw := strings.TrimSpace(s.BasePath)
	if raw == "" || raw == "/" {
		return ""
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return strings.TrimRight(raw, "/")
}

// NormalizedHealthPath returns the effective health probe path.
func (s PrivateService) NormalizedHealthPath() string {
	raw := strings.TrimSpace(s.HealthPath)
	if raw == "" {
		base := s.NormalizedBasePath()
		if base == "" {
			return "/"
		}
		return base
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return raw
}

// ServiceRuntimeStatus represents real-time operational state of a private service tunnel.
type ServiceRuntimeStatus struct {
	ServiceID    string              `json:"serviceId"`
	OwnerUserID  string              `json:"ownerUserId"`
	State        TunnelState         `json:"state"`
	LocalURL     string              `json:"localUrl"`
	LocalPort    int                 `json:"localPort"`
	HealthStatus ServiceHealthStatus `json:"healthStatus"`
	HTTPStatus   int                 `json:"httpStatus"`
	LatencyMs    int64               `json:"latencyMs"`
	ErrorMessage string              `json:"errorMessage,omitempty"`
	LastChecked  *time.Time          `json:"lastChecked,omitempty"`
}

// URL constructs a full loopback URL for the service based on local bind port and base path.
func (s PrivateService) URL(localPort int) string {
	proto := string(s.Protocol)
	if proto == "" {
		proto = string(ServiceProtocolHTTP)
	}
	base := s.NormalizedBasePath()
	u := url.URL{
		Scheme: proto,
		Host:   fmt.Sprintf("127.0.0.1:%d", localPort),
		Path:   base,
	}
	return u.String()
}
