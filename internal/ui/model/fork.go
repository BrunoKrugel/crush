package model

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// forkSessionMsg carries the outcome of a fork back to the UI.
type forkSessionMsg struct {
	session session.Session
	err     error
}

// openForkDialog opens the fork dialog for a session, forking at its
// last user message. Forking from the last prompt is the common case:
// the user wants to try a different direction from where they are now.
func (m *UI) openForkDialog(sessionID string) tea.Cmd {
	if m.dialog.ContainsDialog(dialog.ForkID) {
		m.dialog.BringToFront(dialog.ForkID)
		return nil
	}
	messageID, err := m.lastUserMessageID(sessionID)
	if err != nil {
		return util.ReportError(err)
	}
	if messageID == "" {
		return util.ReportWarn("There is no prompt to fork from yet.")
	}
	m.dialog.OpenDialogWithGrace(dialog.NewFork(m.com, sessionID, messageID))
	return nil
}

// lastUserMessageID returns the id of the session's last user message.
func (m *UI) lastUserMessageID(sessionID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	messages, err := m.com.Workspace.ListUserMessages(ctx, sessionID)
	if err != nil {
		return "", err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.User && !messages[i].Content().Hidden {
			return messages[i].ID, nil
		}
	}
	return "", nil
}

// forkSession runs the fork off the UI loop and switches to the new
// session when it succeeds.
func (m *UI) forkSession(sourceSessionID, messageID, title string, worktree bool) tea.Cmd {
	ws := m.com.Workspace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		forked, err := ws.ForkSession(ctx, sourceSessionID, messageID, title, worktree)
		if err != nil {
			return forkSessionMsg{err: err}
		}
		return forkSessionMsg{session: forked}
	}
}

// applyForkSession switches the UI to a freshly forked session.
func (m *UI) applyForkSession(msg forkSessionMsg) tea.Cmd {
	if msg.err != nil {
		slog.Error("Failed to fork the conversation", "error", msg.err)
		return util.ReportError(fmt.Errorf("could not fork the conversation: %w", msg.err))
	}
	return tea.Batch(
		util.CmdHandler(util.NewInfoMsg("Forked the conversation.")),
		m.loadSession(msg.session.ID),
	)
}
