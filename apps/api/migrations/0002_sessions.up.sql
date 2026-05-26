-- Refresh-token sessions.
-- Access tokens are stateless JWTs (short-lived). Refresh tokens are opaque random
-- strings whose SHA-256 hash is stored here; rotation on every refresh allows
-- detection of stolen-and-reused tokens.

CREATE TABLE sessions (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_token_hash  CHAR(64) NOT NULL,
  user_agent          TEXT,
  ip_address          INET,
  expires_at          TIMESTAMPTZ NOT NULL,
  revoked_at          TIMESTAMPTZ,
  rotated_to          UUID REFERENCES sessions(id) ON DELETE SET NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_used_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (refresh_token_hash)
);

CREATE INDEX idx_sessions_user_id     ON sessions(user_id)    WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_expires_at  ON sessions(expires_at) WHERE revoked_at IS NULL;
