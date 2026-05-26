# Hotel Booking Platform — Master Plan

> Direct Booking Engine + Lightweight PMS สำหรับโรงแรมเล็ก
> Subscription model, ฟรีปีแรก, ตลาดไทย+ต่างชาติ

---

## 1. Vision & Positioning

### ระบบคืออะไร
**Direct booking engine + PMS** สำหรับโรงแรมเล็ก (boutique, B&B, hostel, guesthouse) ไม่ใช่ marketplace

- โรงแรมแต่ละแห่งได้ landing + booking page ของตัวเอง ที่ `book.[platform].com/[hotel-slug]`
- Traffic มาจาก ads/social ของโรงแรมเอง → click ตรงเข้าจอง
- **ไม่มีหน้าค้นหา/discovery**
- โรงแรมเป็นเจ้าของลูกค้าและเงิน

### Business Model
- **Subscription** (ไม่ใช่ commission)
- ฟรี 365 วันแรกเพื่อ acquisition
- หลัง trial: per-room pricing (Starter ~฿59/ห้อง/เดือน, Pro ~฿99, Business ~฿149)
- เงินจาก guest → ตรงโรงแรม (เราไม่ใช่ payment facilitator)

### Value Prop
"เว็บจองของคุณเอง + ระบบจัดการ — จ่ายรายเดือนคงที่ ไม่ต้องเสีย 15-25% commission ให้ OTA"

### Competitors
Cloudbeds, Little Hotelier, Sirvoy, SiteMinder Get Direct, HotelRunner
**ไม่ใช่** Agoda/Booking.com (เราไม่แข่ง discovery)

---

## 2. Tech Stack Decisions

| Layer | Choice | หมายเหตุ |
|---|---|---|
| Backend | **Go** | performance ดี, ต้องระวัง library coverage สำหรับ booking/payment ที่มัก JS/Python-first |
| Frontend | **Next.js App Router** | SSG/ISR เร็วบน landing page + SEO, ecosystem ใหญ่สุด |
| Database | **PostgreSQL** (self-host) | JSONB สำหรับ flexible content, FOR UPDATE สำหรับ concurrency |
| Cache | **Redis** (self-host) | rate cache, session, exchange rates, idempotency keys |
| Search | (ไม่ต้องระยะแรก) | ไม่มี discovery → ไม่ต้อง Elasticsearch/Meilisearch |
| Image processing | **imgproxy** (self-host) | on-the-fly resize, fits self-host stack |
| Object storage | **MinIO หรือ Backblaze B2** | user uploads (รูป) |
| Email | **Resend** | developer-friendly, ราคา OK |
| Notification | **LINE Messaging API** | สำคัญสำหรับตลาดไทย |
| Subscription billing | **Stripe + Omise** | สำหรับ subscription ของเรา (ไม่ใช่ guest payment) |
| Reverse proxy | **Caddy** | auto Let's Encrypt, ง่ายกว่า Nginx |
| Orchestration | **Docker Compose** | k8s/k3s overkill สำหรับทีม 2-3 dev |
| CI/CD | **GitHub Actions + Coolify** (optional) | Coolify ให้ Vercel-like experience บน VPS |
| AI features | **API-based** (Claude/OpenAI/DeepL/iApp) | no model training, Phase 2 onwards |

### 2.1 Hosting: **Self-host บน Hetzner Cloud**
- 2x CPX21 (4vCPU, 8GB) Phase 1, scale up เมื่อโตขึ้น
- Cloudflare ครอบ (DNS, CDN, WAF, DDoS, ฟรี tier)
- Daily Postgres backup → Backblaze B2
- Server location: Helsinki/Germany (latency ~250ms มาไทย — mitigated ด้วย Cloudflare edge cache)
- Alternative ถ้าต้องการ latency ไทย: DigitalOcean/Vultr Singapore (2-3x ราคา)
- Estimated cost Phase 1: **~€30-40/mo (~฿1,200-1,600)** ก่อนรายได้

### 2.2 Self-host Considerations
- 20-30% ของ dev time ช่วงแรกจะหมดไปกับ ops setup
- ต้องมี: uptime monitoring (UptimeRobot/BetterStack), disk alerts, unattended security upgrades
- Secret management: sops + age, หรือ env files + restricted perms Phase 1
- Test restore backup เดือนละครั้งขั้นต่ำ

---

## 3. Architecture Overview

