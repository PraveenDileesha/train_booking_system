#!/bin/bash
# Seeds a demo dataset through the real admin API: stations along the Colombo Fort - Badulla upcountry line, one route connecting them, 8 coaches spanning every class and reservation combination, and 6 trips (three days, two time slots each) fully coached and fared.
# This is enough to exercise customer search and booking, counter unreserved-ticket sales, and every admin screen.
#
# Going through the API instead of raw SQL means every row is created by the same validation, seat-generation, and capacity logic a real admin request would hit, so seeded data can never drift out of sync with the schema.
#
# Usage: ./scripts/seed_demo.sh
# Requires: curl, jq, and a running API (docker compose up, or API_BASE_URL pointing at one).
set -euo pipefail

BASE_URL="${API_BASE_URL:-http://localhost:8080}/api/v1"
ADMIN="$BASE_URL/admin"

command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

echo "Seeding demo data against $BASE_URL ..."

DEMO_ROUTE_NAME="Colombo Fort - Badulla Main Line (Demo)"

existing_route_id="$(curl -sf "$ADMIN/routes?page_size=200" \
  | jq -r --arg name "$DEMO_ROUTE_NAME" '.items[] | select(.name == $name) | .id' | head -1)"
if [ -n "$existing_route_id" ]; then
  echo "Demo route already exists (id $existing_route_id) — nothing to do." >&2
  echo "Run ./scripts/reset_db.sh first if you want to reseed from scratch." >&2
  exit 0
fi

# ---------- Stations ----------
# The real Colombo Fort to Badulla upcountry line, every stop, with exact distance from Colombo Fort in km.
STATIONS=(
  "Colombo Fort:0.0"
  "Maradana:1.4"
  "Dematagoda:2.8"
  "Kelaniya:5.4"
  "Wanawasala:6.7"
  "Hunupitiya:7.8"
  "Enderamulla:9.1"
  "Horape:10.9"
  "Ragama:12.1"
  "Walpola:13.9"
  "Batuwatte:14.8"
  "Bulugahagoda:16.1"
  "Ganemulla:17.4"
  "Yagoda:18.7"
  "Gampaha:21.2"
  "Daraluwa:23.1"
  "Bemmulla:24.8"
  "Magelegoda:26.5"
  "Heendeniya:27.7"
  "Veyangoda:29.2"
  "Wandurawa:30.7"
  "Keenawala:32.4"
  "Pallewala:34.3"
  "Ganegoda:36.2"
  "Wijayarajadahana:38.2"
  "Mirigama:39.4"
  "Wilwatte:40.9"
  "Botale:42.5"
  "Ambeypussa:44.0"
  "Yattalgoda:46.6"
  "Buthgamuwa:48.1"
  "Alawwa:51.2"
  "Wlakubura:54.5"
  "Polgahawela:57.7"
  "Panaleeya:59.9"
  "Tismalpola:61.6"
  "Yatagama:63.6"
  "Rambukkana:65.9"
  "Kadigamuwa:69.8"
  "Gangoda:72.6"
  "Ihalakotte:74.1"
  "Balana:76.7"
  "Kadugannawa:79.7"
  "Pilimatalawa:82.6"
  "Kandy:90.1"
  "Sarasaviuyana:94.3"
  "Peradeniya:94.9"
  "Koshinna:97.1"
  "Gelioya:98.9"
  "Polgaha Anga:100.5"
  "Weligalla:101.3"
  "Gangathilaka:103.1"
  "Kahatapitiya:103.8"
  "Gampola:104.7"
  "Tembligala:107.4"
  "Ulapane:109.6"
  "Nawalapitiya:114.7"
  "Inguruoya:118.3"
  "Galaboda:121.5"
  "Dekinda:123.1"
  "Watawala:126.6"
  "Ihalawatawala:128.1"
  "Rosella:130.4"
  "Hatton:135.6"
  "Kotagala:138.9"
  "Talawakele:143.7"
  "Watagoda:146.4"
  "Great Western:149.8"
  "Radella:152.6"
  "Nanuoya:155.0"
  "Perakumpura:157.2"
  "Ambewela:164.0"
  "Pattipola:166.4"
  "Ohiya:170.0"
  "Idalgasinna:175.9"
  "Haputale:181.5"
  "Diyatalawa:184.5"
  "Bandarawela:188.0"
  "Kinigama:189.7"
  "Heel Oya:191.9"
  "Kital Ella:194.1"
  "Ella:195.4"
  "Demodara:198.2"
  "Uduwara:201.4"
  "Haliela:203.6"
  "Badulla:206.9"
)

declare -a STATION_IDS
declare -a STATION_DISTANCES

existing_stations="$(curl -sf "$ADMIN/stations?page_size=200")"

