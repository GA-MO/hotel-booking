-- Notifications outbox.
-- Domain modules (booking, subscription, ...) enqueue rows here inside their
-- own transactions; the async worker drains the queue and dispatches via the
-- channel-specific sender (Resend, LINE, ...). FOR UPDATE SKIP LOCKED on the
-- claim path lets us scale horizontally without double-sending.

CREATE TABLE notifications (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  channel      VARCHAR(20) NOT NULL CHECK (channel IN ('email','line','sms')),
  template     VARCHAR(80) NOT NULL,
  recipient    VARCHAR(320) NOT NULL,
  payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
  related_type VARCHAR(40),
  related_id   UUID,

  status       VARCHAR(20) NOT NULL DEFAULT 'queued'
                 CHECK (status IN ('queued','sending','sent','failed','dead')),
  attempts     INT NOT NULL DEFAULT 0,
  last_error   TEXT,

  send_after   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  sent_at      TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial index keeps the queue scan cheap regardless of historical volume.
CREATE INDEX idx_notifications_queue
  ON notifications (send_after) WHERE status = 'queued';

CREATE INDEX idx_notifications_related
  ON notifications (related_type, related_id);

CREATE TRIGGER trg_notifications_updated_at
  BEFORE UPDATE ON notifications
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
