-- +goose Up
-- +goose StatementBegin
-- Delivery give-up stamp (#388 follow-up): a terminal payload the
-- delivery retry chain abandoned after repeated failures must not be
-- re-delivered by the next startup's reconcile, or an out-of-credits
-- provider would loop again on every restart. The stamp survives in
-- the same durable record the reconcile reads.
ALTER TABLE a2a_dispatches ADD COLUMN abandoned_at INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE a2a_dispatches DROP COLUMN abandoned_at;
-- +goose StatementEnd
