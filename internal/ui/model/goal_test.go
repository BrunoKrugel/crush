package model

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// goalTestWorkspace records the goal calls the UI makes.
type goalTestWorkspace struct {
	channelsTestWorkspace
	started   bool
	cancelled bool
	sessionID string
	text      string
	budget    int
	err       error
}

func (w *goalTestWorkspace) AgentStartGoal(_ context.Context, sessionID, text string, budget int) error {
	w.started = true
	w.sessionID = sessionID
	w.text = text
	w.budget = budget
	return w.err
}

func (w *goalTestWorkspace) AgentCancelGoal(_ context.Context, sessionID string) error {
	w.cancelled = true
	w.sessionID = sessionID
	return w.err
}

// TestStartGoalStartsTheGoal pins the request: the session, the goal
// text, and the budget reach the workspace.
func TestStartGoalStartsTheGoal(t *testing.T) {
	t.Parallel()
	ws := &goalTestWorkspace{}
	m := &UI{com: &common.Common{Workspace: ws}}

	m.startGoal("session-1", map[string]string{
		dialog.GoalTextArg:   "make the tests green",
		dialog.GoalBudgetArg: "5",
	})()

	require.True(t, ws.started)
	require.False(t, ws.cancelled)
	require.Equal(t, "session-1", ws.sessionID)
	require.Equal(t, "make the tests green", ws.text)
	require.Equal(t, 5, ws.budget)
}

// TestStartGoalWithoutABudgetUsesTheDefault pins that an empty budget
// means the goal mode's own default, not zero.
func TestStartGoalWithoutABudgetUsesTheDefault(t *testing.T) {
	t.Parallel()
	ws := &goalTestWorkspace{}
	m := &UI{com: &common.Common{Workspace: ws}}

	m.startGoal("session-1", map[string]string{
		dialog.GoalTextArg: "make the tests green",
	})()

	require.True(t, ws.started)
	require.Equal(t, 0, ws.budget, "an unset budget is the goal mode's default")
}

// TestStartGoalOffStopsTheGoal pins that "off" stops a running goal
// instead of starting a new one.
func TestStartGoalOffStopsTheGoal(t *testing.T) {
	t.Parallel()
	ws := &goalTestWorkspace{}
	m := &UI{com: &common.Common{Workspace: ws}}

	m.startGoal("session-1", map[string]string{
		dialog.GoalTextArg: "off",
	})()

	require.True(t, ws.cancelled)
	require.False(t, ws.started, "\"off\" stops the goal, it does not start one")
	require.Equal(t, "session-1", ws.sessionID)
}