```
[Hotel's Ads/Social] ──► [book.platform.com/{slug}]
                              │
                              ▼
              ┌───────────────────────────────┐
              │   Cloudflare (DNS+CDN+WAF)    │  ฟรี tier
              └───────────────┬───────────────┘
                              │
                              ▼
                  ┌───────────────────────┐
                  │  Caddy (auto-SSL)     │  reverse proxy
                  └───────────┬───────────┘
                              │
                  ┌───────────┴────────────┐
                  ▼                        ▼
       ┌──────────────────┐    ┌──────────────────┐
       │  Next.js         │    │  Go API          │
       │  (ISR + edge     │    │  (modular        │
       │   cached via CF) │    │   monolith)      │
       └──────────────────┘    └──────────┬───────┘
                                          │
                          ┌───────────────┼───────────────┐
                          ▼               ▼               ▼
                    ┌──────────┐    ┌──────────┐   ┌───────────┐
                    │ Postgres │    │  Redis   │   │ imgproxy  │
                    │ (self-h) │    │ (self-h) │   │ + MinIO   │
                    └──────────┘    └──────────┘   └───────────┘
                          │
                          ▼ nightly
                  ┌──────────────┐
                  │ Backblaze B2 │ off-site backup
                  └──────────────┘

      ┌──────────────────────────────────────────┐
      │  Async workers (Go) — same fleet         │
      │  - cleanup expired holds                 │
      │  - send notifications (email/LINE)       │
      │  - generate invoices                     │
      │  - reconcile Stripe/Omise webhooks       │
      │  - schedule subscription dunning emails  │
      └──────────────────────────────────────────┘
      
      Hetzner Cloud (Helsinki) — 2x CPX21 Phase 1
      All services in docker-compose
```

**Service breakdown (logical, in single monolith Phase 1):**
- Auth & Account
- Hotel & RoomType
- Landing page content
- Availability & Pricing
- Booking
- Payment (subscription only — guest payment ไหลตรงโรงแรม)
- Notification
- Subscription billing
- Reviews

---

## 4. User Roles

### 4.1 Guest
- จองห้องผ่าน hotel landing page
- จัดการ booking ของตัวเอง (modify, cancel)
- เขียนรีวิวหลังพัก

### 4.2 Hotel Staff (multi-level)
| Role | Permissions |
|---|---|
| **Owner** | Full access + billing + delete |
| **Manager** | Manage rooms, pricing, bookings, staff |
| **Front Desk** | Check-in/out, walk-in booking |
| **Read-only** | View only |

### 4.3 Platform Admin
- KYC verification
- Subscription management
- Dispute handling
- Slug conflict resolution

---

## 5. Core Domains & Data Model

### 5.1 Account & Hotels
```sql
accounts          -- 1 owner = 1 account = 1 subscription
  id, owner_user_id, billing_email, country, created_at

users             -- staff users
  id, account_id, email, password_hash, name, role, locale

hotels            -- 1 account can have multiple (Phase 3+)
  id, account_id, slug (unique), name, type, address, lat, lng,
  timezone, base_currency, kyc_status (pending|approved|rejected),
  status (test|live|suspended), policies (jsonb), created_at
```

### 5.2 Room Types & Inventory
```sql
room_types
  id, hotel_id, name, description, max_occupancy, bed_config (jsonb),
  total_inventory, base_rate, base_currency, amenities (jsonb)

availability_overrides       -- เก็บเฉพาะ override per (room_type, date)
  hotel_id, room_type_id, date,
  inventory_change, closed,
  rate_override,
  min_nights, max_nights,
  closed_to_arrival, closed_to_departure
  PRIMARY KEY (hotel_id, room_type_id, date)

pricing_rules               -- season, weekend, LOS, advance purchase
  id, hotel_id, room_type_id (nullable), name, rule_type,
  start_date, end_date, days_of_week (int[]),
  modifier_type, modifier_value,
  min_nights, max_nights, min_days_ahead, max_days_ahead,
  priority, enabled
```

### 5.3 Bookings
```sql
bookings
  id, reference (short code), hotel_id, room_type_id, room_count,
  guest_email, guest_phone, guest_name, guest_country, special_request,
  check_in_date, check_out_date, nights,
  currency, room_subtotal, taxes, fees, discounts, total_amount,
  status (pending_payment|confirmed|cancelled|checked_in|checked_out|no_show|expired),
  expires_at, cancelled_at, cancelled_by, cancellation_reason,
  payment_method, payment_status,
  source (web|walk_in|phone), utm_*, referrer,
  created_at, confirmed_at, checked_in_at, checked_out_at
  -- price breakdown ถูก snapshot — แก้ราคาภายหลังไม่กระทบ

booking_events              -- audit trail
  id, booking_id, event_type, actor_type, actor_id, payload (jsonb), at
```

