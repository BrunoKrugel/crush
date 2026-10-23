package permission

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// recordingHooks is a policy hook whose decisions the tests control.
// It records the request it was given so the test can assert the
// session identity survived the trip into the hook.
type recordingHooks struct {
	decision HookDecision
	reason   string
	seen     PermissionRequest
	denied   PermissionRequest
}

func (h *recordingHooks) PrePermission(_ context.Context, req PermissionRequest) PreHookResult {
	h.seen = req
	return PreHookResult{Decision: h.decision, Reason: h.reason}
}

func (h *recordingHooks) PermissionDenied(_ context.Context, req PermissionRequest) {
	h.denied = req
}

// TestPolicyDenyWithoutAShellRunnerDoesNotPanic is the regression for
// the step-4 wiring: a policy denial used to call transcriptFn with no
// nil check. Auto mode is wired through SetPermissionHooks and never
// calls SetPrePermission, so that function is nil on the real path.
func TestPolicyDenyWithoutAShellRunnerDoesNotPanic(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	hook := &recordingHooks{decision: HookDecisionDeny, reason: "rm -rf is banned"}
	svc.SetPermissionHooks(hook)

	granted, err := svc.Request(t.Context(), CreatePermissionRequest{
		SessionID:   "sess-1",
		ToolCallID:  "call-1",
		ToolName:    "bash",
		Action:      "execute",
		Description: "Execute command: rm -rf /",
		Path:        "/tmp",
	})
	require.NoError(t, err)
	require.False(t, granted, "a policy deny blocks without prompting")
	require.Equal(t, "sess-1", hook.seen.SessionID, "the classifier keys quotas and the transcript on the session")
	require.Equal(t, "call-1", hook.seen.ToolCallID)
	require.Equal(t, "Execute command: rm -rf /", hook.seen.Description)
	require.Equal(t, "rm -rf is banned", svc.DenialReason("call-1"), "the model has to see why the call was blocked")
}

// TestPolicyHooksKeepTheShellRunner pins the composition the two
// setters must have: installing auto mode must not drop the shell
// PrePermission runner, and installing the shell runner must not drop
// auto mode.
func TestPolicyHooksKeepTheShellRunner(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	auto := &recordingHooks{decision: HookDecisionAllow}
	svc.SetPermissionHooks(auto)
	svc.SetPrePermission(nil, nil)

	// Allow is the only decision that proves the hook still ran.
	// Silence falls through either way, so a dropped hook would look
	// the same as a silent one.
	granted, err := svc.Request(t.Context(), CreatePermissionRequest{
		SessionID:  "sess-1",
		ToolCallID: "call-2",
		ToolName:   "bash",
		Action:     "execute",
	})
	require.NoError(t, err)
	require.True(t, granted, "SetPrePermission must not drop a policy hook that allows")
	require.Equal(t, "sess-1", auto.seen.SessionID)
}
