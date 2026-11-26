package ports

import "context"

// HealthProbeRequest configures a single outbound HTTP/HTTPS health probe against a tunneled service.
type HealthProbeRequest struct {
	URL              string
	AllowInsecureTLS bool
}

// HealthProbeResult carries the HTTP status code and round-trip latency of a completed health probe.
type HealthProbeResult struct {
	HTTPStatus int
	LatencyMs  int64
}

// HealthProber performs outbound HTTP/HTTPS health checks for private observability services.
type HealthProber interface {
	Probe(ctx context.Context, req HealthProbeRequest) (HealthProbeResult, error)
}
