package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ec2-shell-desk/internal/core/domain"
	"ec2-shell-desk/internal/core/ports"
)

const tofuPromptTimeout = 60 * time.Second

// TOFUChallengeEvent is emitted to the frontend when an unseen SSH server requires trust confirmation.
type TOFUChallengeEvent struct {
	RequestID   string `json:"requestId"`
	HostAddress string `json:"hostAddress"`
	Port        int    `json:"port"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
}

// TerminalClosedEvent is emitted when an active SSH terminal session exits or disconnects.
type TerminalClosedEvent struct {
	SessionID string `json:"sessionId"`
	Reason    string `json:"reason"`
}

// EventBridge forwards backend terminal, SFTP transfer, and TOFU events to the Wails frontend.
type EventBridge struct {
	mu          sync.RWMutex
	ctx         context.Context
	tofuMu      sync.Mutex
	pendingTOFU map[string]chan bool
}

// NewEventBridge constructs an EventBridge ready to be attached in Wails OnStartup.
func NewEventBridge() *EventBridge {
	return &EventBridge{
		pendingTOFU: make(map[string]chan bool),
	}
}

// SetContext binds the Wails application runtime context.
func (b *EventBridge) SetContext(ctx context.Context) {
	b.mu.Lock()
	b.ctx = ctx
	b.mu.Unlock()
}

// Context returns the bound Wails runtime context, or context.Background() in unit tests.
func (b *EventBridge) Context() context.Context {
	b.mu.RLock()
	ctx := b.ctx
	b.mu.RUnlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// OnTerminalData emits base64-encoded PTY output bytes to the frontend xterm.js tab.
func (b *EventBridge) OnTerminalData(sessionID string, data []byte) {
	b.mu.RLock()
	ctx := b.ctx
	b.mu.RUnlock()
	if ctx == nil || len(data) == 0 {
		return
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	runtime.EventsEmit(ctx, "terminal:data:"+sessionID, encoded)
}

// OnTerminalClosed notifies the frontend that an SSH terminal session has closed.
func (b *EventBridge) OnTerminalClosed(sessionID, reason string) {
	b.mu.RLock()
	ctx := b.ctx
	b.mu.RUnlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, "terminal:closed:"+sessionID, TerminalClosedEvent{
		SessionID: sessionID,
		Reason:    reason,
	})
}

// OnTransferProgress emits throttled SFTP transfer progress snapshots to the frontend queue.
func (b *EventBridge) OnTransferProgress(task domain.TransferTask) {
	b.mu.RLock()
	ctx := b.ctx
	b.mu.RUnlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, "sftp:progress", task)
}

// OnTunnelStatusChanged emits live SSH tunnel state and traffic updates to the frontend.
func (b *EventBridge) OnTunnelStatusChanged(status domain.TunnelRuntimeStatus) {
	b.mu.RLock()
	ctx := b.ctx
	b.mu.RUnlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, "tunnel:status", status)
}

// RequestTOFUTrust emits a host key fingerprint prompt to the UI and waits for the user's decision.
func (b *EventBridge) RequestTOFUTrust(ctx context.Context, challenge ports.HostKeyChallenge) (bool, error) {
	b.mu.RLock()
	rtCtx := b.ctx
	b.mu.RUnlock()
	if rtCtx == nil {
		return false, fmt.Errorf("%w: runtime context is unavailable for TOFU prompt", domain.ErrHostKeyUnknown)
	}

	reqID := uuid.NewString()
	decisionCh := make(chan bool, 1)

	b.tofuMu.Lock()
	b.pendingTOFU[reqID] = decisionCh
	b.tofuMu.Unlock()

	defer func() {
		b.tofuMu.Lock()
		delete(b.pendingTOFU, reqID)
		b.tofuMu.Unlock()
	}()

	runtime.EventsEmit(rtCtx, "ssh:tofu-prompt", TOFUChallengeEvent{
		RequestID:   reqID,
		HostAddress: challenge.HostAddress,
		Port:        challenge.Port,
		KeyType:     challenge.KeyType,
		Fingerprint: challenge.Fingerprint,
	})

	timer := time.NewTimer(tofuPromptTimeout)
	defer timer.Stop()

	select {
	case decision := <-decisionCh:
		return decision, nil
	case <-ctx.Done():
		return false, fmt.Errorf("tofu prompt cancelled: %w", ctx.Err())
	case <-timer.C:
		return false, fmt.Errorf("%w: timed out waiting for TOFU confirmation", domain.ErrHostKeyUnknown)
	}
}

// ResolveTOFU delivers the user's accept/reject decision for an active TOFU challenge.
func (b *EventBridge) ResolveTOFU(requestID string, trusted bool) error {
	b.tofuMu.Lock()
	ch, ok := b.pendingTOFU[requestID]
	b.tofuMu.Unlock()
	if !ok {
		return fmt.Errorf("%w: tofu prompt request", domain.ErrNotFound)
	}
	select {
	case ch <- trusted:
	default:
	}
	return nil
}
