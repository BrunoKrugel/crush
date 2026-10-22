-- +goose Up
-- +goose StatementBegin
-- Goal mode: a session may carry the goal it is working toward, the
-- turn budget it may consume, and the state of that budget, so the
-- ladder can read the goal without asking anybody.
ALTER TABLE sessions ADD COLUMN goal TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN goal;
-- +goose StatementEnd
