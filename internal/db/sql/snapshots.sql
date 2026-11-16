-- name: CreateSnapshot :one
INSERT INTO snapshots (
    id, session_id, message_id, parent_snapshot_id, git_commit_hash, description, created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, strftime('%s', 'now')
) RETURNING *;

-- name: GetSnapshot :one
SELECT * FROM snapshots WHERE id = ?;

-- name: GetSnapshotByMessageID :one
SELECT * FROM snapshots WHERE session_id = ? AND message_id = ?;

-- name: ListSnapshotsBySession :many
SELECT * FROM snapshots WHERE session_id = ? ORDER BY created_at DESC;

-- name: GetLatestSnapshotBySession :one
SELECT * FROM snapshots WHERE session_id = ? ORDER BY created_at DESC LIMIT 1;

-- name: DeleteSnapshotsBySession :exec
DELETE FROM snapshots WHERE session_id = ?;
