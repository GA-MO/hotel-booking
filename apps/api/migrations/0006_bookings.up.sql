-- Bookings + event log.
-- Phase 1 schema. Money stored as BIGINT in minor units (e.g. satang for THB)
-- to avoid float precision issues — convert at the API boundary.

CREATE TABLE bookings (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  reference           VARCHAR(16) NOT NULL UNIQUE,

  hotel_id            UUID NOT NULL REFERENCES hotels(id) ON DELETE RESTRICT,
  room_type_id        UUID NOT NULL REFERENCES room_types(id) ON DELETE RESTRICT,
  room_count          INT NOT NULL CHECK (room_count > 0),

  -- guest snapshot
  guest_email         VARCHAR(255) NOT NULL,
  guest_phone         VARCHAR(50),
  guest_name          VARCHAR(255) NOT NULL,
  guest_country       CHAR(2),
  special_request     TEXT,

  -- stay
  check_in_date       DATE NOT NULL,
  check_out_date      DATE NOT NULL,
  nights              INT GENERATED ALWAYS AS (check_out_date - check_in_date) STORED,

  -- pricing snapshot (locked at booking creation)
  currency            CHAR(3) NOT NULL,
  room_subtotal_cents BIGINT NOT NULL CHECK (room_subtotal_cents >= 0),
  taxes_cents         BIGINT NOT NULL DEFAULT 0 CHECK (taxes_cents >= 0),
  fees_cents          BIGINT NOT NULL DEFAULT 0 CHECK (fees_cents >= 0),
  discounts_cents     BIGINT NOT NULL DEFAULT 0 CHECK (discounts_cents >= 0),
  total_cents         BIGINT NOT NULL CHECK (total_cents >= 0),

  -- lifecycle
  status              VARCHAR(30) NOT NULL DEFAULT 'pending_payment'
                       CHECK (status IN (
                         'pending_payment', 'confirmed', 'cancelled', 'expired',
                         'checked_in', 'checked_out', 'no_show', 'completed'
                       )),
  expires_at          TIMESTAMPTZ,                 -- for pending_payment
  cancelled_at        TIMESTAMPTZ,
  cancelled_by        VARCHAR(20),                  -- guest | hotel | system
  cancellation_reason TEXT,

  -- payment
  payment_method      VARCHAR(50),
  payment_status      VARCHAR(20) NOT NULL DEFAULT 'pending'
                       CHECK (payment_status IN ('pending','paid','refunded','partial','failed')),

  -- attribution
  source              VARCHAR(50) NOT NULL DEFAULT 'web'
                       CHECK (source IN ('web','walk_in','phone','admin')),
  utm_source          VARCHAR(120),
  utm_medium          VARCHAR(120),
  utm_campaign        VARCHAR(120),
  utm_term            VARCHAR(120),
  utm_content         VARCHAR(120),
  referrer            TEXT,

  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  confirmed_at        TIMESTAMPTZ,
  checked_in_at       TIMESTAMPTZ,
  checked_out_at      TIMESTAMPTZ,

  CONSTRAINT valid_dates CHECK (check_out_date > check_in_date)
);

CREATE INDEX idx_bookings_hotel_dates  ON bookings(hotel_id, check_in_date, check_out_date);
CREATE INDEX idx_bookings_room_dates   ON bookings(room_type_id, check_in_date, check_out_date)
  WHERE status IN ('pending_payment','confirmed','checked_in');
CREATE INDEX idx_bookings_expires      ON bookings(expires_at) WHERE status = 'pending_payment';
CREATE INDEX idx_bookings_reference    ON bookings(reference);

CREATE TRIGGER trg_bookings_updated_at
  BEFORE UPDATE ON bookings
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Append-only audit log of every state change / important event.
CREATE TABLE booking_events (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id  UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  event_type  VARCHAR(60) NOT NULL,
  actor_type  VARCHAR(20),                 -- guest | hotel_staff | system | platform_admin
  actor_id    UUID,                        -- user_id when actor_type = hotel_staff
  payload     JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_booking_events_booking ON booking_events(booking_id, created_at);
