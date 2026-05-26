# Glossary

Domain and technical terms used in the hotel-booking codebase. Cross-references point at the file or table where the term is materialised.

## Hospitality domain

| Term | TH | Definition | In code |
|---|---|---|---|
| **ADR** | อัตราเฉลี่ยต่อห้องคืน | Average Daily Rate — total room revenue ÷ rooms sold. KPI, not stored. | reports (Phase 2) |
| **AP** | จองล่วงหน้า | Advance Purchase rule — discount when guest books ≥N days ahead. | `pricing_rules.rule_type='advance_purchase'` |
| **ARI** | — | Availability / Rate / Inventory — the three data streams a channel manager syncs across OTAs. | Phase 3 |
| **BAR** | ราคาฐาน | Best Available Rate — the unrestricted publicly offered nightly rate. | `room_types.base_rate` |
| **Booking** | การจอง | A reservation record. | `bookings` table, `internal/booking/` |
| **Channel Manager** | — | Software that syncs availability + rates across multiple OTAs (Booking.com, Agoda, etc.). | Phase 3, ADR-not-yet |
| **Check-in / Check-out** | เช็คอิน / เช็คเอาท์ | Times stored as `TIME` on hotels (defaults 14:00 / 12:00). The dates are `DATE` on bookings — check-out is exclusive (night-count = check_out − check_in). | `hotels.check_in_time`, `bookings.check_in_date` |
| **CTA** | ปิดวันเข้าพัก | Closed To Arrival — guests may not check in on that date. | `availability_overrides.closed_to_arrival` |
| **CTD** | ปิดวันเช็คเอาท์ | Closed To Departure — guests may not check out on that date. | `availability_overrides.closed_to_departure` |
| **DoW** | วันในสัปดาห์ | Day-of-week pricing rule — typically weekend uplift (Fri/Sat). | `pricing_rules.rule_type='day_of_week'` |
| **Guest** | ผู้เข้าพัก / แขก | A person booking a room. Stored snapshot on each booking — no separate guests table Phase 1. | `bookings.guest_*` |
| **Hotel** | โรงแรม / ที่พัก | Includes boutique hotels, B&Bs, hostels, guesthouses, serviced apartments. | `hotels` table |
| **KYC** | ยืนยันตัวตน | Know Your Customer — proof-of-business uploads we require before a hotel goes live. | `hotels.kyc_status` |
| **LOS** | จำนวนคืน | Length of Stay — number of nights. | `bookings.nights` (generated col) |
| **MLOS** | จองขั้นต่ำ | Minimum LOS — a rule that requires at least N nights to book. | `pricing_rules.min_nights`, `availability_overrides.min_nights` |
| **No-show** | ไม่มาเข้าพัก | Guest didn't arrive on check-in date. | `bookings.status='no_show'` |
| **OTA** | ตัวแทนจองออนไลน์ | Online Travel Agency — Booking.com, Agoda, Expedia, Trip.com. **We are not an OTA.** | competitor reference |
| **PMS** | ระบบจัดการโรงแรม | Property Management System — the hotel's day-to-day operating tool. Our admin app IS a lightweight PMS. | `apps/admin-web/` |
| **PromptPay** | พร้อมเพย์ | Thai national instant payment system (QR-based). Guests typically pay hotels via PromptPay, not us. | future payment integration |
| **Rate plan** | แพ็กเกจราคา | Pricing variant (e.g. Standard, Non-refundable). Phase 1 = one implicit plan; Phase 2 will introduce multiple. | not yet modeled |
| **Room type** | ประเภทห้อง | E.g. "Deluxe Double" — a category with shared rate and amenities. Inventory is tracked at this level (`total_inventory`), not per physical room. | `room_types`, `internal/roomtype/` |
| **Stop sell** | หยุดขาย | Make a date unbookable. Implemented as `availability_overrides.closed = TRUE`. | `internal/pricing/` |
| **Walk-in** | ลูกค้าเดินเข้ามาเอง | Booking taken in-person at the front desk, recorded in the system manually. | `bookings.source='walk_in'` |

## Pricing terms

