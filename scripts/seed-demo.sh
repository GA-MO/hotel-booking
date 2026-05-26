#!/usr/bin/env bash
# Seed a demo account + hotel with realistic content: 4 room types, a rich
# multi-section landing page (TH + EN), and ~12 bookings spread across every
# lifecycle state (pending / confirmed / checked-in / checked-out / cancelled
# / no-show). Slugs are timestamped so re-running is safe. Prints credentials
# + URLs at the end.
#
# Requires: curl, jq, BSD `date -v` (macOS). Hits the API at $API_URL
# (default http://localhost:8080).
set -Eeuo pipefail

API_URL="${API_URL:-http://localhost:8080}"
EMAIL="demo-$(date +%s)@example.com"
PASSWORD="demo-pass-123"
SLUG="demo-$(date +%s)"
PROMPTPAY_ID="0812345678"
HOTEL_NAME="The Sukhumvit Garden Boutique"

command -v jq >/dev/null || { echo "jq required: brew install jq" >&2; exit 1; }

green() { printf '\033[32m%s\033[0m\n' "$*"; }
step()  { printf '\n\033[36m▸ %s\033[0m\n' "$*"; }
sub()   { printf '  \033[90m%s\033[0m\n' "$*"; }

ACCESS=""

# call <METHOD> <PATH> [BODY] — emits response body to stdout, also stashes
# body + status in /tmp/seed_response.json + /tmp/seed_status.txt for expect().
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

# d <offset_days> → YYYY-MM-DD relative to today (BSD `date -v`).
d() {
  local off="$1" sign="+"
  [[ "$off" == -* ]] && { sign="-"; off="${off#-}"; }
  date -v"${sign}${off}d" +%Y-%m-%d
}

# ----------------------------------------------------------------------------
# 1. Account + hotel
# ----------------------------------------------------------------------------

step "signup ($EMAIL)"
call POST /v1/auth/signup "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Demo Owner\"}" >/dev/null
expect 201
ACCESS="$(jq -r '.access_token' /tmp/seed_response.json)"

step "create hotel ($SLUG)"
HOTEL_BODY=$(jq -n --arg slug "$SLUG" --arg name "$HOTEL_NAME" '{
  slug: $slug,
  name: $name,
  hotel_type: "boutique",
  country: "TH",
  timezone: "Asia/Bangkok",
  base_currency: "THB"
}')
call POST /v1/hotels "$HOTEL_BODY" >/dev/null
expect 201
HOTEL_ID="$(jq -r '.id' /tmp/seed_response.json)"
green "  hotel_id=$HOTEL_ID"

step "set promptpay_id (so guest QR is real)"
call PATCH "/v1/hotels/$HOTEL_ID" "{\"promptpay_id\":\"$PROMPTPAY_ID\"}" >/dev/null
expect 200

# ----------------------------------------------------------------------------
# 2. Room types — 4 categories, mixing inventory + price points
# ----------------------------------------------------------------------------

# create_room_type <name> <description> <inventory> <occupancy> <base_rate> <size_sqm>
# Echoes the new room_type_id.
create_room_type() {
  local name="$1" desc="$2" inv="$3" occ="$4" rate="$5" size="$6"
  local body
  body=$(jq -n \
    --arg name "$name" --arg desc "$desc" \
    --argjson inv "$inv" --argjson occ "$occ" \
    --argjson rate "$rate" --argjson size "$size" \
    '{name:$name, description:$desc, total_inventory:$inv, max_occupancy:$occ,
      base_rate:$rate, base_currency:"THB", size_sqm:$size}')
  call POST "/v1/hotels/$HOTEL_ID/room-types" "$body" >/dev/null
  expect 201
  jq -r '.id' /tmp/seed_response.json
}

step "create 4 room types"
RT_STUDIO=$(create_room_type \
  "Garden Studio" \
  "Quiet corner studio with a small private balcony overlooking the koi pond." \
  6 2 1800 24)
sub "Garden Studio        → $RT_STUDIO"

RT_TWIN=$(create_room_type \
  "Standard Twin" \
  "Twin beds, work desk, blackout curtains — built for the practical traveller." \
  8 2 2200 26)
