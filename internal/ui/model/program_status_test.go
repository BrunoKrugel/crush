package model

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

func newProgramStatusUI(t *testing.T) *UI {
	t.Helper()
	pinTTLs(t)
	u := newBusyUI(&countingWorkspace{ready: true})
	warmCaches(u, false)
	return u
}

func runCompleteMsg(rc notify.RunComplete) tea.Msg {
	if rc.SessionID == "" {
		rc.SessionID = "s1"
	}
	return pubsub.Event[notify.RunComplete]{Type: pubsub.UpdatedEvent, Payload: rc}
}

func finished() tea.Msg { return runCompleteMsg(notify.RunComplete{}) }

func TestProgramStatus(t *testing.T) {
	cases := []struct {
		name  string
		setup func(u *UI)
		want  tea.ProgramStatus
	}{
		{
			name: "idle",
			want: tea.ProgramStatus{State: tea.ProgramStateIdle, App: "crush"},
		},
		{
			name:  "working",
			setup: func(u *UI) { u.agentBusyCache.set(true) },
			want:  tea.ProgramStatus{State: tea.ProgramStateWorking, App: "crush"},
		},
		{
			name: "question",
			setup: func(u *UI) {
				u.agentBusyCache.set(true)
				u.activeInline = dialog.NewQuestionForm(u.com.Styles, question.Request{})
			},
			want: tea.ProgramStatus{
				State: tea.ProgramStateBlocked, App: "crush",
				Kind: tea.ProgramStatusKindQuestion, Message: "Questions need your input",
			},
		},
		{
			name: "permission wins over a question",
			setup: func(u *UI) {
				u.activeInline = dialog.NewQuestionForm(u.com.Styles, question.Request{})
				u.dialog.OpenDialogWithGrace(dialog.NewPermissions(u.com, permission.PermissionRequest{ID: "p", ToolName: "bash"}))
			},
			want: tea.ProgramStatus{
				State: tea.ProgramStateBlocked, App: "crush",
				Kind: tea.ProgramStatusKindPermission, Message: "Permission required",
			},
		},
		{
			name: "aws sign-in",
			setup: func(u *UI) {
				dlg, _ := dialog.NewAWSSSO(u.com, "aws sso login")
				u.dialog.OpenDialog(dlg)
			},
			want: tea.ProgramStatus{
				State: tea.ProgramStateBlocked, App: "crush",
				Kind: tea.ProgramStatusKindAuth, Message: "AWS sign-in required",
			},
		},
		{
			name: "working wins over the last outcome",
			setup: func(u *UI) {
				u.turnOutcome = tea.ProgramStateDone
				u.agentBusyCache.set(true)
			},
			want: tea.ProgramStatus{State: tea.ProgramStateWorking, App: "crush"},
		},
		{
			name:  "outcome once idle",
			setup: func(u *UI) { u.turnOutcome = tea.ProgramStateError },
			want:  tea.ProgramStatus{State: tea.ProgramStateError, App: "crush"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := newProgramStatusUI(t)
			if c.setup != nil {
				c.setup(u)
			}
			require.Equal(t, &c.want, u.programStatus())
		})
	}
}

