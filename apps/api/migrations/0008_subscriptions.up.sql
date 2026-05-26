-- Subscriptions + state-transition audit log (Phase 1: state tracking only).
-- One subscription per account (UNIQUE account_id).
-- No Stripe/Omise calls yet — payment_method_id is just an opaque token stored
-- after the gateway client SDK tokenises the card. Real charging logic lives
-- in Phase 2.

CREATE TABLE subscriptions (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id            UUID NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,

  status                VARCHAR(30) NOT NULL DEFAULT 'pending_kyc'
                          CHECK (status IN (
                            'pending_kyc','trialing','trial_ending','trial_lapsed',
                            'active','past_due','suspended','cancelled','terminated'
                          )),
  plan_code             VARCHAR(50),
  billing_cycle         VARCHAR(20) CHECK (billing_cycle IN ('monthly','annual')),

  trial_started_at      TIMESTAMPTZ,
  trial_ends_at         TIMESTAMPTZ,

  current_period_start  TIMESTAMPTZ,
  current_period_end    TIMESTAMPTZ,

  -- Snapshot of pricing inputs (per-room pricing TBD — kept nullable for now).
  room_count_snapshot   INT,
  unit_price_cents      BIGINT,
  currency              CHAR(3),

  payment_provider      VARCHAR(20) CHECK (payment_provider IN ('stripe','omise')),
  payment_method_id     VARCHAR(120),
  payment_method_last4  VARCHAR(4),
  payment_method_brand  VARCHAR(20),

  cancelled_at          TIMESTAMPTZ,
  cancellation_reason   TEXT,
  suspended_at          TIMESTAMPTZ,

  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_trial_end ON subscriptions(trial_ends_at)
  WHERE status IN ('trialing','trial_ending');

CREATE TRIGGER trg_subscriptions_updated_at
  BEFORE UPDATE ON subscriptions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Append-only audit trail of state transitions.
-- Also used by the dunning worker for "remind 30/14/7 days before trial ends".
CREATE TABLE subscription_events (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
  event_type      VARCHAR(60) NOT NULL,
  actor_type      VARCHAR(20),   -- system | platform_admin | hotel_staff
  actor_id        UUID,
  payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscription_events_sub ON subscription_events(subscription_id, created_at);

-- Phase-2 placeholders (invoices, payment_attempts) intentionally not in
-- schema yet — added when we wire up real Stripe/Omise charging.
