package httpprobe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const defaultProbeTimeout = 3 * time.Second

// Prober implements ports.HealthProber with reusable strict and opt-in insecure HTTP transports.
type Prober struct {
	strictClient   *http.Client
	insecureClient *http.Client
}

// NewProber constructs a Prober with shared HTTP clients configured with DisableKeepAlives for tunneled probes.
func NewProber(timeout time.Duration) *Prober {
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}

	strictTransport := &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	insecureTransport := &http.Transport{
		DisableKeepAlives: true,
		// #nosec G402 -- opt-in only when AllowInsecureTLS is enabled for internal self-signed endpoints over SSH loopback.
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		},
	}

	return &Prober{
		strictClient: &http.Client{
			Timeout:   timeout,
			Transport: strictTransport,
		},
		insecureClient: &http.Client{
			Timeout:   timeout,
			Transport: insecureTransport,
		},
	}
}

// Probe performs an HTTP GET request to req.URL using either the strict or opt-in insecure client.
func (p *Prober) Probe(ctx context.Context, req ports.HealthProbeRequest) (ports.HealthProbeResult, error) {
	targetURL := strings.TrimSpace(req.URL)
	if targetURL == "" {
		return ports.HealthProbeResult{}, fmt.Errorf("%w: probe url is required", domain.ErrInvalidInput)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return ports.HealthProbeResult{}, fmt.Errorf("build health probe request: %w", err)
	}

	client := p.strictClient
	if req.AllowInsecureTLS {
		client = p.insecureClient
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return ports.HealthProbeResult{LatencyMs: latency}, fmt.Errorf("execute health probe: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return ports.HealthProbeResult{
		HTTPStatus: resp.StatusCode,
		LatencyMs:  latency,
	}, nil
}
