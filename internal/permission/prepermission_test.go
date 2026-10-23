package permission

import (
	"context"
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
	svc.SetPrePermission(hooks.NewRunner([]config.HookConfig{
		{Command: `echo '{"decision":"allow"}'`},
	}, t.TempDir(), t.TempDir()), blindTranscript)

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
	svc.SetPrePermission(hooks.NewRunner([]config.HookConfig{
		{Command: `echo '{"decision":"deny","reason":"nope"}'`},
	}, t.TempDir(), t.TempDir()), blindTranscript)

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
	svc.SetPrePermission(hooks.NewRunner([]config.HookConfig{
		{Command: `echo ok`},
	}, t.TempDir(), t.TempDir()), blindTranscript)

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
	svc.SetPrePermission(hooks.NewRunner([]config.HookConfig{
		{Command: `echo ok`},
	}, t.TempDir(), t.TempDir()), blindTranscript)

	_, err := svc.Request(ctx, CreatePermissionRequest{
		SessionID:  "sess",
		ToolCallID: "call-1",
		ToolName:   "bash",
		Action:     "run",
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