sub "Standard Twin        → $RT_TWIN"

RT_KING=$(create_room_type \
  "Deluxe King" \
  "King bed, rain shower, espresso machine, and a soaking tub by the window." \
  4 2 3200 32)
sub "Deluxe King          → $RT_KING"

RT_SUITE=$(create_room_type \
  "Family Suite" \
  "Two bedrooms, a living area, and a kitchenette — sleeps four comfortably." \
  2 4 5400 52)
sub "Family Suite         → $RT_SUITE"

# Room cards used by both landing locales — booking-web reads rooms straight
# out of landing.sections[type=rooms].content.rooms (no public /room-types
# endpoint), so the catalog has to be embedded here.
ROOMS_JSON=$(jq -n \
  --arg s "$RT_STUDIO" --arg t "$RT_TWIN" --arg k "$RT_KING" --arg f "$RT_SUITE" \
'[
  {id:$s, name:"Garden Studio",  description:"Quiet corner studio with a private balcony over the koi pond.",
   base_rate:"1800.00", base_currency:"THB", max_occupancy:2,
   image_url:"https://picsum.photos/seed/garden-studio/800/600",
   amenities:["Wi-Fi","Air-con","Mini fridge","Balcony"]},
  {id:$t, name:"Standard Twin",  description:"Twin beds, work desk, blackout curtains — for the practical traveller.",
   base_rate:"2200.00", base_currency:"THB", max_occupancy:2,
   image_url:"https://picsum.photos/seed/standard-twin/800/600",
   amenities:["Wi-Fi","Air-con","Work desk","Blackout curtains"]},
  {id:$k, name:"Deluxe King",    description:"King bed, rain shower, espresso machine, soaking tub.",
   base_rate:"3200.00", base_currency:"THB", max_occupancy:2,
   image_url:"https://picsum.photos/seed/deluxe-king/800/600",
   amenities:["Wi-Fi","Espresso machine","Rain shower","Soaking tub"]},
  {id:$f, name:"Family Suite",   description:"Two bedrooms, living area, kitchenette — sleeps four.",
   base_rate:"5400.00", base_currency:"THB", max_occupancy:4,
   image_url:"https://picsum.photos/seed/family-suite/800/600",
   amenities:["Wi-Fi","Kitchenette","Two bedrooms","Living area"]}
]')

# ----------------------------------------------------------------------------
# 3. Landing pages (TH + EN) — full set of FE-rendered sections
# ----------------------------------------------------------------------------

# Hero/gallery use picsum.photos seeded URLs so the same images render every
# run without depending on uploaded assets. <img> tags (not next/image),
# so no Next remotePatterns config required.
HERO_BG="https://picsum.photos/seed/sukhumvit-hero/1600/900"
GALLERY_JSON=$(jq -n '[
  {url:"https://picsum.photos/seed/lobby/1200/800",     alt:"Lobby"},
  {url:"https://picsum.photos/seed/pool/1200/800",      alt:"Pool"},
  {url:"https://picsum.photos/seed/breakfast/1200/800", alt:"Breakfast"},
  {url:"https://picsum.photos/seed/garden/1200/800",    alt:"Garden"},
  {url:"https://picsum.photos/seed/suite/1200/800",     alt:"Suite"},
  {url:"https://picsum.photos/seed/bar/1200/800",       alt:"Rooftop bar"}
]')

