package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// forkTestWorkspace records the fork calls the UI makes.
type forkTestWorkspace struct {
	channelsTestWorkspace
	messages   []message.Message
	forked     bool
	forkSource string
	forkMsgID  string
	forkTitle  string
	forkTree   bool
	forkErr    error
}

func (w *forkTestWorkspace) ListUserMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	return w.messages, nil
}

func (w *forkTestWorkspace) ForkSession(ctx context.Context, sourceSessionID, messageID, title string, worktree bool) (session.Session, error) {
	w.forked = true
	w.forkSource = sourceSessionID
	w.forkMsgID = messageID
	w.forkTitle = title
	w.forkTree = worktree
	if w.forkErr != nil {
		return session.Session{}, w.forkErr
	}
	return session.Session{ID: "forked-session"}, nil
}

// TestLastUserMessageIDPicksTheLastVisiblePrompt pins the fork point:
// the last user message that is not hidden scaffolding.
func TestLastUserMessageIDPicksTheLastVisiblePrompt(t *testing.T) {
	t.Parallel()

	ws := &forkTestWorkspace{messages: []message.Message{
		{ID: "m1", Role: message.User},
		{ID: "m2", Role: message.Assistant},
		{ID: "m3", Role: message.User},
	}}
	m := &UI{com: &common.Common{Workspace: ws}}

	id, err := m.lastUserMessageID("session-1")
	require.NoError(t, err)
	require.Equal(t, "m3", id)
}

// TestLastUserMessageIDSkipsHiddenPrompts checks that a hidden
// continuation is not offered as a fork point.
func TestLastUserMessageIDSkipsHiddenPrompts(t *testing.T) {
	t.Parallel()

	ws := &forkTestWorkspace{messages: []message.Message{
		{ID: "m1", Role: message.User},
		{ID: "m2", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hidden", Hidden: true}}},
	}}
	m := &UI{com: &common.Common{Workspace: ws}}

	id, err := m.lastUserMessageID("session-1")
	require.NoError(t, err)
	require.Equal(t, "m1", id)
}

// TestForkSessionCallsTheWorkspace checks that the fork request carries
// the session, the fork point, the title, and the worktree choice.
func TestForkSessionCallsTheWorkspace(t *testing.T) {
	t.Parallel()

	ws := &forkTestWorkspace{}
	m := &UI{com: &common.Common{Workspace: ws}}

	msg := m.forkSession("session-1", "message-7", "new name", true)()
	forked, ok := msg.(forkSessionMsg)
	require.True(t, ok, "the fork reports its outcome as a message")
	require.NoError(t, forked.err)
	require.True(t, ws.forked)
	require.Equal(t, "session-1", ws.forkSource)
	require.Equal(t, "message-7", ws.forkMsgID)
	require.Equal(t, "new name", ws.forkTitle)
	require.True(t, ws.forkTree)
}
