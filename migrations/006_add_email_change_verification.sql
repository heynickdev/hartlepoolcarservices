-- +goose Up
ALTER TABLE users ADD COLUMN pending_email TEXT;
ALTER TABLE users ADD COLUMN email_change_token TEXT;
ALTER TABLE users ADD COLUMN email_change_expires TIMESTAMPTZ;

-- +goose Down
ALTER TABLE users DROP COLUMN pending_email;
ALTER TABLE users DROP COLUMN email_change_token;
ALTER TABLE users DROP COLUMN email_change_expires;