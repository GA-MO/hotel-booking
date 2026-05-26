-- Foundation tables (Phase 0)
-- Booking, availability, pricing_rules, subscriptions, landing_pages, reviews
-- จะมาในไฟล์ migration ถัดไป

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- updated_at trigger helper
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ========================================================================
-- accounts: 1 owner = 1 account. Subscription state attaches to account.
-- ========================================================================
CREATE TABLE accounts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  billing_email   VARCHAR(255) NOT NULL,
  country         CHAR(2),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);

CREATE TRIGGER trg_accounts_updated_at
  BEFORE UPDATE ON accounts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ========================================================================
-- users: hotel staff. Roles: owner | manager | front_desk | read_only
-- ========================================================================
CREATE TABLE users (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  email           VARCHAR(255) NOT NULL,
  password_hash   TEXT NOT NULL,
  name            VARCHAR(255) NOT NULL,
  role            VARCHAR(32) NOT NULL DEFAULT 'owner'
                   CHECK (role IN ('owner', 'manager', 'front_desk', 'read_only')),
  locale          VARCHAR(10) NOT NULL DEFAULT 'en',
  email_verified_at TIMESTAMPTZ,
  last_login_at   TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ,
  UNIQUE (email)
);

CREATE INDEX idx_users_account_id ON users(account_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_users_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ========================================================================
-- hotels: 1 account → multiple hotels (multi-property = phase 3)
-- ========================================================================
CREATE TABLE hotels (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  slug            VARCHAR(80) NOT NULL,
  name            VARCHAR(255) NOT NULL,
  hotel_type      VARCHAR(50),
  description     TEXT,

  address_line    VARCHAR(500),
  city            VARCHAR(120),
  country         CHAR(2),
  postal_code     VARCHAR(20),
  latitude        NUMERIC(9,6),
  longitude       NUMERIC(9,6),

  phone           VARCHAR(50),
  email           VARCHAR(255),
  line_id         VARCHAR(120),

  timezone        VARCHAR(64) NOT NULL DEFAULT 'Asia/Bangkok',
  base_currency   CHAR(3) NOT NULL DEFAULT 'THB',

  check_in_time   TIME NOT NULL DEFAULT '14:00',
  check_out_time  TIME NOT NULL DEFAULT '12:00',

  policies        JSONB NOT NULL DEFAULT '{}'::jsonb,

  kyc_status      VARCHAR(20) NOT NULL DEFAULT 'pending'
                   CHECK (kyc_status IN ('pending', 'submitted', 'approved', 'rejected')),
  kyc_reviewed_at TIMESTAMPTZ,
  kyc_notes       TEXT,

  status          VARCHAR(20) NOT NULL DEFAULT 'test'
                   CHECK (status IN ('test', 'live', 'suspended', 'archived')),

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ,

  UNIQUE (slug),
  CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$')
);

CREATE INDEX idx_hotels_account_id ON hotels(account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_hotels_status     ON hotels(status)     WHERE deleted_at IS NULL;
CREATE TRIGGER trg_hotels_updated_at
  BEFORE UPDATE ON hotels
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ========================================================================
-- room_types: ห้องประเภทเดียวกัน — inventory ระดับนี้ ไม่ใช่ per room unit
-- ========================================================================
CREATE TABLE room_types (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hotel_id        UUID NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  name            VARCHAR(120) NOT NULL,
  description     TEXT,

  total_inventory INT NOT NULL CHECK (total_inventory >= 0),
  max_occupancy   INT NOT NULL CHECK (max_occupancy > 0),
  size_sqm        NUMERIC(6,2),
  bed_config      JSONB NOT NULL DEFAULT '{}'::jsonb,
  amenities       JSONB NOT NULL DEFAULT '[]'::jsonb,

  base_rate       NUMERIC(10,2) NOT NULL CHECK (base_rate >= 0),
  base_currency   CHAR(3) NOT NULL DEFAULT 'THB',

  display_order   INT NOT NULL DEFAULT 0,
  enabled         BOOLEAN NOT NULL DEFAULT TRUE,

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_room_types_hotel_id ON room_types(hotel_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_room_types_updated_at
  BEFORE UPDATE ON room_types
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
