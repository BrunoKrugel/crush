package hooks

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPermissionPayload(t *testing.T) {
	t.Parallel()

	detail := PermissionDetail{
		ToolName:    "bash",
		Action:      "run",
		Description: "run the test suite",
	}
	payload := BuildPermissionPayload(EventPrePermission, detail, `{"user_messages":["go"]}`)
	s := string(payload)
	require.Contains(t, s, `"event":"`+EventPrePermission+`"`)
	require.Contains(t, s, `"tool_name":"bash"`)
	require.Contains(t, s, `"action":"run"`)
	require.Contains(t, s, `"user_messages":["go"]`)
}

func TestPrePermissionAllowShortCircuitsThePrompt(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"allow"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	agg, err := r.RunPermission(context.Background(), EventPrePermission, "sess", PermissionDetail{ToolName: "bash"}, "{}")
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, agg.Decision)
}

func TestPrePermissionDenyCarriesTheReason(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"deny","reason":"not on my watch"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	agg, err := r.RunPermission(context.Background(), EventPrePermission, "sess", PermissionDetail{ToolName: "bash"}, "{}")
	require.NoError(t, err)
	require.Equal(t, DecisionDeny, agg.Decision)
	require.Equal(t, "not on my watch", agg.Reason)
}

func TestPrePermissionSilenceIsNotAnOpinion(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo ok`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	agg, err := r.RunPermission(context.Background(), EventPrePermission, "sess", PermissionDetail{ToolName: "bash"}, "{}")
	require.NoError(t, err)
	require.Equal(t, DecisionNone, agg.Decision)
}

func TestPermissionDeniedFiresAndForgets(t *testing.T) {
	t.Parallel()
	hookCfg := config.HookConfig{
		Command: `echo '{"decision":"deny"}'`,
	}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	// The denied event's decisions are recorded but never enforced; the
	// call must succeed and carry the detail.
	agg, err := r.RunPermission(context.Background(), EventPermissionDenied, "sess", PermissionDetail{ToolName: "bash", Action: "run"}, "{}")
	require.NoError(t, err)
	require.Equal(t, 1, agg.HookCount)
}
