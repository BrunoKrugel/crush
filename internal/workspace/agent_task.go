package workspace

import (
	"context"
	"fmt"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/scheduler"
	"github.com/charmbracelet/crush/internal/snapshot"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"

	"github.com/charmbracelet/crush/internal/dispatch"
	"github.com/charmbracelet/crush/internal/session"
)

// AgentTask is the workspace's view of one dispatched agent (#421), as the
// A2A host's agent index describes it: what the TUI renders on the
// dispatch card, resolves an @handle against, and steers or cancels. It
// comes from the index in both workspace modes, never from the dispatch
// registry. A finished dispatch's durable record stays in the session
// store (#410); this type carries the live state.
type AgentTask struct {
	StartedAt        time.Time
	FinishedAt       time.Time
	StatusText       string
	CurrentTodo      string
	ParentSessionID  string
	Handle           string
	Role             string
	Status           dispatch.Status
	DispatchID       string
	SessionID        string
	Todos            []session.Todo
	CompletionTokens int64
	Cost             float64
	PromptTokens     int64
	TodoTotal        int
	TodoCompleted    int
	Served           bool
}

// Terminal reports whether the dispatch has ended.
func (t AgentTask) Terminal() bool {
	switch t.Status {
	case dispatch.StatusCompleted, dispatch.StatusFailed, dispatch.StatusKilled:
		return true
	default:
		return false
	}
}

// agentTaskFromDescriptor maps an index descriptor onto the workspace's
// view of it.
func dispatchStatusFromIndex(state string, served bool) dispatch.Status {
	switch state {
	case "completed", "failed", "canceled":
	default:
		if !served {
			return dispatch.StatusFailed
		}
	}
	switch state {
	case "":
		return dispatch.StatusProvisioned
	case "completed":
		return dispatch.StatusCompleted
	case "failed":
		return dispatch.StatusFailed
	case "canceled":
		return dispatch.StatusKilled
	default:
		return dispatch.StatusRunning
	}
}

// No-op implementations of the dispatched-agent surface used by the fork
// UI. The A2A host is not integrated in this build, so these always
// report "no tasks".
func (w *AppWorkspace) ListAgentTasks(sessionID string) []AgentTask  { return nil }
func (w *AppWorkspace) AgentTask(sessionID string) (AgentTask, bool) { return AgentTask{}, false }
func (w *AppWorkspace) AgentTaskByHandle(sessionID, handle string) (AgentTask, bool) {
	return AgentTask{}, false
}
func (w *AppWorkspace) SendAgentMessage(ctx context.Context, sessionID, handle, text string, attachments []message.Attachment) error {
	return nil
}
func (w *AppWorkspace) CancelAgentTask(ctx context.Context, ref string) error { return nil }

func (w *ClientWorkspace) ListAgentTasks(sessionID string) []AgentTask  { return nil }
func (w *ClientWorkspace) AgentTask(sessionID string) (AgentTask, bool) { return AgentTask{}, false }
func (w *ClientWorkspace) AgentTaskByHandle(sessionID, handle string) (AgentTask, bool) {
	return AgentTask{}, false
}
func (w *ClientWorkspace) SendAgentMessage(ctx context.Context, sessionID, handle, text string, attachments []message.Attachment) error {
	return nil
}
func (w *ClientWorkspace) CancelAgentTask(ctx context.Context, ref string) error { return nil }
func (w *ClientWorkspace) ForkSession(ctx context.Context, sessionID, messageID, title string, worktree bool) (session.Session, error) {
	return session.Session{}, fmt.Errorf("unavailable without the fork session feature")
}
func (w *AppWorkspace) ForkSession(ctx context.Context, sessionID, messageID, title string, worktree bool) (session.Session, error) {
	return session.Session{}, fmt.Errorf("unavailable without the fork session feature")
}
func (w *ClientWorkspace) AgentStartGoal(ctx context.Context, sessionID, text string, budget int) error {
	return nil
}
func (w *ClientWorkspace) AgentCancelGoal(ctx context.Context, sessionID string) error { return nil }
func (w *AppWorkspace) AgentStartGoal(ctx context.Context, sessionID, text string, budget int) error {
	return nil
}
func (w *AppWorkspace) AgentCancelGoal(ctx context.Context, sessionID string) error { return nil }

func (w *ClientWorkspace) LoadDiagnostics() []config.LoadDiagnostic { return nil }
func (w *AppWorkspace) LoadDiagnostics() []config.LoadDiagnostic    { return nil }