for entry in "${STATIONS[@]}"; do
  name="${entry%%:*}"
  distance="${entry##*:}"

  id="$(echo "$existing_stations" | jq -r --arg name "$name" '.items[] | select(.name == $name) | .id' | head -1)"

  if [ -z "$id" ]; then
    id="$(curl -sf -X POST "$ADMIN/stations" \
      -H 'Content-Type: application/json' \
      -d "$(jq -n --arg name "$name" '{name: $name}')" | jq -r '.id')"
    echo "  created station: $name (id $id)"
  else
    echo "  reusing station: $name (id $id)"
  fi

  STATION_IDS+=("$id")
  STATION_DISTANCES+=("$distance")
done

# ---------- Route ----------
stations_json="$(
  for i in "${!STATION_IDS[@]}"; do
    jq -n --argjson id "${STATION_IDS[$i]}" --argjson dist "${STATION_DISTANCES[$i]}" \
      '{station_id: $id, distance_from_origin: $dist}'
  done | jq -s '.'
)"

route_resp="$(curl -sf -X POST "$ADMIN/routes" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n --arg name "$DEMO_ROUTE_NAME" --argjson stations "$stations_json" \
    '{name: $name, stations: $stations}')")"
route_id="$(echo "$route_resp" | jq -r '.route.id')"
echo "created route: $DEMO_ROUTE_NAME (id $route_id)"

# ---------- Coaches ----------
# 8 coaches, 8 rows each: 3 reserved (one per class), 5 unreserved.
# name:class:is_reservable:row_count
COACHES=(
  "1st AC Saloon A:FIRST_AC:true:8"
  "2nd Class Reserved A:SECOND:true:8"
  "3rd Class Reserved A:THIRD:true:8"
  "2nd Class Unreserved A:SECOND:false:8"
  "2nd Class Unreserved B:SECOND:false:8"
  "3rd Class Unreserved A:THIRD:false:8"
  "3rd Class Unreserved B:THIRD:false:8"
  "3rd Class Unreserved C:THIRD:false:8"
)

declare -a COACH_IDS
for entry in "${COACHES[@]}"; do
  IFS=':' read -r name class reservable rows <<< "$entry"
  id="$(curl -sf -X POST "$ADMIN/coaches" \
    -H 'Content-Type: application/json' \
    -d "$(jq -n --arg name "$name" --arg class "$class" --argjson reservable "$reservable" --argjson rows "$rows" \
      '{coach_name: $name, class: $class, is_reservable: $reservable, row_count: $rows}')" \
    | jq -r '.coach.id')"
  echo "  created coach: $name (id $id)"
  COACH_IDS+=("$id")
done

coach_ids_json="$(printf '%s\n' "${COACH_IDS[@]}" | jq -R 'tonumber' | jq -s '.')"

# Fare rate (Rs/km) per (class, is_reservable), mirroring the admin UI's default ratios against a Rs 5/km third-class-unreserved base rate.
fares_json='[
  {"class":"FIRST_AC","is_reservable":true,"rate_per_km":15.0},
  {"class":"SECOND","is_reservable":true,"rate_per_km":9.0},
  {"class":"SECOND","is_reservable":false,"rate_per_km":4.5},
  {"class":"THIRD","is_reservable":true,"rate_per_km":5.0},
  {"class":"THIRD","is_reservable":false,"rate_per_km":2.5}
]'

# ---------- Trips ----------
# Three days (today, +1, +2), two time slots each, every coach attached to both so reserved-seat and unreserved-ticket flows always have inventory.
is_gnu_date=true
date -v+1d >/dev/null 2>&1 && is_gnu_date=false

offset_date() {
  local offset="$1"
  if $is_gnu_date; then
    date -d "+${offset} day" +%F
  else
    date -v+"${offset}"d +%F
  fi
}

SLOTS=("06:00:14:00" "14:30:22:30")

for day_offset in 0 1 2; do
  dep_date="$(offset_date "$day_offset")"
  for slot in "${SLOTS[@]}"; do
    IFS=':' read -r dep_h dep_m arr_h arr_m <<< "$slot"
    dep_time="${dep_h}:${dep_m}"
    arr_time="${arr_h}:${arr_m}"

    trip_id="$(curl -sf -X POST "$ADMIN/trips" \
      -H 'Content-Type: application/json' \
      -d "$(jq -n \
        --argjson route_id "$route_id" \
        --arg dep_date "$dep_date" \
        --arg dep_time "$dep_time" \
        --arg arr_date "$dep_date" \
        --arg arr_time "$arr_time" \
        --argjson coach_ids "$coach_ids_json" \
        --argjson fares "$fares_json" \
        '{route_id: $route_id, departure_date: $dep_date, departure_time: $dep_time,
          arrival_date: $arr_date, arrival_time: $arr_time,
          coach_ids: $coach_ids, fares: $fares}')" \
      | jq -r '.id')"
    echo "  created trip: $dep_date $dep_time -> $arr_time (id $trip_id)"
  done
done

echo "Done. Demo data ready at $BASE_URL."