func TestTurnOutcome(t *testing.T) {
	cases := []struct {
		name string
		want tea.ProgramState
		msgs []tea.Msg
	}{
		{name: "finished", want: tea.ProgramStateDone, msgs: []tea.Msg{finished()}},
		{name: "failed", want: tea.ProgramStateError, msgs: []tea.Msg{runCompleteMsg(notify.RunComplete{Error: "boom"})}},
		{name: "cancelled is idle", want: "", msgs: []tea.Msg{finished(), runCompleteMsg(notify.RunComplete{Error: "context canceled", Cancelled: true})}},
		{name: "queued prompt failing after a finished one", want: tea.ProgramStateError, msgs: []tea.Msg{finished(), runCompleteMsg(notify.RunComplete{Error: "boom"})}},
		// In local mode AgentRun returns only after the turn, so this arrives
		// after the run completes and must not clear it.
		{name: "run submitted after finishing", want: tea.ProgramStateDone, msgs: []tea.Msg{finished(), agentRunSubmittedMsg{}}},
		{name: "focus clears", want: "", msgs: []tea.Msg{finished(), tea.FocusMsg{}}},
		{name: "other session ignored", want: tea.ProgramStateDone, msgs: []tea.Msg{finished(), runCompleteMsg(notify.RunComplete{SessionID: "other", Error: "boom"})}},
		{name: "other session cancel ignored", want: tea.ProgramStateDone, msgs: []tea.Msg{finished(), runCompleteMsg(notify.RunComplete{SessionID: "other", Cancelled: true})}},
		{name: "other session finish ignored", want: "", msgs: []tea.Msg{runCompleteMsg(notify.RunComplete{SessionID: "child"})}},
		{name: "switching session clears", want: "", msgs: []tea.Msg{finished(), loadSessionMsg{session: &session.Session{ID: "s2"}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := newProgramStatusUI(t)
			for _, msg := range c.msgs {
				_, cmd := u.Update(msg)
				runCmds(u, cmd)
			}
			require.Equal(t, c.want, u.turnOutcome)
		})
	}
}

func TestSendMessageClearsTurnOutcome(t *testing.T) {
	u := newProgramStatusUI(t)
	u.turnOutcome = tea.ProgramStateDone

	require.NotNil(t, u.sendMessage("next"))
	require.Empty(t, u.turnOutcome)
}

func TestNewSessionClearsTurnOutcome(t *testing.T) {
	u := newProgramStatusUI(t)
	u.turnOutcome = tea.ProgramStateError

	u.newSession()
	require.Empty(t, u.turnOutcome)
}

func TestProgramStatusCompletionLifecycle(t *testing.T) {
	pinTTLs(t)
	for _, tc := range []struct {
		name    string
		state   notify.FinishState
		prior   tea.ProgramState
		want    tea.ProgramState
		pending int
		busy    bool
	}{
		{name: "success idle", state: notify.FinishIdleSuccess, busy: false, pending: 0, prior: "", want: tea.ProgramStateDone},
		{name: "queued work", state: notify.FinishContinuing, busy: true, pending: 0, prior: "", want: tea.ProgramStateWorking},
		{name: "handoff admission gap", state: notify.FinishContinuing, busy: false, pending: 0, prior: "", want: tea.ProgramStateIdle},
		{name: "new work", state: notify.FinishIdleSuccess, busy: true, pending: 0, prior: "", want: tea.ProgramStateWorking},
		{name: "pending submission", state: notify.FinishIdleSuccess, busy: false, pending: 1, prior: "", want: tea.ProgramStateWorking},
		{name: "cancelled", state: notify.FinishIdleUnsuccessful, busy: false, pending: 0, prior: "", want: tea.ProgramStateIdle},
		{name: "cancelled after success", state: notify.FinishIdleUnsuccessful, busy: false, pending: 0, prior: tea.ProgramStateDone, want: tea.ProgramStateIdle},
		{name: "failed", state: notify.FinishIdleUnsuccessful, busy: false, pending: 0, prior: tea.ProgramStateError, want: tea.ProgramStateError},
		{name: "legacy", state: "", busy: false, pending: 0, prior: "", want: tea.ProgramStateIdle},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newBusyUI(&countingWorkspace{ready: true, agentBusy: tc.busy})
			warmCaches(u, true)
			u.turnOutcome = tc.prior
			u.pendingSubmissions = tc.pending
			u.activityFor("s1").pending = tc.pending
			runCmds(u, u.handleAgentNotification(notify.Notification{
				Type: notify.TypeAgentFinished, FinishState: tc.state, SessionID: "s1",
			}))
			require.Equal(t, tc.want, u.programStatus().State)
		})
	}
}

func TestProgramStatusIgnoresOtherSessionOutcomes(t *testing.T) {
	pinTTLs(t)
	u := newBusyUI(&countingWorkspace{ready: true})
	warmCaches(u, false)
	for _, typ := range []notify.Type{notify.TypeAgentFinished, notify.TypeAgentError} {
		runCmds(u, u.handleAgentNotification(notify.Notification{
			Type: typ, FinishState: notify.FinishIdleSuccess, SessionID: "s2",
		}))
		require.Equal(t, tea.ProgramStateIdle, u.programStatus().State)
	}

	u.turnOutcome = tea.ProgramStateDone
	u.sendMessage("new work")
	t.Cleanup(func() { common.StopTurn("s1") })
	require.Empty(t, u.turnOutcome)
	_, cmd := u.Update(agentRunSubmittedMsg{sessionID: "s1", gen: u.submissionGen})
	runCmds(u, cmd)
	u.turnOutcome = tea.ProgramStateError
	u.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
	require.Empty(t, u.turnOutcome)
}

func TestProgramStatusSubmissionReturn(t *testing.T) {
	pinTTLs(t)
	for _, tc := range []struct {
		name string
		err  error
		want tea.ProgramState
	}{
		{"completed before return", nil, tea.ProgramStateDone},
		{"cancelled", context.Canceled, tea.ProgramStateIdle},
		{"failed", context.DeadlineExceeded, tea.ProgramStateError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newBusyUI(&countingWorkspace{ready: true})
			warmCaches(u, true)
			u.pendingSubmissions = 1
			u.activityFor("s1").pending = 1
			runCmds(u, u.handleAgentNotification(notify.Notification{
				Type: notify.TypeAgentFinished, FinishState: notify.FinishIdleSuccess, SessionID: "s1",
			}))
			require.Equal(t, tea.ProgramStateWorking, u.programStatus().State)
			_, cmd := u.Update(agentRunSubmittedMsg{sessionID: "s1", err: tc.err})
			runCmds(u, cmd)
			require.Equal(t, tc.want, u.programStatus().State)
		})
	}
}
