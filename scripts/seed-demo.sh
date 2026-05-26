#!/usr/bin/env bash
# Seed a demo account + hotel + room type + landing pages, then flip the
# hotel to status='live' so the public booking-web pages resolve. Each
# invocation creates a fresh hotel (slug is timestamped) so this is safe
# to run repeatedly. Prints credentials + URLs at the end.
set -Eeuo pipefail

API_URL="${API_URL:-http://localhost:8080}"
EMAIL="demo-$(date +%s)@example.com"
PASSWORD="demo-pass-123"
SLUG="demo-$(date +%s)"
PROMPTPAY_ID="0812345678"

command -v jq >/dev/null || { echo "jq required: brew install jq" >&2; exit 1; }

green() { printf '\033[32m%s\033[0m\n' "$*"; }
step()  { printf '\n\033[36m▸ %s\033[0m\n' "$*"; }

ACCESS=""

call() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-sS -X "$method" -H 'Content-Type: application/json' -w '\n__HTTP_STATUS__:%{http_code}')
  [[ -n "$ACCESS" ]] && args+=(-H "Authorization: Bearer $ACCESS")
  [[ -n "$body"   ]] && args+=(-d "$body")
  local raw status body_out
  raw="$(curl "${args[@]}" "${API_URL}${path}")"
  status="${raw##*__HTTP_STATUS__:}"
  body_out="${raw%__HTTP_STATUS__:*}"
  echo "$body_out" > /tmp/seed_response.json
  echo "$status"  > /tmp/seed_status.txt
  echo "$body_out"
}

expect() {
  local got; got="$(cat /tmp/seed_status.txt)"
  if [[ "$got" != "$1" ]]; then
    echo "FAIL: expected HTTP $1, got $got" >&2
    cat /tmp/seed_response.json >&2
    exit 1
  fi
}

step "signup ($EMAIL)"
call POST /v1/auth/signup "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Demo Owner\"}" >/dev/null
expect 201
ACCESS="$(jq -r '.access_token' /tmp/seed_response.json)"
REFRESH="$(jq -r '.refresh_token' /tmp/seed_response.json)"

step "create hotel ($SLUG)"
HOTEL_BODY="{\"slug\":\"$SLUG\",\"name\":\"Demo Boutique\",\"hotel_type\":\"boutique\",\"country\":\"TH\",\"timezone\":\"Asia/Bangkok\",\"base_currency\":\"THB\"}"
call POST /v1/hotels "$HOTEL_BODY" >/dev/null
expect 201
HOTEL_ID="$(jq -r '.id' /tmp/seed_response.json)"
green "  hotel_id=$HOTEL_ID"

step "set promptpay_id (so guest QR is real)"
call PATCH "/v1/hotels/$HOTEL_ID" "{\"promptpay_id\":\"$PROMPTPAY_ID\"}" >/dev/null
expect 200

step "create room type"
ROOM_BODY='{"name":"Garden Studio","description":"Quiet corner with garden view","total_inventory":5,"max_occupancy":2,"base_rate":1800,"base_currency":"THB"}'
call POST "/v1/hotels/$HOTEL_ID/room-types" "$ROOM_BODY" >/dev/null
expect 201
ROOM_TYPE_ID="$(jq -r '.id' /tmp/seed_response.json)"

# booking-web has no direct /room-types public endpoint; it reads the catalog
# from landing.sections[type=rooms].content.rooms so the admin's "publish"
# action carries the bookable rooms onto the public page. Stuff our room into
# that block so the demo's /book page actually has a room to select.
ROOMS_JSON=$(jq -n --arg id "$ROOM_TYPE_ID" '[
  {id:$id, name:"Garden Studio", base_rate:"1800.00", base_currency:"THB", max_occupancy:2}
]')

step "upsert + publish landing (th)"
LANDING_BODY=$(jq -n --argjson rooms "$ROOMS_JSON" '{
  branding:{primary_color:"#0F766E", accent_color:"#F59E0B", font_family:"Inter"},
  sections:[
    {type:"hero", enabled:true, order:0, content:{headline:"พักผ่อนใจกลางเมือง", subhead:"Demo Boutique"}},
    {type:"rooms", enabled:true, order:1, content:{rooms:$rooms}},
    {type:"amenities", enabled:true, order:2, content:{items:["Wi-Fi","สระว่ายน้ำ","อาหารเช้า"]}}
  ],
  seo:{title:"Demo Boutique", description:"โรงแรมตัวอย่างสำหรับลองเล่นระบบ"},
  tracking:{}
}')
call PUT "/v1/hotels/$HOTEL_ID/landing/th" "$LANDING_BODY" >/dev/null
expect 200
call POST "/v1/hotels/$HOTEL_ID/landing/th/publish" "" >/dev/null
expect 200

step "upsert + publish landing (en)"
LANDING_EN=$(jq -n --argjson rooms "$ROOMS_JSON" '{
  branding:{primary_color:"#0F766E", accent_color:"#F59E0B", font_family:"Inter"},
  sections:[
    {type:"hero", enabled:true, order:0, content:{headline:"A quiet city escape", subhead:"Demo Boutique"}},
    {type:"rooms", enabled:true, order:1, content:{rooms:$rooms}},
    {type:"amenities", enabled:true, order:2, content:{items:["Wi-Fi","Pool","Breakfast"]}}
  ],
  seo:{title:"Demo Boutique", description:"Sample hotel for trying out the platform"},
  tracking:{}
}')
call PUT "/v1/hotels/$HOTEL_ID/landing/en" "$LANDING_EN" >/dev/null
expect 200
call POST "/v1/hotels/$HOTEL_ID/landing/en/publish" "" >/dev/null
expect 200

step "flip hotel to live (dev shortcut — bypasses KYC)"
docker compose exec -T postgres psql -U hotel hotel_booking -c \
  "UPDATE hotels SET status='live' WHERE id='$HOTEL_ID';" >/dev/null
green "  hotels.status = 'live'"

# Sanity check the public endpoint.
step "verify /v1/public/landing/$SLUG/th"
HTTP="$(curl -sS -o /dev/null -w '%{http_code}' "$API_URL/v1/public/landing/$SLUG/th")"
[[ "$HTTP" == "200" ]] || { echo "public landing returned $HTTP" >&2; exit 1; }
green "  200 OK"

cat <<EOF

==============================================================================
DEMO READY
==============================================================================

Guest (booking-web):
  TH:  http://localhost:3000/$SLUG?lang=th
  EN:  http://localhost:3000/$SLUG?lang=en

Admin (admin-web):
  http://localhost:3001/login
  email:    $EMAIL
  password: $PASSWORD

Hotel internals:
  hotel_id:      $HOTEL_ID
  room_type_id:  $ROOM_TYPE_ID
  promptpay_id:  $PROMPTPAY_ID  (the QR on the confirmation page uses this)

Try this happy path:
  1. Open the TH landing URL above → click "Book"
  2. Pick dates + guest info, complete checkout
  3. Confirmation page renders a real EMVCo PromptPay QR
  4. Log in to admin → /bookings → confirm or cancel the reservation
  5. Audit timeline at /bookings/{id} shows the events
==============================================================================
EOF
