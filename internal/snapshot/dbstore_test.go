package snapshot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/db"
)

func TestDBStore(t *testing.T) {
	t.Cleanup(db.ResetPool)

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	q := db.New(conn)

	_, err = q.CreateSession(context.Background(), db.CreateSessionParams{
		ID:    "sess-1",
		Title: "test",
	})
	require.NoError(t, err)

	store := NewDBStore(q)
	// The store path alone is under test; the repo is irrelevant here.

	require.NoError(t, store.CreateSnapshot(SnapshotInfo{
		SessionID:  "sess-1",
		MessageID:  "m1",
		CommitHash: "abc",
	}))
	require.NoError(t, store.CreateSnapshot(SnapshotInfo{
		SessionID:   "sess-1",
		MessageID:   "m2",
		CommitHash:  "def",
		Description: "second",
	}))

	// Make the ordering deterministic within the same clock second.
	_, err = conn.ExecContext(context.Background(),
		`UPDATE snapshots SET created_at = created_at - 1 WHERE message_id = 'm1'`)
	require.NoError(t, err)

	got, err := store.GetSnapshotByMessageID("sess-1", "m1")
	require.NoError(t, err)
	require.Equal(t, "abc", got.CommitHash)

	latest, err := store.GetLatestSnapshotBySession("sess-1")
	require.NoError(t, err)
	require.Equal(t, "m2", latest.MessageID)

	list, err := store.ListSnapshotsBySession("sess-1")
	require.NoError(t, err)
	require.Len(t, list, 2)
}
