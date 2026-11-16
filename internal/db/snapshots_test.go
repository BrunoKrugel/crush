package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotsCRUD(t *testing.T) {
	t.Cleanup(ResetPool)

	ctx := context.Background()
	conn, err := Connect(ctx, t.TempDir())
	require.NoError(t, err)

	q := New(conn)

	_, err = q.CreateSession(ctx, CreateSessionParams{
		ID:               "sess-1",
		Title:            "test",
		MessageCount:     0,
		PromptTokens:     0,
		CompletionTokens: 0,
		Cost:             0,
	})
	require.NoError(t, err)

	snap, err := q.CreateSnapshot(ctx, CreateSnapshotParams{
		ID:               "snap-1",
		SessionID:        "sess-1",
		MessageID:        "msg-1",
		ParentSnapshotID: sql.NullString{},
		GitCommitHash:    "abc123",
		Description:      sql.NullString{String: "first snapshot", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "snap-1", snap.ID)
	require.Equal(t, "abc123", snap.GitCommitHash)
	require.NotZero(t, snap.CreatedAt, "created_at should be set by the database")

	snap2, err := q.CreateSnapshot(ctx, CreateSnapshotParams{
		ID:               "snap-2",
		SessionID:        "sess-1",
		MessageID:        "msg-2",
		ParentSnapshotID: sql.NullString{String: "snap-1", Valid: true},
		GitCommitHash:    "def456",
		Description:      sql.NullString{},
	})
	require.NoError(t, err)
	require.Equal(t, "snap-2", snap2.ID)

	_, err = conn.ExecContext(ctx,
		`UPDATE snapshots SET created_at = created_at - 1 WHERE id = 'snap-1'`)
	require.NoError(t, err)

	got, err := q.GetSnapshot(ctx, "snap-1")
	require.NoError(t, err)
	require.Equal(t, "abc123", got.GitCommitHash)

	byMsg, err := q.GetSnapshotByMessageID(ctx, GetSnapshotByMessageIDParams{
		SessionID: "sess-1",
		MessageID: "msg-2",
	})
	require.NoError(t, err)
	require.Equal(t, "snap-2", byMsg.ID)

	list, err := q.ListSnapshotsBySession(ctx, "sess-1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "snap-2", list[0].ID, "latest first")

	latest, err := q.GetLatestSnapshotBySession(ctx, "sess-1")
	require.NoError(t, err)
	require.Equal(t, "snap-2", latest.ID)

	require.NoError(t, q.DeleteSnapshotsBySession(ctx, "sess-1"))

	list, err = q.ListSnapshotsBySession(ctx, "sess-1")
	require.NoError(t, err)
	require.Empty(t, list)
}
