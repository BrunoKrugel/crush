package workspace

import (
	"time"

	"github.com/charmbracelet/crush/internal/a2a"
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
	descriptor       a2a.AgentDescriptor
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
func agentTaskFromDescriptor(d a2a.AgentDescriptor) AgentTask {
	task := AgentTask{
		DispatchID:      d.ID,
		SessionID:       d.ContextID,
		ParentSessionID: d.ParentSessionID,
		Handle:          d.Handle,
		Role:            d.Role,
		Status:          dispatchStatusFromIndex(d.State, d.Served),
		StatusText:      d.StatusText,
		Served:          d.Served,
		StartedAt:       d.StartedAt,
		descriptor:      d,
	}
	if d.FinishedAt != nil {
		task.FinishedAt = *d.FinishedAt
	}
	if p := d.Progress; p != nil {
		task.CurrentTodo = p.Current
		task.TodoCompleted = p.Completed
		task.TodoTotal = p.Total
		task.Todos = make([]session.Todo, 0, len(p.Todos))
		for _, item := range p.Todos {
			task.Todos = append(task.Todos, session.Todo{
				Content:    item.Content,
				Status:     session.TodoStatus(item.Status),
				ActiveForm: item.ActiveForm,
			})
		}
	}
	if u := d.Usage; u != nil {
		task.PromptTokens = u.PromptTokens
		task.CompletionTokens = u.CompletionTokens
		task.Cost = u.Cost
	}
	return task
}

// dispatchStatusFromIndex maps an index state token onto the dispatch
// vocabulary the TUI renders. A canceled task is a killed dispatch: the
// only way a dispatch's task is canceled is a kill. A route that went
// down before its task reached a terminal state — the stream failed
// before the dispatch's first message, say — ended without finishing,
// which the TUI shows as failed rather than live forever.
func dispatchStatusFromIndex(state string, served bool) dispatch.Status {
	switch state {
	case a2a.DispatchStatusCompleted, a2a.DispatchStatusFailed, a2a.DispatchStatusCanceled:
	default:
		if !served {
			return dispatch.StatusFailed
		}
	}
	switch state {
	case "":
		return dispatch.StatusProvisioned
	case a2a.DispatchStatusCompleted:
		return dispatch.StatusCompleted
	case a2a.DispatchStatusFailed:
		return dispatch.StatusFailed
	case a2a.DispatchStatusCanceled:
		return dispatch.StatusKilled
	default:
		return dispatch.StatusRunning
	}
}
