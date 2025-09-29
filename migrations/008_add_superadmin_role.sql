-- +goose Up
-- Add a role column to the users table
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';

-- Set existing admins to the admin role
UPDATE users SET role = 'admin' WHERE is_admin = TRUE;

-- Remove the is_admin column
ALTER TABLE users DROP COLUMN is_admin;

-- +goose Down
-- Re-add the is_admin column
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- Set existing admins back to is_admin = TRUE
UPDATE users SET is_admin = TRUE WHERE role IN ('admin', 'super_admin');

-- Remove the role column
ALTER TABLE users DROP COLUMN role;
