package httpprobe_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ec2-shell-desk/internal/adapters/httpprobe"
	"ec2-shell-desk/internal/core/ports"
)

func TestProberHTTPAndTLSVerificationModes(t *testing.T) {
	ctx := context.Background()
	prober := httpprobe.NewProber(2 * time.Second)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(httpSrv.Close)

	res, err := prober.Probe(ctx, ports.HealthProbeRequest{
		URL:              httpSrv.URL,
		AllowInsecureTLS: false,
	})
	if err != nil || res.HTTPStatus != http.StatusOK {
		t.Fatalf("Probe(HTTP) = (%+v, %v), want 200 OK", res, err)
	}

	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(tlsSrv.Close)

	// Strict TLS must reject self-signed certificates by default.
	if _, err := prober.Probe(ctx, ports.HealthProbeRequest{
		URL:              tlsSrv.URL,
		AllowInsecureTLS: false,
	}); err == nil {
		t.Fatal("Probe(self-signed TLS, AllowInsecureTLS=false) succeeded, want TLS verification error")
	}

	// Opt-in AllowInsecureTLS accepts the self-signed certificate over the loopback tunnel.
	tlsRes, err := prober.Probe(ctx, ports.HealthProbeRequest{
		URL:              tlsSrv.URL,
		AllowInsecureTLS: true,
	})
	if err != nil || tlsRes.HTTPStatus != http.StatusNoContent {
		t.Fatalf("Probe(self-signed TLS, AllowInsecureTLS=true) = (%+v, %v), want 204", tlsRes, err)
	}
}