step "upsert + publish landing (th)"
LANDING_TH=$(jq -n \
  --argjson rooms "$ROOMS_JSON" \
  --argjson gallery "$GALLERY_JSON" \
  --arg hero "$HERO_BG" \
  --arg promptpay "$PROMPTPAY_ID" \
'{
  branding:{primary_color:"#0F766E", accent_color:"#F59E0B", font_family:"Inter"},
  sections:[
    {type:"hero", enabled:true, order:0, content:{
      headline:"พักผ่อนใจกลางสุขุมวิท",
      subheadline:"บูทีคโฮเทล 4 ชั้นใจกลางเมือง เดิน 5 นาทีถึง BTS",
      background_image_url:$hero,
      cta_label:"จองเลย"
    }},
    {type:"about", enabled:true, order:1, content:{
      title:"เรื่องราวของเรา",
      body:"เปิดให้บริการตั้งแต่ปี 2018 บนซอยเงียบในย่านสุขุมวิท ห้องพักทุกห้องตกแต่งด้วยไม้สักและผ้าทอท้องถิ่น เน้นการพักผ่อนแบบสงบใจกลางเมืองที่ไม่หยุดนิ่ง"
    }},
    {type:"gallery", enabled:true, order:2, content:{images:$gallery}},
    {type:"rooms",   enabled:true, order:3, content:{title:"ห้องพัก", rooms:$rooms}},
    {type:"amenities", enabled:true, order:4, content:{
      title:"สิ่งอำนวยความสะดวก",
      items:[
        {icon:"📶", label:"Wi-Fi ฟรีทุกพื้นที่"},
        {icon:"🏊", label:"สระว่ายน้ำชั้นดาดฟ้า"},
        {icon:"🍳", label:"อาหารเช้าแบบไทย-ตะวันตก"},
        {icon:"❄️", label:"ปรับอากาศทุกห้อง"},
        {icon:"🚗", label:"ที่จอดรถฟรี"},
        {icon:"🧺", label:"บริการซักรีด"},
        {icon:"🛎️", label:"แผนกต้อนรับ 24 ชม."},
        {icon:"🐾", label:"รับสัตว์เลี้ยงขนาดเล็ก"}
      ]
    }},
    {type:"location", enabled:true, order:5, content:{
      title:"ทำเล",
      address:"123 ซอยสุขุมวิท 31 แขวงคลองตันเหนือ เขตวัฒนา กรุงเทพฯ 10110",
      latitude:13.7382, longitude:100.5733,
      directions:"เดิน 5 นาทีจาก BTS พร้อมพงษ์ ทางออก 5 — มีรถรับส่งสนามบินสุวรรณภูมิตามนัด"
    }},
    {type:"reviews", enabled:true, order:6, content:{
      title:"รีวิวจากแขก",
      items:[
        {author:"ปิยะนุช ส.", rating:5, body:"ห้องเงียบมาก พนักงานน่ารักทุกคน อาหารเช้าอร่อย จะกลับมาอีกแน่นอน", created_at:"2026-04-12"},
        {author:"Anders L.",   rating:5, body:"Perfect base for Bangkok — quiet street, friendly staff, walkable to BTS.", created_at:"2026-03-21"},
        {author:"คุณวีรพล",   rating:4, body:"คุ้มราคา บรรยากาศดี ติดตรงที่จอดรถจำกัด แนะนำให้นั่ง BTS มา", created_at:"2026-02-08"}
      ]
    }},
    {type:"faq", enabled:true, order:7, content:{items:[
      {q:"เช็คอินกี่โมง?", a:"เช็คอิน 14:00 น. เช็คเอาท์ 12:00 น. ยินดีฝากกระเป๋าก่อนเช็คอินและหลังเช็คเอาท์"},
      {q:"มีรถรับส่งสนามบินไหม?", a:"มีบริการรถรับส่งสนามบินสุวรรณภูมิ 800 บาท/เที่ยว กรุณานัดล่วงหน้า 24 ชม."},
      {q:"ชำระเงินอย่างไร?", a:"PromptPay (QR), บัตรเครดิต Visa/Mastercard หรือเงินสดที่หน้าเคาน์เตอร์"}
    ]}},
    {type:"policies", enabled:true, order:8, content:{
      body:"ยกเลิกฟรีก่อนวันเข้าพัก 7 วัน • ยกเลิก 3-7 วันคิด 50% • ภายใน 72 ชม. ไม่คืนเงิน • ห้ามสูบบุหรี่ในห้องพัก (มีโซนสูบบุหรี่ที่ดาดฟ้า)"
    }},
    {type:"contact", enabled:true, order:9, content:{
      phone:"+66 2 123 4567",
      email:"hello@sukhumvit-garden.example.com",
      line_id:"@sukhumvitgarden"
    }}
  ],
  seo:{
    title:"The Sukhumvit Garden Boutique — บูทีคโฮเทลใจกลางกรุงเทพ",
    description:"บูทีคโฮเทล 20 ห้อง เดิน 5 นาทีถึง BTS พร้อมพงษ์ จองตรงราคาดีที่สุด"
  },
  tracking:{}
}')
call PUT "/v1/hotels/$HOTEL_ID/landing/th" "$LANDING_TH" >/dev/null
expect 200
call POST "/v1/hotels/$HOTEL_ID/landing/th/publish" "" >/dev/null
expect 200

