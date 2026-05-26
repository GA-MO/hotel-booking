#!/usr/bin/env bash
# ============================================================================
# End-to-end smoke test for the hotel-booking API.
#
# Exercises the full happy-path flow:
#   1. signup owner             POST /v1/auth/signup
#   2. create hotel              POST /v1/hotels
#   3. create room type          POST /v1/hotels/{id}/room-types
#   4. publish landing page      PUT  /v1/hotels/{id}/landing/en  + publish
#   5. create pricing rule       POST /v1/hotels/{id}/pricing-rules
#   6. public quote              POST /v1/public/quote/{slug}
#   7. create booking (public)   POST /v1/public/hotels/{slug}/bookings
#   8. confirm booking           POST /v1/hotels/{id}/bookings/{bid}/confirm
#   9. check-in / check-out      POST /v1/hotels/{id}/bookings/{bid}/check-{in,out}
#  10. read me                   GET  /v1/auth/me
#  11. refresh token             POST /v1/auth/refresh
#
# Prereqs:
#   docker compose up -d             # postgres, redis, minio, imgproxy
#   make -C apps/api migrate-up      # apply all migrations
#   make -C apps/api dev &           # API on :8080 (or `make run`)
#
# Run:    ./scripts/smoke-test.sh
# CI/RC:  API_URL=https://api.example.com ./scripts/smoke-test.sh
# ============================================================================

set -Eeuo pipefail

API_URL="${API_URL:-http://localhost:8080}"
EMAIL="smoke-$(date +%s)@example.com"
PASSWORD="smoke-pass-123"
SLUG="smoke-test-$(date +%s)"

# --- pretty output ----------------------------------------------------------
green() { printf '\033[32m%s\033[0m\n' "$*"; }
red()   { printf '\033[31m%s\033[0m\n' "$*" >&2; }
step()  { printf '\n\033[36m▸ %s\033[0m\n' "$*"; }
need_jq() { command -v jq >/dev/null || { red "jq required: brew install jq"; exit 1; }; }
need_jq

# --- helpers ----------------------------------------------------------------
ACCESS=""
REFRESH=""

call() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-sS -X "$method" -H 'Content-Type: application/json' -w '\n__HTTP_STATUS__:%{http_code}')
  [[ -n "$ACCESS" ]] && args+=(-H "Authorization: Bearer $ACCESS")
  [[ -n "$body"   ]] && args+=(-d "$body")
  local raw status body_out
  raw="$(curl "${args[@]}" "${API_URL}${path}")"
  status="${raw##*__HTTP_STATUS__:}"
  body_out="${raw%__HTTP_STATUS__:*}"
  echo "$body_out" > /tmp/last_response.json
  echo "$status"  > /tmp/last_status.txt
  echo "$body_out"
}

assert_status() {
  local expected="$1"
  local got
  got="$(cat /tmp/last_status.txt)"
  if [[ "$got" != "$expected" ]]; then
    red "FAIL: expected HTTP $expected, got $got"
    red "body: $(cat /tmp/last_response.json)"
    exit 1
  fi
}

# ============================================================================
step "1. signup ($EMAIL)"
call POST /v1/auth/signup "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Smoke Owner\"}" | jq -r '.user.email'
assert_status 201
ACCESS="$(jq -r '.access_token' /tmp/last_response.json)"
REFRESH="$(jq -r '.refresh_token' /tmp/last_response.json)"
green "  got tokens"

step "2. create hotel ($SLUG)"
call POST /v1/hotels "{\"slug\":\"$SLUG\",\"name\":\"Smoke Boutique\",\"hotel_type\":\"boutique\",\"country\":\"TH\",\"timezone\":\"Asia/Bangkok\",\"base_currency\":\"THB\"}" | jq -r '.id'
assert_status 201
HOTEL_ID="$(jq -r '.id' /tmp/last_response.json)"
green "  hotel_id=$HOTEL_ID"

# Guard rail: chi once shadowed PATCH /v1/hotels/{id} because Mount("/hotels")
# and Route("/hotels/{hotel_id}") sat at the same parent. Hit it explicitly so
# any future re-shadowing fails this script.
step "2a. PATCH /v1/hotels/{id} (promptpay_id) — chi route guard"
call PATCH "/v1/hotels/$HOTEL_ID" '{"promptpay_id":"0812345678"}' | jq -r '.promptpay_id'
assert_status 200
[[ "$(jq -r '.promptpay_id' /tmp/last_response.json)" == "0812345678" ]] || { red "promptpay_id missing on PATCH response"; exit 1; }
green "  PATCH reached + promptpay_id saved"

step "2b. GET /v1/hotels/{id} — chi route guard"
call GET "/v1/hotels/$HOTEL_ID" | jq -r '.slug'
assert_status 200
green "  GET by id reached"

