package fork

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// newEnv builds a real session and message service over a temporary
// database, the same way the app wires them.
func newEnv(t *testing.T) (session.Service, message.Service, db.Querier) {
	t.Helper()
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	queries := db.New(conn)
	return session.NewService(queries, conn), message.NewService(queries), queries
}

func TestForkCopiesMessagesUpToTheForkPoint(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sessions, messages, queries := newEnv(t)

	source, err := sessions.Create(ctx, "source")
	require.NoError(t, err)

	first, err := messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "first"}},
	})
	require.NoError(t, err)
	second, err := messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "second"}},
	})
	require.NoError(t, err)
	_, err = messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "third"}},
	})
	require.NoError(t, err)

	svc := New(sessions, messages, queries, nil, t.TempDir())
	result, err := svc.Fork(ctx, source.ID, second.ID, ForkOptions{})
	require.NoError(t, err)
	require.NotEqual(t, source.ID, result.Session.ID)

	copied, err := messages.List(ctx, result.Session.ID)
	require.NoError(t, err)
	require.Len(t, copied, 2, "only the messages up to the fork point are copied")
	require.Equal(t, "first", copied[0].Content().Text)
	require.Equal(t, "second", copied[1].Content().Text)
	require.NotEqual(t, first.ID, copied[0].ID, "a copied message gets its own id")

	original, err := messages.List(ctx, source.ID)
	require.NoError(t, err)
	require.Len(t, original, 3, "the source session is untouched")
}

func TestForkRejectsAMessageFromAnotherSession(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sessions, messages, queries := newEnv(t)

	source, err := sessions.Create(ctx, "source")
	require.NoError(t, err)
	other, err := sessions.Create(ctx, "other")
	require.NoError(t, err)
	msg, err := messages.Create(ctx, other.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "elsewhere"}},
	})
	require.NoError(t, err)

	svc := New(sessions, messages, queries, nil, t.TempDir())
	_, err = svc.Fork(ctx, source.ID, msg.ID, ForkOptions{})
	require.Error(t, err)
}

func TestForkRejectsAnUnknownSession(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sessions, messages, queries := newEnv(t)

	svc := New(sessions, messages, queries, nil, t.TempDir())
	_, err := svc.Fork(ctx, "nope", "also-nope", ForkOptions{})
	require.Error(t, err)
}

func TestForkExcludesHiddenAndSteerMessages(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sessions, messages, queries := newEnv(t)

	source, err := sessions.Create(ctx, "source")
	require.NoError(t, err)
	_, err = messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "visible"}},
	})
	require.NoError(t, err)
	hidden, err := messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hidden", Hidden: true}},
	})
	require.NoError(t, err)
	steer, err := messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "steer", Steer: true}},
	})
	require.NoError(t, err)

	svc := New(sessions, messages, queries, nil, t.TempDir())
	result, err := svc.Fork(ctx, source.ID, steer.ID, ForkOptions{})
	require.NoError(t, err)

	copied, err := messages.List(ctx, result.Session.ID)
	require.NoError(t, err)
	require.Len(t, copied, 1, "hidden and steer messages are scaffolding, not conversation")
	require.Equal(t, "visible", copied[0].Content().Text)
	require.NotEqual(t, hidden.ID, copied[0].ID)
}

func TestForkRecordsTheOrigin(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sessions, messages, queries := newEnv(t)

	source, err := sessions.Create(ctx, "source")
	require.NoError(t, err)
	msg, err := messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "fork point"}},
	})
	require.NoError(t, err)

	svc := New(sessions, messages, queries, nil, t.TempDir())
	result, err := svc.Fork(ctx, source.ID, msg.ID, ForkOptions{Title: "forked run"})
	require.NoError(t, err)
	require.Equal(t, "forked run", result.Session.Title)
	require.Equal(t, msg.ID, result.Session.ForkedFromMessageID)
	require.Equal(t, source.ID, result.Session.ForkedFromSessionID)
}

func TestForkToleratesAMissingSnapshotService(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sessions, messages, queries := newEnv(t)

	source, err := sessions.Create(ctx, "source")
	require.NoError(t, err)
	msg, err := messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "no snapshot"}},
	})
	require.NoError(t, err)

	svc := New(sessions, messages, queries, nil, t.TempDir())
	result, err := svc.Fork(ctx, source.ID, msg.ID, ForkOptions{})
	require.NoError(t, err)
	require.Equal(t, msg.ID, result.Session.ForkedFromMessageID)
	require.Empty(t, result.Session.ForkedFromSnapshotID)
}

func TestForkDerivesATitleWhenNoneIsGiven(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sessions, messages, queries := newEnv(t)

	source, err := sessions.Create(ctx, "original title")
	require.NoError(t, err)
	msg, err := messages.Create(ctx, source.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hi"}},
	})
	require.NoError(t, err)

	svc := New(sessions, messages, queries, nil, t.TempDir())
	result, err := svc.Fork(ctx, source.ID, msg.ID, ForkOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Session.Title)
	require.NotEqual(t, "original title", result.Session.Title, "a fork gets its own title")
}
