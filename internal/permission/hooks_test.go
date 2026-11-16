package permission

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// recordingHooks is a controllable pre-permission hook: it answers every
// evaluation with its fixed decision and records human grants.
type recordingHooks struct {
	decision HookDecision
	grants   []string
}

func (h *recordingHooks) PrePermission(context.Context, PermissionRequest) PreHookResult {
	switch h.decision {
	case HookDecisionAllow:
		return PreHookResult{Decision: HookDecisionAllow}
	case HookDecisionDeny:
		return PreHookResult{Decision: HookDecisionDeny, Reason: "blocked by test"}
	default:
		return PreHookResult{Decision: HookDecisionNone}
	}
}

func (h *recordingHooks) PermissionDenied(context.Context, PermissionRequest) {}

func (h *recordingHooks) OnAutoModeGrant(sessionID string) {
	h.grants = append(h.grants, sessionID)
}

func prePermissionRequest(sessionID string) CreatePermissionRequest {
	return CreatePermissionRequest{
		SessionID:   sessionID,
		ToolCallID:  "call-1",
		ToolName:    "bash",
		Action:      "execute",
		Description: "run tests",
		Path:        "/tmp",
	}
}

func TestPrePermissionHooks_AllowSkipsPrompt(t *testing.T) {
	t.Parallel()

	hooks := &recordingHooks{decision: HookDecisionAllow}
	svc := NewPermissionService(t.TempDir(), nil, nil)
	svc.SetPermissionHooks(hooks)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	requests := svc.Subscribe(ctx)

	granted, err := svc.Request(t.Context(), prePermissionRequest("s1"))
	require.NoError(t, err)
	require.True(t, granted)

	select {
	case ev := <-requests:
		require.Fail(t, "an auto-approved request must not be published", ev)
	default:
	}
}

func TestPrePermissionHooks_DenySkipsPrompt(t *testing.T) {
	t.Parallel()

	hooks := &recordingHooks{decision: HookDecisionDeny}
	svc := NewPermissionService(t.TempDir(), nil, nil)
	svc.SetPermissionHooks(hooks)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	notifications := svc.SubscribeNotifications(ctx)

	granted, err := svc.Request(t.Context(), prePermissionRequest("s1"))
	require.NoError(t, err)
	require.False(t, granted)

	require.Eventually(t, func() bool {
		select {
		case ev := <-notifications:
			return ev.Payload.Denied && !ev.Payload.Granted
		default:
			return false
		}
	}, 2*time.Second, 5*time.Millisecond, "the hook denial must reach notification subscribers")
}

func TestPrePermissionHooks_DeferPromptsAndGrantResetsQuota(t *testing.T) {
	t.Parallel()

	hooks := &recordingHooks{decision: HookDecisionNone}
	svc := NewPermissionService(t.TempDir(), nil, nil)
	svc.SetPermissionHooks(hooks)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	requests := svc.Subscribe(ctx)

	// Resolve the prompt the way a subscriber (the UI) would.
	go func() {
		select {
		case ev := <-requests:
			svc.Grant(ev.Payload)
		case <-ctx.Done():
		}
	}()

	granted, err := svc.Request(t.Context(), prePermissionRequest("s1"))
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, []string{"s1"}, hooks.grants,
		"a human grant on an escalated request must reach the auto-mode observer")
}

func TestPrePermissionHooks_DeferDenyNoGrantObserver(t *testing.T) {
	t.Parallel()

	hooks := &recordingHooks{decision: HookDecisionNone}
	svc := NewPermissionService(t.TempDir(), nil, nil)
	svc.SetPermissionHooks(hooks)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	requests := svc.Subscribe(ctx)

	go func() {
		select {
		case ev := <-requests:
			svc.Deny(ev.Payload)
		case <-ctx.Done():
		}
	}()

	granted, err := svc.Request(t.Context(), prePermissionRequest("s1"))
	require.NoError(t, err)
	require.False(t, granted)
	require.Empty(t, hooks.grants,
		"a denial is not a human grant and must not reset the auto-mode quota")
}

// TestServiceImplementsBroker pins that the service still satisfies its
// embedded subscriber contract after the hooks addition.
var _ pubsub.Subscriber[PermissionRequest] = Service(nil)
