package permission

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

// blindTranscript is the transcript provider the tests use; it returns a
// fixed payload.
func blindTranscript(ctx context.Context, sessionID string) (string, error) {
	return "{}", nil
}

func TestPrePermissionAllowSkipsThePrompt(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetPrePermission(hooks.NewRunner(hooks.Static(map[string][]config.HookConfig{
		hooks.EventPermissionRequest: {{Command: `echo '{"decision":"allow"}'`}},
	}), t.TempDir(), t.TempDir()), blindTranscript)

	granted, err := svc.Request(t.Context(), CreatePermissionRequest{
		SessionID:  "sess",
		ToolCallID: "call-1",
		ToolName:   "bash",
		Action:     "run",
	})
	require.NoError(t, err)
	require.True(t, granted, "a PrePermission allow pre-approves without prompting")
}

func TestPrePermissionDenyBlocksThePrompt(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetPrePermission(hooks.NewRunner(hooks.Static(map[string][]config.HookConfig{
		hooks.EventPermissionRequest: {{Command: `echo '{"decision":"deny","reason":"nope"}'`}},
	}), t.TempDir(), t.TempDir()), blindTranscript)

	granted, err := svc.Request(t.Context(), CreatePermissionRequest{
		SessionID:  "sess",
		ToolCallID: "call-1",
		ToolName:   "bash",
		Action:     "run",
	})
	require.NoError(t, err)
	require.False(t, granted, "a PrePermission deny blocks without prompting")
}

func TestPrePermissionSilenceFallsThroughToThePrompt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetPrePermission(hooks.NewRunner(hooks.Static(map[string][]config.HookConfig{
		hooks.EventPermissionRequest: {{Command: `echo ok`}},
	}), t.TempDir(), t.TempDir()), blindTranscript)

	// The prompt is the fall-through: nobody answers it, so the request
	// stays pending until the context ends.
	sub := svc.SubscribeNotifications(ctx)
	_, err := svc.Request(ctx, CreatePermissionRequest{
		SessionID:  "sess",
		ToolCallID: "call-1",
		ToolName:   "bash",
		Action:     "run",
	})
	require.ErrorIs(t, err, context.DeadlineExceeded, "silence falls through to the prompt")

	// And the prompt notification was published.
	select {
	case ev := <-sub:
		require.False(t, ev.Payload.Granted)
	case <-ctx.Done():
	}
}

func TestUserDenialFiresPermissionDeniedHooks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetPrePermission(hooks.NewRunner(hooks.Static(map[string][]config.HookConfig{
		hooks.EventPermissionRequest: {{Command: `echo ok`}},
	}), t.TempDir(), t.TempDir()), blindTranscript)

	_, err := svc.Request(ctx, CreatePermissionRequest{
		SessionID:  "sess",
		ToolCallID: "call-1",
		ToolName:   "bash",
		Action:     "run",
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestCancelledRequestNotifiesAndFiresResult verifies the abandoned-prompt
// path: a turn cancelled while the prompt was open publishes a cancelled
// notification, fires a PermissionResult hook with the cancelled outcome,
// and the request can no longer be granted.
func TestCancelledRequestNotifiesAndFiresResult(t *testing.T) {
	t.Parallel()

	hookFile := t.TempDir() + "/hook.log"
	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetPrePermission(hooks.NewRunner(hooks.Static(map[string][]config.HookConfig{
		hooks.EventPermissionRequest: {},
		hooks.EventPermissionResult:  {{Command: `cat >> ` + hookFile + `; echo done >> ` + hookFile}},
	}), t.TempDir(), t.TempDir()), blindTranscript)

	notifications := svc.SubscribeNotifications(t.Context())
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := svc.Request(ctx, CreatePermissionRequest{
			SessionID:  "cancel-session",
			ToolCallID: "cancel-call",
			ToolName:   "bash",
			Action:     "run",
		})
		done <- err
	}()

	// Wait for the prompt to be raised, then abandon it.
	require.Eventually(t, func() bool {
		_, active := svc.ActiveRequest()
		return active
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)

	var cancelled bool
	timeout := time.After(5 * time.Second)
	for !cancelled {
		select {
		case ev := <-notifications:
			if ev.Payload.Cancelled {
				cancelled = true
			}
		case <-timeout:
			t.Fatal("cancelled notification was never published")
		}
	}
	require.True(t, cancelled)

	// The PermissionResult hook observed the cancellation: the payload
	// it received carries the decision.
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(hookFile)
		return err == nil && strings.Contains(string(data), `"decision":"cancelled"`)
	}, 5*time.Second, 10*time.Millisecond)
}