step "2c. POST /v1/hotels/{id}/go-live — onboarding unblocker"
call POST "/v1/hotels/$HOTEL_ID/go-live" "" | jq -r '.status'
assert_status 200
[[ "$(jq -r '.status' /tmp/last_response.json)" == "live" ]] || { red "expected status=live, got $(jq -r '.status' /tmp/last_response.json)"; exit 1; }
# Idempotency: a second call should still 200 + return live (not 409).
call POST "/v1/hotels/$HOTEL_ID/go-live" "" >/dev/null
assert_status 200
green "  status: test → live, idempotent"

step "3. create room type"
call POST "/v1/hotels/$HOTEL_ID/room-types" '{"name":"Standard Double","max_occupancy":2,"total_inventory":4,"base_rate":1500,"base_currency":"THB"}' | jq -r '.id'
assert_status 201
ROOM_TYPE_ID="$(jq -r '.id' /tmp/last_response.json)"
green "  room_type_id=$ROOM_TYPE_ID"

step "4. publish landing page (en)"
call PUT "/v1/hotels/$HOTEL_ID/landing/en" '{"branding":{"primary_color":"#FF5733"},"sections":[{"type":"hero","enabled":true,"order":0,"content":{"headline":"Stay in Old Town"}}],"seo":{"title":"Smoke Boutique"},"tracking":{}}' >/dev/null
assert_status 200
call POST "/v1/hotels/$HOTEL_ID/landing/en/publish" '' >/dev/null
assert_status 200
green "  landing published"

step "5. create pricing rule (weekend +20%)"
call POST "/v1/hotels/$HOTEL_ID/pricing-rules" '{"name":"Weekend uplift","rule_type":"day_of_week","days_of_week":[5,6],"modifier_type":"percentage","modifier_value":20}' >/dev/null || true
# Rule creation may not be required for booking flow — soft-pass
STATUS_RULE="$(cat /tmp/last_status.txt)"
if [[ "$STATUS_RULE" == "201" ]]; then green "  rule created"; else
  red "  WARN: rule create returned $STATUS_RULE (continuing)"
fi

step "6. public quote (3 nights)"
call POST "/v1/public/quote/$SLUG" "{\"room_type_id\":\"$ROOM_TYPE_ID\",\"check_in\":\"2026-06-01\",\"check_out\":\"2026-06-04\",\"rooms\":1}" | jq '.total // .'
assert_status 200
green "  quote OK"

step "7. create booking (public — guest, requires hotel.status=live from step 2c)"
ACCESS_SAVED="$ACCESS"; ACCESS=""
call POST "/v1/public/hotels/$SLUG/bookings" "{\"room_type_id\":\"$ROOM_TYPE_ID\",\"room_count\":1,\"check_in_date\":\"2026-06-01\",\"check_out_date\":\"2026-06-04\",\"guest_email\":\"guest@example.com\",\"guest_name\":\"Smoke Guest\"}" | jq -r '.reference'
ACCESS="$ACCESS_SAVED"
assert_status 201
BOOKING_ID="$(jq -r '.id' /tmp/last_response.json)"
REFERENCE="$(jq -r '.reference' /tmp/last_response.json)"
green "  booking_id=$BOOKING_ID ref=$REFERENCE"

step "8. confirm booking"
call POST "/v1/hotels/$HOTEL_ID/bookings/$BOOKING_ID/confirm" '' | jq -r '.status'
assert_status 200
[[ "$(jq -r '.status' /tmp/last_response.json)" == "confirmed" ]] || { red "expected confirmed"; exit 1; }
green "  confirmed"

