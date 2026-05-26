-- Availability overrides + pricing rules.
-- The "computed" daily rate/availability per (hotel, room_type, date) is derived
-- on read by combining room_types defaults, sparse availability_overrides rows,
-- and pricing_rules in code (see internal/pricing/engine.go).

-- ------------------------------------------------------------
-- Daily availability + rate overrides per (hotel, room_type, date).
-- Sparse — only inserted when admin overrides defaults (close,
-- reduce inventory, set a different rate, restrict min-nights).
-- ------------------------------------------------------------
CREATE TABLE availability_overrides (
  hotel_id            UUID NOT NULL REFERENCES hotels(id)      ON DELETE CASCADE,
  room_type_id        UUID NOT NULL REFERENCES room_types(id)  ON DELETE CASCADE,
  date                DATE NOT NULL,

  inventory_change    INT  NOT NULL DEFAULT 0,   -- + or - vs total_inventory
  closed              BOOLEAN NOT NULL DEFAULT FALSE,

  rate_override       NUMERIC(10,2),             -- NULL = use rule-based pricing
  rate_currency       CHAR(3),                   -- NULL = use room_type.base_currency

  min_nights          INT,
  max_nights          INT,
  closed_to_arrival   BOOLEAN NOT NULL DEFAULT FALSE,
  closed_to_departure BOOLEAN NOT NULL DEFAULT FALSE,

  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  PRIMARY KEY (hotel_id, room_type_id, date)
);

CREATE INDEX idx_availability_overrides_date ON availability_overrides(hotel_id, date);

CREATE TRIGGER trg_availability_overrides_updated_at
  BEFORE UPDATE ON availability_overrides
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ------------------------------------------------------------
-- Rule-based pricing modifiers (season, weekend, LOS, advance purchase).
-- ------------------------------------------------------------
CREATE TABLE pricing_rules (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hotel_id        UUID NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  room_type_id    UUID REFERENCES room_types(id) ON DELETE CASCADE, -- NULL = applies to all room types

  name            VARCHAR(120) NOT NULL,
  rule_type       VARCHAR(30) NOT NULL
                    CHECK (rule_type IN ('season','day_of_week','length_of_stay','advance_purchase')),

  start_date      DATE,
  end_date        DATE,
  days_of_week    INT[],          -- 1..7, Mon..Sun (for day_of_week)

  modifier_type   VARCHAR(20) NOT NULL
                    CHECK (modifier_type IN ('percentage','fixed_amount','set_value')),
  modifier_value  NUMERIC(10,2) NOT NULL,

  min_nights      INT,
  max_nights      INT,
  min_days_ahead  INT,
  max_days_ahead  INT,

  priority        INT NOT NULL DEFAULT 100,
  enabled         BOOLEAN NOT NULL DEFAULT TRUE,

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CHECK (
    (rule_type = 'season'          AND start_date IS NOT NULL AND end_date IS NOT NULL)
    OR (rule_type = 'day_of_week'  AND days_of_week IS NOT NULL AND array_length(days_of_week, 1) > 0)
    OR (rule_type = 'length_of_stay'   AND min_nights IS NOT NULL)
    OR (rule_type = 'advance_purchase' AND min_days_ahead IS NOT NULL)
  )
);

CREATE INDEX idx_pricing_rules_hotel ON pricing_rules(hotel_id) WHERE enabled = TRUE;

CREATE TRIGGER trg_pricing_rules_updated_at
  BEFORE UPDATE ON pricing_rules
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
