package model

import (
	"context"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// goalOffText stops a running goal: it is what the user types in place
// of a goal.
const goalOffText = "off"

// startGoal starts goal mode on a session, or stops a running goal when
// the goal text says "off".
//
// The call runs off the UI loop: the goal loop keeps working until the
// goal is met, its budget runs out, or the user stops it, and the UI
// learns about the outcome through the goal notification.
func (m *UI) startGoal(sessionID string, args map[string]string) tea.Cmd {
	text := strings.TrimSpace(args[dialog.GoalTextArg])
	budget := 0
	if raw := strings.TrimSpace(args[dialog.GoalBudgetArg]); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			budget = parsed
		}
	}
	stop := strings.EqualFold(text, goalOffText)
	ws := m.com.Workspace
	return func() tea.Msg {
		ctx := context.Background()
		if stop {
			if err := ws.AgentCancelGoal(ctx, sessionID); err != nil {
				return util.ReportError(err)
			}
			return util.NewInfoMsg("Goal mode stopped.")
		}
		if err := ws.AgentStartGoal(ctx, sessionID, text, budget); err != nil {
			return util.ReportError(err)
		}
		return util.NewInfoMsg("Goal mode started.")
	}
}