step "upsert + publish landing (en)"
LANDING_EN=$(jq -n \
  --argjson rooms "$ROOMS_JSON" \
  --argjson gallery "$GALLERY_JSON" \
  --arg hero "$HERO_BG" \
'{
  branding:{primary_color:"#0F766E", accent_color:"#F59E0B", font_family:"Inter"},
  sections:[
    {type:"hero", enabled:true, order:0, content:{
      headline:"A garden in the middle of Sukhumvit",
      subheadline:"Four-storey boutique hotel, 5 minutes on foot from BTS.",
      background_image_url:$hero,
      cta_label:"Book direct"
    }},
    {type:"about", enabled:true, order:1, content:{
      title:"Our story",
      body:"Open since 2018 on a quiet Sukhumvit soi, each room is finished with teak and locally-woven textiles. A calm base inside one of the busiest neighbourhoods in Bangkok."
    }},
    {type:"gallery", enabled:true, order:2, content:{images:$gallery}},
    {type:"rooms",   enabled:true, order:3, content:{title:"Rooms", rooms:$rooms}},
    {type:"amenities", enabled:true, order:4, content:{
      title:"Amenities",
      items:[
        {icon:"📶", label:"Free Wi-Fi throughout"},
        {icon:"🏊", label:"Rooftop pool"},
        {icon:"🍳", label:"Thai + Western breakfast"},
        {icon:"❄️", label:"Air-conditioning"},
        {icon:"🚗", label:"Free parking"},
        {icon:"🧺", label:"Laundry service"},
        {icon:"🛎️", label:"24-hour reception"},
        {icon:"🐾", label:"Small pets welcome"}
      ]
    }},
    {type:"location", enabled:true, order:5, content:{
      title:"Location",
      address:"123 Sukhumvit Soi 31, Khlong Tan Nuea, Watthana, Bangkok 10110",
      latitude:13.7382, longitude:100.5733,
      directions:"5 minutes on foot from BTS Phrom Phong exit 5. Suvarnabhumi airport transfers by request."
    }},
    {type:"reviews", enabled:true, order:6, content:{
      title:"Guest reviews",
      items:[
        {author:"Piyanut S.", rating:5, body:"Very quiet rooms, lovely staff, great breakfast. Will be back.", created_at:"2026-04-12"},
        {author:"Anders L.",  rating:5, body:"Perfect base for Bangkok — quiet street, friendly staff, walkable to BTS.", created_at:"2026-03-21"},
        {author:"Weerapol K.",rating:4, body:"Good value, nice atmosphere. Parking is tight; take the BTS instead.", created_at:"2026-02-08"}
      ]
    }},
    {type:"faq", enabled:true, order:7, content:{items:[
      {q:"What time is check-in?", a:"Check-in from 14:00, check-out by 12:00. Luggage storage available before and after."},
      {q:"Do you offer airport transfers?", a:"Yes — Suvarnabhumi transfers at THB 800 each way, please book 24h in advance."},
      {q:"How can I pay?", a:"PromptPay (QR), Visa/Mastercard, or cash at the reception."}
    ]}},
    {type:"policies", enabled:true, order:8, content:{
      body:"Free cancellation up to 7 days before arrival • 50% fee for 3–7 days • Non-refundable within 72 hours • No smoking in rooms (designated rooftop area)."
    }},
    {type:"contact", enabled:true, order:9, content:{
      phone:"+66 2 123 4567",
      email:"hello@sukhumvit-garden.example.com",
      line_id:"@sukhumvitgarden"
    }}
  ],
  seo:{
    title:"The Sukhumvit Garden Boutique — Bangkok boutique hotel",
    description:"20-room boutique hotel, 5 minutes on foot from BTS Phrom Phong. Best rate guaranteed when you book direct."
  },
  tracking:{}
}')
call PUT "/v1/hotels/$HOTEL_ID/landing/en" "$LANDING_EN" >/dev/null
expect 200
call POST "/v1/hotels/$HOTEL_ID/landing/en/publish" "" >/dev/null
expect 200