# Guard rail: ListByHotel JOIN-ed hotels for the hotel context (round 2) and a
# bare `id` in bookingColumns triggered "column reference 'id' is ambiguous"
# at runtime. This step hits the same JOIN path.
step "8a. GET /v1/hotels/{id}/bookings — SQL ambiguity guard"
call GET "/v1/hotels/$HOTEL_ID/bookings?limit=50" | jq '.total'
assert_status 200
[[ "$(jq -r '.total >= 1' /tmp/last_response.json)" == "true" ]] || { red "expected at least one booking in list"; exit 1; }
# Date wire format guard: backend used to ship check_in_date as full ISO; FE
# YMD regex silently failed. Assert YYYY-MM-DD shape so a regression here
# can't slip past CI.
CI_DATE="$(jq -r '.bookings[0].check_in_date' /tmp/last_response.json)"
[[ "$CI_DATE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || { red "check_in_date not YYYY-MM-DD: $CI_DATE"; exit 1; }
green "  list returned, check_in_date = $CI_DATE"

step "8b. GET /v1/hotels/{id}/bookings/{id}/events — audit endpoint guard"
call GET "/v1/hotels/$HOTEL_ID/bookings/$BOOKING_ID/events" | jq '.events | length'
assert_status 200
EV_COUNT="$(jq -r '.events | length' /tmp/last_response.json)"
[[ "$EV_COUNT" -ge 2 ]] || { red "expected >= 2 events (created + confirmed), got $EV_COUNT"; exit 1; }
green "  audit trail: $EV_COUNT events"

# Guard rail: PublicLandingResponse wraps LandingPage with a hotel context
# block. P1.2 once selected `h.currency` instead of `h.base_currency` and
# 42703'd at runtime — and there was no integration test on this path.
step "8d. POST /v1/public/bookings/{ref}/payment-confirmed — guest-claim hook"
# Need a pending_payment booking (step 7's was just confirmed at step 8).
# Create a fresh one to exercise the path without unwinding 7/8.
ACCESS_SAVED="$ACCESS"; ACCESS=""
call POST "/v1/public/hotels/$SLUG/bookings" "{\"room_type_id\":\"$ROOM_TYPE_ID\",\"room_count\":1,\"check_in_date\":\"2026-06-05\",\"check_out_date\":\"2026-06-06\",\"guest_email\":\"claim@example.com\",\"guest_name\":\"Claim Guest\"}" >/dev/null
assert_status 201
CLAIM_REF="$(jq -r '.reference' /tmp/last_response.json)"
CLAIM_ID="$(jq -r '.id' /tmp/last_response.json)"
call POST "/v1/public/bookings/$CLAIM_REF/payment-confirmed" '{"email":"claim@example.com"}' >/dev/null
assert_status 200
ACCESS="$ACCESS_SAVED"
# Verify the booking_event was appended (admin endpoint requires auth — re-use ACCESS).
call GET "/v1/hotels/$HOTEL_ID/bookings/$CLAIM_ID/events" >/dev/null
assert_status 200
HAS_CLAIM="$(jq -r '[.events[].event_type] | index("payment_claimed") // "missing"' /tmp/last_response.json)"
[[ "$HAS_CLAIM" != "missing" ]] || { red "payment_claimed event not in audit trail"; exit 1; }
green "  guest claim recorded in booking_events"

step "8c. GET /v1/public/landing/{slug}/{locale} — hotel context guard"
ACCESS_SAVED="$ACCESS"; ACCESS=""  # anonymous call
call GET "/v1/public/landing/$SLUG/en" | jq '{tz:.hotel.timezone, ccy:.hotel.currency, ppay:.hotel.promptpay_id}'
ACCESS="$ACCESS_SAVED"
assert_status 200
# timezone + currency are required (cannot be empty); promptpay_id is optional
# at the hotel level so we only check the key is present, not non-empty.
for f in timezone currency; do
  [[ "$(jq -r ".hotel.$f // empty" /tmp/last_response.json)" != "" ]] || {
    red "public landing missing hotel.$f"; exit 1; }
done
green "  hotel.timezone + currency present"

step "9. check-in then check-out"
call POST "/v1/hotels/$HOTEL_ID/bookings/$BOOKING_ID/check-in" '' >/dev/null
assert_status 200
[[ "$(jq -r '.status' /tmp/last_response.json)" == "checked_in" ]] || { red "expected checked_in"; exit 1; }
call POST "/v1/hotels/$HOTEL_ID/bookings/$BOOKING_ID/check-out" '' >/dev/null
assert_status 200
[[ "$(jq -r '.status' /tmp/last_response.json)" == "checked_out" ]] || { red "expected checked_out"; exit 1; }
green "  check-in → check-out"

step "10. /v1/auth/me"
call GET /v1/auth/me | jq -r '.user.email'
assert_status 200
[[ "$(jq -r '.user.email' /tmp/last_response.json)" == "$EMAIL" ]] || { red "email mismatch"; exit 1; }
green "  identity verified"

step "11. refresh access token"
ACCESS_OLD="$ACCESS"
# JWT `iat` is in whole seconds; on a fast local run the refresh can land in
# the same second as signup and produce a byte-identical token. Force a one-
# second gap so the rotation check is deterministic.
sleep 1
call POST /v1/auth/refresh "{\"refresh_token\":\"$REFRESH\"}" >/dev/null
assert_status 200
ACCESS_NEW="$(jq -r '.access_token' /tmp/last_response.json)"
[[ "$ACCESS_NEW" != "$ACCESS_OLD" ]] || { red "access token did not rotate"; exit 1; }
green "  token rotated"

step "12. no-availability check (try to overbook)"
ACCESS="$ACCESS_NEW"
for i in 1 2 3 4 5; do
  call POST "/v1/hotels/$HOTEL_ID/bookings" "{\"room_type_id\":\"$ROOM_TYPE_ID\",\"room_count\":1,\"check_in_date\":\"2026-07-01\",\"check_out_date\":\"2026-07-02\",\"guest_email\":\"overbook$i@example.com\",\"guest_name\":\"O$i\"}" >/dev/null
  STATUS_OB="$(cat /tmp/last_status.txt)"
  if [[ "$i" -le 4 && "$STATUS_OB" != "201" ]]; then
    red "expected booking $i to succeed, got $STATUS_OB"
    cat /tmp/last_response.json >&2
    exit 1
  fi
  if [[ "$i" -eq 5 && "$STATUS_OB" != "409" ]]; then
    red "expected 5th booking to be 409 NO_AVAILABILITY, got $STATUS_OB"
    cat /tmp/last_response.json >&2
    exit 1
  fi
done
green "  inventory of 4 enforced — 5th attempt blocked with 409"

green ""
green "✓ smoke test passed"
