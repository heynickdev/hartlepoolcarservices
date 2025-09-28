-- +goose Up
ALTER TABLE users ADD COLUMN password_reset_token TEXT;
ALTER TABLE users ADD COLUMN password_reset_expires TIMESTAMPTZ;

-- +goose Down
ALTER TABLE users DROP COLUMN password_reset_token;
ALTER TABLE users DROP COLUMN password_reset_expires;