# ----------------------------------------------------------------------------
# 4. Go live
# ----------------------------------------------------------------------------

step "go-live"
call POST "/v1/hotels/$HOTEL_ID/go-live" "" >/dev/null
expect 200
[[ "$(jq -r '.status' /tmp/seed_response.json)" == "live" ]] || { echo "expected status=live"; exit 1; }
green "  hotels.status = 'live'"

step "verify /v1/public/landing/$SLUG/th"
HTTP="$(curl -sS -o /dev/null -w '%{http_code}' "$API_URL/v1/public/landing/$SLUG/th")"
[[ "$HTTP" == "200" ]] || { echo "public landing returned $HTTP" >&2; exit 1; }
green "  200 OK"

# ----------------------------------------------------------------------------
# 5. Bookings — 12 reservations across every lifecycle state.
#
# We create everything via the staff walk-in endpoint (skips the "hotel must
# be live" check, lets us script bookings deterministically) and then drive
# the state machine with confirm / check-in / check-out / cancel / no-show.
#
# State machine reminder:
#   pending_payment → confirmed → checked_in → checked_out
#                  ↘ cancelled
#                      confirmed → cancelled
#                      confirmed → no_show
# ----------------------------------------------------------------------------

# seed_booking <room_type_id> <rooms> <check_in> <check_out> \
#              <name> <email> <phone> <country> <special> <final_state>
# final_state ∈ pending|confirmed|checked_in|checked_out|cancelled|no_show
# Echoes a one-line summary.
seed_booking() {
  local rt="$1" rooms="$2" ci="$3" co="$4" \
        name="$5" email="$6" phone="$7" country="$8" special="$9" final="${10}"
  local body resp id ref
  body=$(jq -n \
    --arg rt "$rt" --argjson rooms "$rooms" \
    --arg ci "$ci" --arg co "$co" \
    --arg name "$name" --arg email "$email" --arg phone "$phone" \
    --arg country "$country" --arg special "$special" \
    '{room_type_id:$rt, room_count:$rooms,
      check_in_date:$ci, check_out_date:$co,
      guest_name:$name, guest_email:$email, guest_phone:$phone,
      guest_country:$country, special_request:$special}')
  resp=$(call POST "/v1/hotels/$HOTEL_ID/bookings" "$body")
  expect 201
  id="$(jq -r '.id' /tmp/seed_response.json)"
  ref="$(jq -r '.reference' /tmp/seed_response.json)"

  case "$final" in
    pending)
      ;;
    confirmed)
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/confirm" "" >/dev/null
      expect 200
      ;;
    checked_in)
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/confirm"  "" >/dev/null; expect 200
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/check-in" "" >/dev/null; expect 200
      ;;
    checked_out)
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/confirm"   "" >/dev/null; expect 200
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/check-in"  "" >/dev/null; expect 200
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/check-out" "" >/dev/null; expect 200
      ;;
    cancelled)
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/confirm" "" >/dev/null; expect 200
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/cancel" '{"reason":"Guest requested cancellation"}' >/dev/null
      expect 200
      ;;
    no_show)
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/confirm" "" >/dev/null; expect 200
      call POST "/v1/hotels/$HOTEL_ID/bookings/$id/no-show" "" >/dev/null; expect 200
      ;;
    *)
      echo "seed_booking: unknown final state '$final'" >&2; exit 1;;
  esac
  sub "$(printf '%-12s %-7s %s → %s  %s' "$final" "$ref" "$ci" "$co" "$name")"
}

step "seed 12 bookings across all lifecycle states"

# Past — already checked out
seed_booking "$RT_KING"   1 "$(d -14)" "$(d -11)" \
  "Somchai Wattana"   "somchai@example.com"    "+66891112222" "TH" "" checked_out
seed_booking "$RT_TWIN"   1 "$(d -10)" "$(d  -7)" \
  "Emma Schmidt"      "emma.schmidt@example.com" "+4915112345678" "DE" "Late arrival, ~22:00" checked_out
