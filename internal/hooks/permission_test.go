package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPermissionEventCarriesDetailAndTranscript(t *testing.T) {
	t.Parallel()

	ev := PermissionEvent(EventPermissionRequest, "sess", PermissionDetail{
		ToolName:    "bash",
		Action:      "run",
		Description: "run the test suite",
	}, `{"user_messages":["go"]}`)
	require.NotNil(t, ev.Transcript)
	require.Contains(t, *ev.Transcript, "user_messages")
	require.NotNil(t, ev.Tool)
	require.Contains(t, ev.Tool.Input, `"action":"run"`)

	// An invalid transcript is dropped rather than breaking the payload.
	bad := PermissionEvent(EventPermissionRequest, "sess", PermissionDetail{ToolName: "bash"}, "not json")
	require.Nil(t, bad.Transcript)
}

func TestPermissionRequestAllowShortCircuitsThePrompt(t *testing.T) {
	t.Parallel()
	r := NewRunner(Static(map[string][]HookConfig{
		EventPermissionRequest: {{Command: `echo '{"decision":"allow"}'`}},
	}), t.TempDir(), t.TempDir())
	agg, _ := r.RunPermission(t.Context(), EventPermissionRequest, "sess", PermissionDetail{ToolName: "bash"}, "{}")
	require.Equal(t, DecisionAllow, agg.Decision)
}

func TestPermissionRequestDenyCarriesTheReason(t *testing.T) {
	t.Parallel()
	r := NewRunner(Static(map[string][]HookConfig{
		EventPermissionRequest: {{Command: `echo '{"decision":"deny","reason":"not on my watch"}'`}},
	}), t.TempDir(), t.TempDir())
	agg, _ := r.RunPermission(t.Context(), EventPermissionRequest, "sess", PermissionDetail{ToolName: "bash"}, "{}")
	require.Equal(t, DecisionDeny, agg.Decision)
	require.Equal(t, "not on my watch", agg.Reason)
}

func TestPermissionRequestSilenceIsNotAnOpinion(t *testing.T) {
	t.Parallel()
	r := NewRunner(Static(map[string][]HookConfig{
		EventPermissionRequest: {{Command: `echo ok`}},
	}), t.TempDir(), t.TempDir())
	agg, _ := r.RunPermission(t.Context(), EventPermissionRequest, "sess", PermissionDetail{ToolName: "bash"}, "{}")
	require.Equal(t, DecisionNone, agg.Decision)
}

func TestPermissionResultFiresAndForgets(t *testing.T) {
	t.Parallel()
	r := NewRunner(Static(map[string][]HookConfig{
		EventPermissionResult: {{Command: `echo '{"decision":"deny"}'`}},
	}), t.TempDir(), t.TempDir())
	// The result event's decisions are recorded but never enforced; the
	// call must succeed and carry the detail.
	agg, _ := r.RunPermission(t.Context(), EventPermissionResult, "sess", PermissionDetail{ToolName: "bash", Action: "run"}, "{}")
	require.Equal(t, 1, agg.HookCount)
}

// TestDeprecatedPermissionEventNames verifies the fork's pre-#3146 event
// names normalize onto upstream's.
func TestDeprecatedPermissionEventNames(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]string{
		"PrePermission":     EventPermissionRequest,
		"PermissionDenied":  EventPermissionResult,
		"pre_permission":    EventPermissionRequest,
		"permission_denied": EventPermissionResult,
	} {
		got, ok := NormalizeEvent(name)
		require.True(t, ok, name)
		require.Equal(t, want, got, name)
		require.True(t, IsDeprecatedEvent(got), name)
	}
}
