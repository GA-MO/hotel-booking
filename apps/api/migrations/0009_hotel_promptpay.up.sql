-- PromptPay ID on hotels.
-- Used by booking-web to render a real EMVCo PromptPay QR on the confirmation
-- page. Nullable because not every hotel will configure it (foreign hotels,
-- non-TH currencies, hotels that prefer cards-only).
--
-- Format validation (10-digit phone / 13-digit national ID / 15-char tax ID)
-- is enforced in the service layer, not via a CHECK constraint — the rules
-- are normalised (strip dashes/spaces) before validation and may evolve.
ALTER TABLE hotels
  ADD COLUMN promptpay_id VARCHAR(50);
