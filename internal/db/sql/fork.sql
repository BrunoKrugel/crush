-- name: ForkMessage :exec
-- Copy one source message into the forked session: same role, parts,
-- timestamps, and provider metadata, so the forked session replays exactly
-- the history it inherited. The message gets a fresh id, because
-- messages.id is the table's primary key and the source keeps its copy.
INSERT INTO messages (
    id,
    session_id,
    role,
    parts,
    model,
    provider,
    is_summary_message,
    created_at,
    updated_at,
    finished_at,
    prism_model_id,
    prism_model_name,
    prism_hypercredit_savings,
    prism_dollar_savings
)
SELECT
    sqlc.arg(new_id),
    sqlc.arg(session_id),
    m.role,
    m.parts,
    m.model,
    m.provider,
    m.is_summary_message,
    m.created_at,
    m.updated_at,
    m.finished_at,
    m.prism_model_id,
    m.prism_model_name,
    m.prism_hypercredit_savings,
    m.prism_dollar_savings
FROM messages m
WHERE m.id = sqlc.arg(source_id);

-- name: SetSessionForkOrigin :exec
UPDATE sessions
SET
    forked_from_session_id = ?,
    forked_from_snapshot_id = ?,
    forked_from_message_id = ?
WHERE id = ?;

-- name: GetSessionMessagePosition :one
-- 1-based position of a message within its session's ordering, so a fork
-- can bound the copy to everything at or before the fork point.
SELECT COUNT(*) + 1 AS position
FROM messages m
WHERE m.session_id = ?
  AND m.created_at < (SELECT s.created_at FROM messages s WHERE s.id = ?);
