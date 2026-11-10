-- +goose Up
-- +goose StatementBegin
-- Conversation forking: a session may carry the session, snapshot, and
-- message it was forked from, so the UI can show the lineage and the
-- restore tooling can resolve the fork point.
ALTER TABLE sessions ADD COLUMN forked_from_session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL;
ALTER TABLE sessions ADD COLUMN forked_from_snapshot_id TEXT REFERENCES snapshots (id) ON DELETE SET NULL;
ALTER TABLE sessions ADD COLUMN forked_from_message_id TEXT REFERENCES messages (id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN forked_from_message_id;
ALTER TABLE sessions DROP COLUMN forked_from_snapshot_id;
ALTER TABLE sessions DROP COLUMN forked_from_session_id;
-- +goose StatementEnd