### 5.4 Landing Page
```sql
landing_pages
  hotel_id, locale, version, status (draft|published),
  branding (jsonb), sections (jsonb[]), seo (jsonb),
  tracking (jsonb -- fb_pixel, ga, gtm, line_tag, google_ads),
  updated_at
  PRIMARY KEY (hotel_id, locale, version)
```

### 5.5 Subscription
```sql
subscriptions
  id, account_id, status, plan_code, billing_cycle,
  trial_started_at, trial_ends_at,
  current_period_start, current_period_end,
  room_count, unit_price, currency,
  payment_provider, payment_method_id,
  cancelled_at, suspended_at, created_at

subscription_events         -- audit
invoices                    -- monthly invoices
payment_attempts            -- dunning history
```

### 5.6 Reviews
```sql
reviews
  id, booking_id, hotel_id, rating, title, body,
  photos (text[]), status (pending|published|rejected),
  hotel_response, response_at, created_at
```

---

## 6. Key Flows

### 6.1 Guest Booking Flow

```
Ad/Social → click → Landing page (CDN, <1.5s LCP)
   ↓ select dates + guests
Availability check (API)
   ↓ select room type + rate plan
Checkout page (single page funnel)
   ↓ guest info + payment selection
Payment processing (hold 10 min)
   ↓ confirm
Confirmation + email + LINE
   ↓ fire conversion pixel
```

### 6.2 Booking State Machine
```
initiating → pending_payment → confirmed → checked_in → checked_out → completed
                  │                │
                  ↓                ↓
              expired          cancelled / no_show
```

### 6.3 Race Condition Prevention
```
BEGIN TX
  SELECT availability_overrides + room_types FOR UPDATE
  Check: total_inventory + override - bookings >= requested
  INSERT booking (status=pending_payment, expires_at=NOW+10min)
COMMIT

Worker every 1 min: expire pending_payment ที่หมดเวลา
```

### 6.4 Payment Options (เงินตรงโรงแรม)
1. **PromptPay QR + manual confirm** — phase 1 default ในไทย
2. **Hotel's own Omise/Stripe** — โรงแรมที่มี gateway แล้ว
3. **Pay at hotel** — no online payment
4. **Slip OCR auto-verify** — Phase 2

### 6.5 Subscription Lifecycle
```
pending_kyc → trialing (เริ่มนับเมื่อ first booking หรือ +60d max)
            → trial_ending (30d ก่อนหมด)
            → trial_lapsed (grace 14d ถ้าไม่ใส่บัตร)
            → suspended (read-only 30d)
            → cancelled (archive 90d)
            → terminated (hard delete)
```

### 6.6 Hotel Onboarding Wizard (7 steps)
1. Signup (email/Google/LINE)
2. Hotel basics (name, type, location — Google Places autofill)
3. Room types (with template suggestions)
4. Photos (drag-drop, min 5, allow skip)
5. Policies (recommend Flexible)
6. Payment methods
7. Booking page customization + finish

KYC แยก async — ไม่บล็อก setup, แค่บล็อก go-live

---

## 7. Landing Page Builder Architecture

### Approach: Structured template + content customization
**ไม่ใช่** free-form page builder. โรงแรมเลือก content แต่ layout เราคุม

### Section catalog (Phase 1)
Hero → Gallery → About → Room Types → Amenities → Location → Reviews → FAQ → Policies → Contact

แต่ละ section: toggle on/off + reorder + structured fields

### Rendering: Next.js ISR + CDN
- `[slug]/page.tsx` fetches landing_page config
- Static-generated, revalidate on update
- LCP target < 1.5s on 4G mobile

### Customizable
รูป, ข้อความ, brand color (primary + accent), logo, ลำดับ section, SEO meta, tracking IDs

### Not customizable (intentional)
Layout structure, fonts (preset 4-5 ตัว), custom CSS/JS

### Multi-language
- Hotel admin กรอกแต่ละ locale แยก (TH/EN Phase 1)
- URL: `/[slug]/[locale]` หรือ `?lang=`
- Phase 2: auto-translate suggestion (DeepL/GPT)

### Marketing integration (ทุกโรงแรม)
- FB Pixel, Google Analytics, Google Ads conversion, GTM, LINE Tag, TikTok Pixel
- Auto-fire events: PageView, ViewContent, InitiateCheckout, AddPaymentInfo, Purchase
- Consent management (PDPA/GDPR) ก่อน fire