| Term | Definition | In code |
|---|---|---|
| **Modifier types** | How a pricing rule changes a rate: `percentage`, `fixed_amount`, `set_value`. | `pricing_rules.modifier_type` |
| **Priority** | Order in which multiple matching rules of the same type apply (ascending). | `pricing_rules.priority` |
| **Quote** | A price preview without inventory reservation. | `POST /v1/public/quote/{slug}` |
| **Snapshot** | The booking row stores the breakdown computed at creation time; later rule edits don't retroactively change it. | `bookings.room_subtotal_cents`, `total_cents`, etc. |

## Subscription / billing terms

| Term | Definition | In code |
|---|---|---|
| **Trial** | Free 365-day window starting on the hotel's first confirmed booking (capped at +60d post-signup). | `subscriptions.trial_started_at`, `trial_ends_at` |
| **Trial ending** | Final 30-day window before trial expiry — system flips status here so reminders + payment-method captures can be triggered. | `subscriptions.status='trial_ending'` |
| **Trial lapsed** | Trial expired without a payment method on file. 14-day grace before suspension. | `subscriptions.status='trial_lapsed'` |
| **Past due** | Active subscription with a failed charge. Retried before suspension. | `subscriptions.status='past_due'` |
| **Suspended** | Read-only; can't accept new bookings. Reactivatable. | `subscriptions.status='suspended'` |
| **Terminated** | Data retention window expired; account is fully closed. | `subscriptions.status='terminated'` |

## Technical / architectural

| Term | Definition | Reference |
|---|---|---|
| **ADR** | Architecture Decision Record — short doc capturing a single decision. | `docs/decisions/` |
| **CTE** | Common Table Expression (`WITH ... AS`). Used in `notification.ClaimBatch` to alias `id` and avoid ambiguous-column errors. | `internal/notification/repository.go` |
| **FOR UPDATE** | Postgres row-level lock — held until transaction commit. Used to serialize booking creation per `room_type`. | ADR-0007 |
| **FOR UPDATE SKIP LOCKED** | Lock if available, otherwise skip the row. Used in the notification worker so multiple workers don't claim the same row. | ADR-0008 |
| **Idempotency key** | Client-supplied UUID on `POST /v1/bookings` so retries are safe. | `X-Idempotency-Key` header (Phase 2) |
| **ISR** | Incremental Static Regeneration — Next.js feature that serves a static page from CDN and rebuilds it after N seconds. Used for landing pages. | ADR-not-yet |
| **JSONB** | Postgres binary JSON. Used for landing page `branding`, `sections`, `seo`, `tracking` plus the booking_events `payload`. | various |
| **Minor units** | Smallest currency denomination — satang for THB (1 THB = 100 satang). All money in code is stored as `int64` satang to avoid float precision. | ADR-0006 |
| **Outbox pattern** | Domain code writes a `notifications` row in the same transaction as the state change; a worker drains it. Decouples send latency from request latency. | ADR-0008 |
| **PHC string** | Password Hashing Competition's string format: `$argon2id$v=19$m=...$<salt>$<hash>`. | `internal/auth/password.go` |
| **Soft delete** | `deleted_at TIMESTAMPTZ` column on accounts/users/hotels/room_types; queries filter `WHERE deleted_at IS NULL`. Hard delete is used for bookings, events, notifications. | various |
| **Tenant** | An account (1 owner, 1 billing relationship, ≥1 hotels). Everything is account-scoped. | `accounts` table |

## Thai-specific

| Term | Definition |
|---|---|
| **PDPA** | Personal Data Protection Act B.E. 2562 (2019) — Thailand's GDPR-equivalent. |
| **VAT 7%** | Standard Thai value-added tax applied to subscription invoices. |
| **WHT 3%** | Withholding Tax — some B2B customers will deduct 3% from the subscription invoice; we issue a WHT certificate. |
| **e-Tax invoice** | RD-approved electronic tax invoice. Phase 2 via Leceipt or similar ETDA-approved provider. |
| **LINE OA** | LINE Official Account — Thai-market messaging channel. Phase 2 integration via LINE Messaging API. |
