-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS token_blacklist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(100) NOT NULL, -- 'admin_change', 'user_deleted', 'logout', etc.
    blacklisted_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_token_blacklist_hash ON token_blacklist(token_hash);
CREATE INDEX idx_token_blacklist_expires ON token_blacklist(expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS token_blacklist;
-- +goose StatementEnd