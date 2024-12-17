-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS rabbit_migrations (
     id SERIAL PRIMARY KEY,
     version_id BIGINT NOT NULL UNIQUE,
     description TEXT NOT NULL,
     is_applied BOOLEAN NOT NULL DEFAULT FALSE,
     applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