---

## 8. Pricing & Availability Engine

### Calculation order
```
1. base_rate (from room_type)
2. apply day-of-week rule (e.g., weekend +20%)
3. apply season rule (overrides)
4. apply date-specific override (top priority — manual)
5. sum per-night
6. apply length-of-stay modifier (-X% if >= N nights)
7. add extras (breakfast, etc.)
8. add taxes (VAT 7% ในไทย)
9. add fees (service, tourist tax)
10. apply promo code
= TOTAL
```

### Multi-currency
- Hotel base currency: typically THB
- Display: guest's locale or selected
- Lock exchange rate on booking creation
- Settle in hotel's base currency

### Restrictions
Min nights, Max nights, CTA (closed to arrival), CTD (closed to departure), Advance purchase, Stop sell

### Money handling rules
- ใช้ DECIMAL หรือ int (minor units) — ห้าม float
- Snapshot price ในแต่ละ booking
- Exchange rate snapshot ใน booking

---

## 9. Roadmap

### Phase 0 — Foundation (3-5 wk — ลากนานขึ้นเพราะ self-host ops)
**Code:**
- Monorepo / repo structure
- Go API skeleton (modules: auth, hotel, room, booking, billing, notification)
- Next.js skeleton (apps: marketing site, booking, admin)
- Shared TypeScript types from Go (gen ผ่าน OpenAPI หรือ codegen)
- Auth (hotel staff)
- Basic data model migrations
- Dev tooling (linters, formatters, pre-commit)

**Ops (critical — self-host):**
- Provision Hetzner servers + Cloudflare DNS
- Docker Compose stack: Caddy, Go API, Next.js, Postgres, Redis, imgproxy, MinIO
- GitHub Actions deploy pipeline
- Daily Postgres backup → Backblaze B2 + restore test
- UptimeRobot + log aggregation (Loki or BetterStack)
- Sentry self-host หรือ SaaS tier
- Secret management (sops + age)
- Staging vs. production environment
- SSL auto-renewal verified

**Customer research (parallel — co-founder/PM):**
- เริ่ม interview hotel owners 5-10 ราย
- Competitor pricing audit
- Pricing landing page (collect emails + WTP signals)

### Phase 1 — Booking MVP (6-10 wk)
**Goal: โรงแรมแรกรับ booking จริงผ่าน ads**
- Hotel signup + onboarding wizard
- Landing page (structured template + customize)
- Booking flow (availability check + checkout + email confirm)
- Payment: PromptPay QR + manual confirm
- Hotel admin: calendar view, booking list, walk-in
- Subscription state tracking (ทุกคน trial)
- Pixel/tracking integration (FB, GA, GTM)
- KYC submission + review
- English locale on guest side

### Phase 2 — Polish & Conversion (4-6 wk)
- Mobile-optimized admin (PWA)
- LINE OA notification
- Conversion dashboard ให้โรงแรม
- A/B testing landing pages
- Reviews (after-stay)
- Walk-in booking polish
- Slip OCR auto-verify
- Multi-language hotel content (auto-translate suggestion)
- Promo codes
- Non-refundable rate plan
- Length-of-stay & advance-purchase rules
- Pre-launch subscription payment capture flow

### Phase 3 — Scale & Retain (ก่อนหมด trial ปี)
- **Subscription billing เปิดใช้จริง + dunning**
- **Channel manager** (Booking.com/Agoda sync) — critical
- Custom domain support (book.zenhostel.com)
- Advanced pricing rules
- Multi-property
- API access for power users
- Yield management suggestions (AI)
- Annual plans, promo codes for subscription

### Phase 4+ (post-launch)
- Mobile native apps (optional)
- More OTA channels
- Booking.com extranet-like features
- White-label option for chains

---

## 10. Critical Considerations

### 10.1 Speed = Revenue
Landing page LCP > 2.5s → conversion drop 7-15% → ad budget wasted
- ISR + CDN
- Image optimization (WebP/AVIF, srcset, lazy load)
- Critical CSS inlined
- Defer 3rd-party scripts (analytics ขนาดเล็ก)

### 10.2 Security & Compliance
- **PDPA** (ไทย) compliance Day 1
- **GDPR-ready** สำหรับลูกค้าต่างชาติ
- Cookie consent ก่อน fire tracking pixel
- 2FA for hotel admin
- Audit log ทุก sensitive action
- Rate limiting, CAPTCHA on booking
- Idempotency keys
- Hotel KYC: ใบทะเบียน, ID