seed_booking "$RT_STUDIO" 1 "$(d  -6)" "$(d  -4)" \
  "Hiroshi Tanaka"    "h.tanaka@example.com"     "+81901234567" "JP" "" checked_out

# Past — cancelled / no-show
seed_booking "$RT_SUITE"  1 "$(d  -5)" "$(d  -2)" \
  "Marie Dubois"      "marie.dubois@example.com" "+33612345678" "FR" "" cancelled
seed_booking "$RT_STUDIO" 1 "$(d  -3)" "$(d  -1)" \
  "Liam O'Connor"     "liam.oc@example.com"      "+353871234567" "IE" "" no_show

# Currently staying — checked in
seed_booking "$RT_KING"   1 "$(d  -1)" "$(d   2)" \
  "Apinya Boonsri"    "apinya.b@example.com"     "+66822334455" "TH" "ขอเตียงเสริม 1 ชุด" checked_in
seed_booking "$RT_SUITE"  1 "$(d   0)" "$(d   3)" \
  "Chen Wei"          "chen.wei@example.com"     "+8613812345678" "CN" "Anniversary trip" checked_in

# Upcoming — confirmed
seed_booking "$RT_STUDIO" 1 "$(d   2)" "$(d   5)" \
  "Olivia Martin"     "olivia.m@example.com"     "+447700900123" "GB" "" confirmed
seed_booking "$RT_TWIN"   2 "$(d   4)" "$(d   8)" \
  "Niran Phongphan"   "niran.p@example.com"      "+66866677788" "TH" "Quiet floor please" confirmed
seed_booking "$RT_KING"   1 "$(d   9)" "$(d  12)" \
  "Rafael Souza"      "r.souza@example.com"      "+5511987654321" "BR" "" confirmed

# Upcoming — still waiting on payment (pending_payment)
seed_booking "$RT_TWIN"   1 "$(d   3)" "$(d   6)" \
  "Sophie Laurent"    "sophie.l@example.com"     "+33687654321" "FR" "" pending
seed_booking "$RT_SUITE"  1 "$(d  14)" "$(d  18)" \
  "The Patel Family"  "patel.family@example.com" "+919812345678" "IN" "Two cots if possible" pending

# Sold-out window — corporate group books both Family Suites for the same
# stretch. Inventory for that room type is 2, so dates +20..+23 are full and
# the /book page should disable the submit button + show "Sold out".
seed_booking "$RT_SUITE"  2 "$(d  20)" "$(d  23)" \
  "TechCo Offsite"    "events@techco.example.com" "+66811223344" "TH" "Corporate retreat, both suites" confirmed

# ----------------------------------------------------------------------------
# 6. Summary
# ----------------------------------------------------------------------------

cat <<EOF

==============================================================================
DEMO READY  —  $HOTEL_NAME
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
  promptpay_id:  $PROMPTPAY_ID
  room types:
    Garden Studio   ($RT_STUDIO) — 1,800 THB · 6 rooms
    Standard Twin   ($RT_TWIN)   — 2,200 THB · 8 rooms
    Deluxe King     ($RT_KING)   — 3,200 THB · 4 rooms
    Family Suite    ($RT_SUITE)  — 5,400 THB · 2 rooms

Seeded bookings (13):
  3 checked_out (past)        — audit timeline + completed revenue
  1 cancelled, 1 no_show      — exception flows on the admin list
  2 checked_in (in-house now) — currently-staying guests
  4 confirmed (upcoming)      — incl. a 2-room group booking that fills
                                Family Suite for +20..+23 (sold-out window)
  2 pending_payment           — held seats waiting on PromptPay confirmation

Try the happy path:
  1. Log in to admin → /bookings shows all 13 reservations
  2. /bookings/calendar → coloured by state across the month
  3. Click any pending → tap "Mark paid" to flip it to confirmed
  4. Open the TH landing URL → "Book" → complete a reservation as a guest
  5. Sold-out demo: on /book pick Family Suite + dates inside +20..+23 → the
     Confirm button switches to "ห้องเต็มในช่วงวันที่เลือก" and is disabled
==============================================================================
EOF
