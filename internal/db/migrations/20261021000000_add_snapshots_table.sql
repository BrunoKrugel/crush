-- +goose Up
-- +goose StatementBegin
-- Snapshots map a session message to the git commit that captured the working
-- tree at that point, forming a chain via parent_snapshot_id.
CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    parent_snapshot_id TEXT,
    git_commit_hash TEXT NOT NULL,
    description TEXT,
    created_at INTEGER NOT NULL,  -- Unix timestamp in seconds
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_snapshots_session_id ON snapshots (session_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_snapshots_session_id;
DROP TABLE IF EXISTS snapshots;
-- +goose StatementEnd