### 10.3 Race Conditions
- Booking: SELECT FOR UPDATE + pending hold
- Idempotency key on POST /bookings
- Cleanup worker for expired holds
- Distributed lock (Redis) ถ้า scale ใหญ่

### 10.4 Multi-tenancy Isolation
- Row-level security where possible
- Account-scoped queries everywhere
- Slug uniqueness + collision handling
- Subdomain/path tenancy on landing pages

### 10.5 Money & Tax (ไทย)
- VAT 7%
- e-Tax invoice generation
- Withholding tax 3% สำหรับ B2B subscription
- Tourist tax (พิเศษบางจังหวัด)
- Currency rounding rules

### 10.6 Timezone Handling
- Store UTC in DB
- All display in hotel.timezone
- check-in date = "วันที่ในโรงแรม"
- Critical for: pricing rules, booking dates, availability cleanup, notifications

### 10.7 Failure Modes
- Payment webhook missed → reconciliation job ทุก 6 ชม.
- Image upload fails → retry queue
- Email bounce → mark + alternative channel (LINE)
- Hotel cancel booking → guest notification + refund flow
- System outage → guest sees friendly error + can resume booking

---

## 11. Anti-features (สิ่งที่ตั้งใจไม่ทำ)

- ❌ Discovery/search across hotels (เราไม่ใช่ marketplace)
- ❌ Hotel-to-hotel comparison
- ❌ Free-form page builder (custom CSS/JS)
- ❌ Payment escrow / marketplace payouts
- ❌ Commission billing model
- ❌ Mobile native app Phase 1 (use mobile web)
- ❌ Booking.com-style ranking algorithm
- ❌ Live chat between guests pre-booking (เกินขอบเขต)

---

## 12. Decisions Log

### ✅ Decided

- **Frontend:** Next.js App Router
- **Backend:** Go
- **Hosting:** Self-host on Hetzner Cloud + Cloudflare CDN
- **Orchestration:** Docker Compose (not k8s)
- **Database:** PostgreSQL self-host with B2 off-site backup
- **Image processing:** imgproxy self-host
- **Object storage:** MinIO or Backblaze B2
- **Reverse proxy / SSL:** Caddy + Let's Encrypt
- **CI/CD:** GitHub Actions + (optionally Coolify)
- **Custom domain (Phase 3):** Caddy + Let's Encrypt (auto-issue per hotel domain)
- **Channel manager (Phase 3):** Integrate with aggregator (SiteMinder/Hotellink) — not build
- **e-Tax invoice (Phase 2):** Leceipt or other ETDA-approved provider
- **AI features:** API-based (Claude/OpenAI/DeepL/iApp), no model training, Phase 2+
- **Payment method capture during trial:** Optional, push hard at day 270+
- **Team:** 2-3 dev (BE/Ops, FE, shared/QA)
- **Subscription tier design principle:** feature-flag driven, not hardcoded — allow pricing changes without redeploy

### ⏳ Pending

- **Pricing tiers (exact ฿):** TBD after customer research during Phase 0-1
  - Approach: 10-15 hotel owner interviews + competitor audit + landing page A/B before launch
  - Placeholder draft: Starter ~฿299-499, Growth ~฿699-999, Pro ~฿1499-2499/mo
- **Server region:** Hetzner Helsinki (cheap, ~250ms latency mitigated by Cloudflare edge) vs. DO/Vultr Singapore (2-3x cost, low latency)
- **Designer:** in-team vs. freelance Phase 2

### 📝 Notes

- Subscription billing internal still uses Stripe + Omise (we are billing the hotel, not facilitating guest payment)
- Self-host means budgeting **20-30% of dev time on ops setup** during Phase 0-1

---

## 13. Glossary

| Term | Definition |
|---|---|
| OTA | Online Travel Agency (Booking.com, Agoda, Expedia) |
| PMS | Property Management System |
| BAR | Best Available Rate |
| LOS | Length of Stay |
| MLOS | Minimum Length of Stay |
| CTA | Closed To Arrival |
| CTD | Closed To Departure |
| ISR | Incremental Static Regeneration |
| LCP | Largest Contentful Paint |
| PDPA | Personal Data Protection Act (Thailand) |
| Slug | URL-safe identifier (e.g., `zen-hostel-chiangmai`) |
| Channel Manager | Software that syncs availability/pricing across multiple OTAs |
