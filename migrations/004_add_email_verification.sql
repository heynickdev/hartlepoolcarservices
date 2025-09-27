-- +goose Up
-- Add email verification fields to users table
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN email_verification_token TEXT;
ALTER TABLE users ADD COLUMN email_verification_expires TIMESTAMPTZ;

-- Set existing users as email verified (grandfather them in)
UPDATE users SET email_verified = TRUE WHERE email_verified = FALSE;

-- +goose Down
-- Remove email verification fields from users table
ALTER TABLE users DROP COLUMN email_verification_expires;
ALTER TABLE users DROP COLUMN email_verification_token;
ALTER TABLE users DROP COLUMN email_verified;