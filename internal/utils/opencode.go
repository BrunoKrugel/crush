// Package utils provides shared internal helpers for the Crush CLI.
package utils

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// OpenCode WAF routing headers. OpenCode Zen/Go strictly validate the
// presence, format, and behavior of these headers; requests missing them
// (or carrying malformed values) are rejected with 403 Forbidden.
const (
	// OpenCodeUserAgent is the exact User-Agent fingerprint the WAF expects.
	OpenCodeUserAgent = "opencode/1.18.31 ai-sdk/provider-utils/4.0.40 runtime/bun/1.3.14"
	// OpenCodeClient is the exact value for the x-opencode-client header.
	OpenCodeClient = "cli"
)

// base62Charset is the alphabet for the random 14-char ID suffix.
const base62Charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// hexMask keeps the packed timestamp+counter to 6 bytes (12 hex chars).
const hexMask uint64 = 0xFFFFFFFFFFFF

// counter is a process-wide uint16 sequence mixed into every generated ID
// so IDs created within the same millisecond stay unique. Accessed atomically.
var counter atomic.Uint32

// normalizePrefix accepts both "ses" and "ses_" and always returns the
// underscored form, keeping total ID length at 30 chars.
func normalizePrefix(prefix string) string {
	if strings.HasSuffix(prefix, "_") {
		return prefix
	}
	return prefix + "_"
}

// GenerateOpenCodeID builds an ID in OpenCode's internal schema for the
// given prefix (e.g. "msg_", "ses_", "ses", "prj_").
//
// Layout (total 30 chars): prefix (4) + 12-char lowercase hex + 14-char
// random Base62 suffix.
//
// The 6-byte hex segment packs time.Now().UnixMilli() and a thread-safe
// uint16 counter as (timestamp << 12) | counter. When descending is true,
// a bitwise NOT is applied to the packed value before hex formatting
// (used for session IDs so newer sessions sort first).
func GenerateOpenCodeID(prefix string, descending bool) string {
	p := normalizePrefix(prefix)

	c := uint16(counter.Add(1) & 0xFFFF)
	ts := uint64(time.Now().UnixMilli())
	v := (ts << 12) | uint64(c)
	if descending {
		v = ^v
	}
	v &= hexMask
	hexPart := fmt.Sprintf("%012x", v)

	var rnd [14]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		// crypto/rand essentially never fails; fall back to a
		// time+counter mix so we still return a well-formed ID.
		var fb [16]byte
		binary.LittleEndian.PutUint64(fb[0:8], uint64(time.Now().UnixNano()))
		binary.LittleEndian.PutUint64(fb[8:16], uint64(counter.Load()))
		copy(rnd[:], fb[:14])
	}
	suffix := make([]byte, 14)
	for i, b := range rnd {
		suffix[i] = base62Charset[int(b)%len(base62Charset)]
	}

	return p + hexPart + string(suffix)
}

// projectOnce guards the process-stable project ID.
var (
	projectOnce sync.Once
	projectID   string
)

// GetProjectID returns the process-stable x-opencode-project value,
// generated once via GenerateOpenCodeID("prj", false).
func GetProjectID() string {
	projectOnce.Do(func() {
		projectID = GenerateOpenCodeID("prj", false)
	})
	return projectID
}

// sessionIDs maps a Crush conversation (crush session ID) to its stable
// OpenCode session ID so multi-turn prompts reuse one ses_ value and hit
// the vendor KV cache.
var (
	sessionMu      sync.Mutex
	sessionIDs     = make(map[string]string)
	fallbackOnce   sync.Once
	fallbackSessID string
)

// GetSessionID returns the stable OpenCode ses_ ID for a Crush
// conversation. Empty input yields a process-stable fallback.
func GetSessionID(crushSessionID string) string {
	if crushSessionID == "" {
		fallbackOnce.Do(func() {
			fallbackSessID = GenerateOpenCodeID("ses", true)
		})
		return fallbackSessID
	}
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if id, ok := sessionIDs[crushSessionID]; ok {
		return id
	}
	id := GenerateOpenCodeID("ses", true)
	sessionIDs[crushSessionID] = id
	return id
}

// NewRequestID returns a fresh msg_ ID for a single outgoing LLM request.
func NewRequestID() string {
	return GenerateOpenCodeID("msg", false)
}

// crushSessionKey carries the Crush session ID through request contexts.
type crushSessionKey struct{}

// WithCrushSessionID tags ctx with the Crush conversation ID so the HTTP
// transport can resolve the stable x-opencode-session value per request.
func WithCrushSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, crushSessionKey{}, sessionID)
}

// crushSessionIDFromContext returns the Crush session ID tagged by
// WithCrushSessionID, or "" when absent.
func crushSessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(crushSessionKey{}).(string); ok {
		return v
	}
	return ""
}

// openCodeTransport is an http.RoundTripper that injects the OpenCode WAF
// routing headers on every outgoing request:
//
//   - User-Agent: exact CLI fingerprint
//   - x-opencode-client: "cli"
//   - x-opencode-project: process-stable prj_ ID
//   - x-opencode-session: per-conversation stable ses_ ID (KV-cache affinity)
//   - x-opencode-request: fresh msg_ ID per request
//
// Explicit user-configured project/session values are respected; the
// request ID is always regenerated so each LLM call carries a unique one.
type openCodeTransport struct {
	base http.RoundTripper
}

// NewOpenCodeTransport wraps base (or http.DefaultTransport when nil) with
// per-request OpenCode WAF header injection.
func NewOpenCodeTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &openCodeTransport{base: base}
}

// RoundTrip implements http.RoundTripper.
func (t *openCodeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	h := r.Header
	h.Set("User-Agent", OpenCodeUserAgent)
	h.Set("x-opencode-client", OpenCodeClient)
	if h.Get("x-opencode-project") == "" {
		h.Set("x-opencode-project", GetProjectID())
	}
	if h.Get("x-opencode-session") == "" {
		h.Set("x-opencode-session", GetSessionID(crushSessionIDFromContext(r.Context())))
	}
	h.Set("x-opencode-request", NewRequestID())
	return t.base.RoundTrip(r)
}
