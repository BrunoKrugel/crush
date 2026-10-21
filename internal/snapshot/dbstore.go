package snapshot

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/charmbracelet/crush/internal/db"
)

// DBStore persists snapshot metadata in the snapshots table.
type DBStore struct {
	q *db.Queries
}

// NewDBStore returns a Store backed by the snapshots table.
func NewDBStore(q *db.Queries) *DBStore {
	return &DBStore{q: q}
}

// CreateSnapshot records a snapshot, chaining it to the session's current
// latest snapshot when one exists.
func (s *DBStore) CreateSnapshot(info SnapshotInfo) error {
	ctx := context.Background()
	var parentID string
	if latest, err := s.q.GetLatestSnapshotBySession(ctx, info.SessionID); err == nil {
		parentID = latest.ID
	}
	_, err := s.q.CreateSnapshot(ctx, db.CreateSnapshotParams{
		ID:               uuid.NewString(),
		SessionID:        info.SessionID,
		MessageID:        info.MessageID,
		ParentSnapshotID: nullString(parentID),
		GitCommitHash:    info.CommitHash,
		Description:      nullString(info.Description),
	})
	return err
}

// GetSnapshotByMessageID finds a snapshot by session and message.
func (s *DBStore) GetSnapshotByMessageID(sessionID, messageID string) (SnapshotInfo, error) {
	row, err := s.q.GetSnapshotByMessageID(context.Background(), db.GetSnapshotByMessageIDParams{
		SessionID: sessionID,
		MessageID: messageID,
	})
	if err != nil {
		return SnapshotInfo{}, err
	}
	return snapshotInfoFromRow(row), nil
}

// GetLatestSnapshotBySession returns the newest snapshot of a session.
func (s *DBStore) GetLatestSnapshotBySession(sessionID string) (SnapshotInfo, error) {
	row, err := s.q.GetLatestSnapshotBySession(context.Background(), sessionID)
	if err != nil {
		return SnapshotInfo{}, err
	}
	return snapshotInfoFromRow(row), nil
}

// ListSnapshotsBySession returns the snapshots newest first.
func (s *DBStore) ListSnapshotsBySession(sessionID string) ([]SnapshotInfo, error) {
	rows, err := s.q.ListSnapshotsBySession(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]SnapshotInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, snapshotInfoFromRow(row))
	}
	return out, nil
}

func snapshotInfoFromRow(row db.Snapshot) SnapshotInfo {
	info := SnapshotInfo{
		SessionID:  row.SessionID,
		MessageID:  row.MessageID,
		CommitHash: row.GitCommitHash,
		CreatedAt:  time.Unix(row.CreatedAt, 0),
	}
	if row.Description.Valid {
		info.Description = row.Description.String
	}
	return info
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
