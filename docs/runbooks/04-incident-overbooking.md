# 04 — Resolve a double-booking incident

**When to use:** a guest (or hotel) reports that two bookings exist for the same room on overlapping dates.
**Time estimate:** 15–45 minutes depending on root cause + how many guests are affected.
**Risk level:** High customer impact — handle with urgency.

> Our booking-creation path uses `SELECT ... FOR UPDATE` on the `room_types` row inside a transaction that also counts active bookings (see [ADR-0007](../decisions/0007-select-for-update-booking-race.md)). Genuine oversells from this code path are nearly impossible — investigate other causes first.

## 1. Confirm the report

Get from the reporter:

- Hotel slug or ID
- Room type / room category
- Affected check-in date
- Booking references (if known) or guest emails

## 2. Reproduce the state in the DB

```sql
-- All overlapping bookings for the suspect (hotel, room_type, date).
SELECT id, reference, status, room_count, check_in_date, check_out_date,
       created_at, source, guest_email
FROM bookings
WHERE hotel_id = $hotel_id
  AND room_type_id = $room_type_id
  AND status IN ('pending_payment','confirmed','checked_in')
  AND check_in_date  < $check_out_date::date
  AND check_out_date > $check_in_date::date
ORDER BY created_at;

-- Inventory at the time.
SELECT total_inventory FROM room_types WHERE id = $room_type_id;

-- Any override that reduces inventory for this date?
SELECT date, inventory_change, closed
FROM availability_overrides
WHERE room_type_id = $room_type_id
  AND date BETWEEN $check_in_date AND ($check_out_date::date - INTERVAL '1 day');
```

## 3. Classify the cause

| Pattern | Diagnosis | Action |
|---|---|---|
| `SUM(room_count)` of overlapping rows ≤ `total_inventory + inventory_change` | Not actually oversold — reporter is confused. Walk through with them. | Reply with proof; no DB change. |
| Two `pending_payment` rows from the same minute, both with `room_count` exceeding capacity | Concurrent booking attempts before the FOR UPDATE landed. **Bug in CreatePending.** | See §4 — emergency fix. |
| Hotel admin inserted a walk-in booking via the API that ignored the capacity check | Walk-in endpoint relies on the same `CreatePending`. If oversell occurred here, same bug as above. | See §4. |
| Hotel staff manually edited the bookings table | Don't do that. Page the team. | Audit-log review + comms with hotel. |
| Inventory was REDUCED *after* bookings were taken (e.g. room offline for maintenance) | Existing bookings are valid — no oversell at booking time; hotel needs to relocate guests. | §5. |

## 4. Emergency mitigation if a real oversell is confirmed

```bash
# Stop accepting new bookings for the affected room type until we can patch.
psql ... -c "
UPDATE room_types SET enabled = FALSE
WHERE id = '<room_type_id>';"

# (Optional) Mark the second booking as cancelled to free inventory.
# Use this only after coordinating with the hotel which booking they will honour.
psql ... -c "
UPDATE bookings
SET status = 'cancelled',
    cancelled_at = NOW(),
    cancelled_by = 'platform_admin',
    cancellation_reason = 'Inventory oversell — resolved manually, see runbook 04'
WHERE id = '<booking_id_to_drop>'
  AND status IN ('pending_payment','confirmed');"
```

Then:

- File a P0 bug. The relevant test is `booking.TestRepo_CreatePending_RaceNoOversell` — extend it to reproduce the failure mode you observed.
- Page the maintainer.

## 5. Inventory-reduction case

If bookings are valid but inventory shrank after the fact:

- Talk to the hotel about which guest they'll relocate or upgrade.
- Cancel the relocated guest's booking with `cancelled_by = 'hotel'` and `cancellation_reason` capturing the agreement.
- Offer compensation (refund + future credit). Out of scope for the platform — the hotel handles this.
- If the system caused the inventory misconfiguration (e.g. admin UI bug), file it.

## 6. Communication template

```
Subject: Update on your booking <reference>

Dear <guest_name>,

We're writing to let you know about an issue with your booking at <hotel_name> for <dates>. <Specific issue and resolution>.

We've issued a full refund of <amount> to your original payment method; please allow up to 7 business days for it to appear. The hotel has also offered <compensation>.

We're sorry for the inconvenience. Please reply to this email if you have any questions.

— <Hotel> via [Platform]
```

## 7. After-action

- [ ] Incident note in `docs/runbooks/incidents/`.
- [ ] Regression test added if the cause was code.
- [ ] If hotel-process related, update the hotel onboarding docs (Phase 2 admin-web) to surface the relevant rule.
