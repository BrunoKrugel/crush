package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

// TestSessionEventUpdatesTodoPill pins the todo pill's freshness contract: a
// session UpdatedEvent that carries an advanced todo list must move the pill
// to the newly in-progress task. A report of the pill lagging two todo calls
// behind the model's actual step pointed here.
func TestSessionEventUpdatesTodoPill(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Content: "step one", Status: session.TodoStatusInProgress, ActiveForm: "doing step one"},
		{Content: "step two", Status: session.TodoStatusPending},
		{Content: "step three", Status: session.TodoStatusPending},
		{Content: "step four", Status: session.TodoStatusPending},
	}}
	u.promptQueue = 0
	u.pillsExpanded = true
	u.updateLayoutAndSize()
	u.renderPills()
	if !strings.Contains(u.pillsView, "step one") {
		t.Fatalf("precondition: the collapsed pill names the in-progress task:\n%s", u.pillsView)
	}

	// The todos tool saved step three as in-progress and published the row.
	updated := *u.session
	updated.Todos = []session.Todo{
		{Content: "step one", Status: session.TodoStatusCompleted},
		{Content: "step two", Status: session.TodoStatusCompleted},
		{Content: "step three", Status: session.TodoStatusInProgress, ActiveForm: "doing step three"},
		{Content: "step four", Status: session.TodoStatusPending},
	}
	updated.PromptTokens = 1
	if _, cmd := u.Update(pubsub.Event[session.Session]{Type: pubsub.UpdatedEvent, Payload: updated}); cmd != nil {
		_ = cmd
	}

	if u.session.Todos[2].Status != session.TodoStatusInProgress {
		t.Fatalf("the UI session did not adopt the event's todos: %+v", u.session.Todos)
	}
	if !strings.Contains(u.pillsView, "step three") {
		t.Fatalf("the pill still names the stale task:\n%s", u.pillsView)
	}
	if strings.Contains(u.pillsView, "doing step one") {
		t.Fatalf("the pill still shows the stale active form:\n%s", u.pillsView)
	}
}

// TestSessionEventFailsOpenForOtherSessions: a session event for a DIFFERENT
// session (a dispatched agent's row) must not clobber the viewed session.
func TestSessionEventFailsOpenForOtherSessions(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Content: "step one", Status: session.TodoStatusInProgress},
	}}
	other := session.Session{ID: "child-session", Todos: nil}
	_, _ = u.Update(pubsub.Event[session.Session]{Type: pubsub.UpdatedEvent, Payload: other})

	if u.session.ID != "s1" {
		t.Fatalf("another session's event replaced the viewed session: %s", u.session.ID)
	}
	if len(u.session.Todos) != 1 {
		t.Fatalf("another session's event wiped the viewed session's todos: %+v", u.session.Todos)
	}
}

// TestTodoSpinnerStartsOnFirstInProgress keeps the spinner edge pinned: the
// first in_progress todo arms the spinner; later status changes must not stop
// it while the session is busy.
func TestTodoSpinnerStartsOnFirstInProgress(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Content: "step one", Status: session.TodoStatusPending},
	}}
	warmCaches(u, true) // busy

	updated := *u.session
	updated.Todos = []session.Todo{
		{Content: "step one", Status: session.TodoStatusInProgress},
	}
	_, _ = u.Update(pubsub.Event[session.Session]{Type: pubsub.UpdatedEvent, Payload: updated})
	if !u.todoIsSpinning {
		t.Fatal("the first in-progress todo must start the spinner")
	}

	// Step two goes in-progress: the spinner must keep running.
	updated.Todos = []session.Todo{
		{Content: "step one", Status: session.TodoStatusCompleted},
		{Content: "step two", Status: session.TodoStatusInProgress},
	}
	_, _ = u.Update(pubsub.Event[session.Session]{Type: pubsub.UpdatedEvent, Payload: updated})
	if !u.todoIsSpinning {
		t.Fatal("the spinner must not stop on a later todo advance")
	}
}

var _ = notify.TypeAgentFinished