func (w *ClientWorkspace) ListSnapshots(ctx context.Context, sessionID string) ([]snapshot.SnapshotInfo, error) {
	return nil, nil
}
func (w *AppWorkspace) ListSnapshots(ctx context.Context, sessionID string) ([]snapshot.SnapshotInfo, error) {
	return nil, nil
}
func (w *ClientWorkspace) RestoreSnapshot(ctx context.Context, sessionID, commitHash string) error {
	return nil
}
func (w *AppWorkspace) RestoreSnapshot(ctx context.Context, sessionID, commitHash string) error {
	return nil
}
func (w *ClientWorkspace) DiffSnapshot(ctx context.Context, sessionID, commitHash string) (string, error) {
	return "", nil
}
func (w *AppWorkspace) DiffSnapshot(ctx context.Context, sessionID, commitHash string) (string, error) {
	return "", nil
}
func (w *AppWorkspace) MCPSetLazy(ctx context.Context, name string, lazy bool) error    { return nil }
func (w *ClientWorkspace) MCPSetLazy(ctx context.Context, name string, lazy bool) error { return nil }
func (w *ClientWorkspace) SetMCPLazy(workspaceID, serverID, enabled string, lazy bool) error {
	return nil
}
func (w *AppWorkspace) MCPReconnect(ctx context.Context, name string) error {
	return nil
}
func (w *ClientWorkspace) MCPReconnect(ctx context.Context, name string) error {
	return nil
}
func (w *AppWorkspace) CallMCPTool(ctx context.Context, name, toolName string, args map[string]any) (MCPToolCallResult, error) {
	return MCPToolCallResult{}, nil
}
func (w *ClientWorkspace) CallMCPTool(ctx context.Context, name, toolName string, args map[string]any) (MCPToolCallResult, error) {
	return MCPToolCallResult{}, nil
}
func (w *AppWorkspace) ReloadSkills() error                                      { return nil }
func (w *ClientWorkspace) ReloadSkills() error                                   { return nil }
func (w *AppWorkspace) ReloadModelDiscovery(ctx context.Context) (int, error)    { return 0, nil }
func (w *ClientWorkspace) ReloadModelDiscovery(ctx context.Context) (int, error) { return 0, nil }
func (w *AppWorkspace) PermissionSkipRequests() bool                             { return false }
func (w *ClientWorkspace) PermissionSkipRequests() bool                          { return false }
func (w *AppWorkspace) PermissionAutoMode() bool                                 { return false }
func (w *ClientWorkspace) PermissionAutoMode() bool                              { return false }
func (w *AppWorkspace) AgentListCronTasks(sessionID string) []scheduler.Task     { return nil }
func (w *ClientWorkspace) AgentListCronTasks(sessionID string) []scheduler.Task  { return nil }
func (w *AppWorkspace) ListAllChildSessions(ctx context.Context) ([]session.Session, error) {
	return nil, nil
}
func (w *ClientWorkspace) ListAllChildSessions(ctx context.Context) ([]session.Session, error) {
	return nil, nil
}
func (w *AppWorkspace) AgentSummarizePreview(ctx context.Context, sessionID, instructions string) (compaction.Preview, error) {
	return compaction.Preview{}, nil
}
func (w *ClientWorkspace) AgentSummarizePreview(ctx context.Context, sessionID, instructions string) (compaction.Preview, error) {
	return compaction.Preview{}, nil
}
func (w *AppWorkspace) AgentConfirmSummarize(ctx context.Context, sessionID string, preview compaction.Preview) error {
	return nil
}
func (w *ClientWorkspace) AgentConfirmSummarize(ctx context.Context, sessionID string, preview compaction.Preview) error {
	return nil
}
func (w *AppWorkspace) AgentDiscardSummarize(ctx context.Context, sessionID, checkpointID string) error {
	return nil
}
func (w *ClientWorkspace) AgentDiscardSummarize(ctx context.Context, sessionID, checkpointID string) error {
	return nil
}

func (w *AppWorkspace) PermissionSetSkipRequests(skip bool)    {}
func (w *AppWorkspace) PermissionSetAutoMode(auto bool)        {}
func (w *ClientWorkspace) PermissionSetSkipRequests(skip bool) {}
func (w *ClientWorkspace) PermissionSetAutoMode(auto bool)     {}
