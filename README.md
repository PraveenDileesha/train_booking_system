# Train Booking System

![Train crossing a viaduct on the Colombo Fort to Badulla line](docs/banner.png)

A segment-based seat booking system modeled on Sri Lanka Railways' Colombo Fort to Badulla upcountry line, using its real stations, distances and coach classes as the basis for the data in this project.

## Background

The line runs with 8 coaches, 3 reserved (every seat must be booked in advance) and 5 unreserved (first-come-first-served, no seat assignment).

Reserved coaches are frequently under-occupied for long stretches of the route since many passengers disembark partway through the journey, while the unreserved coaches are frequently overcrowded. Fares reflect this rigidity too. A passenger booking a reserved seat from, say, Colombo Fort to Kandy pays roughly double what a passenger takes for that same partial leg in an unreserved coach, the department's reasoning being that the fare has to cover the seat sitting empty for the rest of the journey (Kandy to Badulla) since under the current system that seat can't be resold once the train has departed.

Leadership believes there's revenue being left on the table and cost being passed unfairly to some passengers, but hasn't committed to a fix.

This system addresses that directly. A reserved seat is booked per segment rather than for the whole journey, so the same physical seat can be sold independently to a Colombo Fort to Kandy passenger and to a Kandy to Badulla passenger on the same trip. Two customers, two bookings, one seat, no overlap.

## Getting started

### Requirements

- Docker and Docker Compose

### Run it

1. Clone the repository.
2. Copy `.env.sample` to `.env` and fill in the values.
3. Start every service with a single command.

   ```
   docker compose up -d
   ```

That brings up Postgres, the Go API, pgAdmin, and the React frontend together. No local Go, Node, or pnpm installation is required.

| Service | URL |
|---|---|
| Frontend | http://localhost:3000 |
| API | http://localhost:8080 |
| pgAdmin | http://localhost:5050 |
| Postgres | localhost:5432 |

### Seeding demo data

The database starts empty. To populate it with a realistic dataset (every real station on the line, one route, 8 coaches, and a few days of scheduled trips) run this.

```
./scripts/seed_demo.sh
```

It seeds through the real admin API rather than raw SQL, so every row is created by the same validation and seat-generation logic a real admin request would hit. To wipe the database back to a clean slate, run `./scripts/reset_db.sh`.

## Available UI

| Page | Path | Purpose |
|---|---|---|
| Customer search and booking | `/` | Search trips between two stations on a date, view seat availability, hold and confirm reserved seats. |
| Ticket counter | `/counter` | Issue unreserved, first-come-first-served tickets for walk-up passengers. |
| Admin dashboard | `/admin` | Entry point to every admin screen. |
| Stations | `/admin/stations` | Add, view, and remove stations. |
| Routes | `/admin/routes` | List routes and open one for editing. |
| Route detail | `/admin/routes/:id` (or `/admin/routes/new`) | Build a route from stations with distances, reorder stops, and manage versions. |
| Coaches | `/admin/coaches` | Add coaches by class and auto-generate their seat layout. |
| Trips | `/admin/trips` | Schedule trips, attach coaches, and set per-class fares. |

## API

Base URL: `http://localhost:8080/api/v1`

### Conventions

**Authentication.** There is none. Every endpoint below, admin or public, is open to anyone who can reach the API. That's acceptable for this build's scope but is real future work before this could go anywhere near production.

**Pagination.** Every list endpoint returns the same envelope.

```json
{
  "items": [ ... ],
  "page": 1,
  "page_size": 20,
  "total": 86,
  "total_pages": 5
}
```

Controlled with `?page=` (default `1`) and `?page_size=` (default `20`, capped at `200`) query params.

**Rate limiting.** The public endpoints (everything under `/trips`, `/bookings`, and `/counter`) are limited to 5 requests per second per IP with bursts up to 20, enforced in-process per API instance. Admin endpoints are not rate limited. Exceeding the limit returns this response.

| Status | Body |
|---|---|
| `429 Too Many Requests` | `rate limit exceeded` |

**Errors.** Errors are plain text in the response body, not JSON. Beyond each endpoint's own validation, a shared constraint-violation mapping applies wherever a request hits the database.

| Status | Body | Trigger |
|---|---|---|
| `400 Bad Request` | `referenced record does not exist` | Foreign key violation, e.g. referencing a station ID that doesn't exist. |
| `400 Bad Request` | `invalid data: failed a validation rule` | Check constraint violation. |
| `409 Conflict` | `a record with this value already exists` | Unique constraint violation. |
| `409 Conflict` | `cannot delete: record is still referenced by other data` | Restrict violation, e.g. deleting a station still used by a route. |
| `409 Conflict` | `conflicts with an existing record` | Exclusion constraint violation. This is the seat double-booking collision path, see `POST /bookings`. |
| `409 Conflict` | `temporary conflict, please retry` | Deadlock detected. |

### Stations

#### `POST /admin/stations`

Creates a station.

```json
{ "name": "Kandy" }
```

Response `201 Created`.

```json
{ "id": 44, "name": "Kandy" }
```

| Status | Condition |
|---|---|
| `400` | `name` is empty. |
| `409` | A station with this name already exists. |

#### `GET /admin/stations`

Lists stations. Accepts `page`, `page_size`, and `?sort=name` (defaults to numeric ID order, `?sort=name` returns alphabetical order for picker or dropdown use).

Response `200 OK`, wrapped in the pagination envelope.

```json
{ "id": 1, "name": "Colombo Fort", "in_use": true }
```

`in_use` is `true` if any route references the station, which is what makes it non-deletable.

#### `GET /admin/stations/{id}`

Response `200 OK`, `{ "id": 1, "name": "Colombo Fort" }`

| Status | Condition |
|---|---|
| `404` | No station with this ID. |

#### `DELETE /admin/stations/{id}`

Response `204 No Content`.

| Status | Condition |
|---|---|
| `404` | No station with this ID. |
| `409` | The station is still referenced by a route. |

### Routes

#### `POST /admin/routes`

Creates a route, its first version (`1.0`), and that version's stations in order. Requires at least 2 stations.

```json
{
  "name": "Colombo Fort - Badulla Main Line",
  "stations": [
    { "station_id": 1, "distance_from_origin": 0 },
    { "station_id": 2, "distance_from_origin": 1.4 },
    { "station_id": 44, "distance_from_origin": 90.1 }
  ]
}
```

Response `201 Created`.

```json
{
  "route": { "id": 3, "name": "Colombo Fort - Badulla Main Line" },
  "version": { "id": 3, "route_id": 3, "version_no": "1.0", "created_at": "2026-08-05T10:00:00Z", "is_active": true },
  "stations": [
    { "id": 12, "route_version_id": 3, "station_id": 1, "station_name": "Colombo Fort", "stop_sequence": 0, "distance_from_origin": 0 }
  ]
}
```

| Status | Condition |
|---|---|
| `400` | `name` is empty, fewer than 2 stations, or a bad `distance_from_origin` value. |
| `400` | A `station_id` doesn't exist. |
| `409` | A route with this name already exists, or the same station appears twice in the list. |

#### `GET /admin/routes`

Lists routes. Response `200 OK`, wrapped in the pagination envelope, `{ "id": 3, "name": "Colombo Fort - Badulla Main Line" }`

#### `GET /admin/routes/{id}`

Returns a route with its currently active version and stations.

Response `200 OK` has the same shape as `POST /admin/routes`'s response.

| Status | Condition |
|---|---|
| `404` | No route with this ID. |

#### `DELETE /admin/routes/{id}`

Response `204 No Content`.

| Status | Condition |
|---|---|
| `404` | No route with this ID. |
| `409` | The route still has trips scheduled against it. |

#### `PUT /admin/routes/{id}/stations`

Publishes a new version of a route's stations. Nothing is mutated in place. This creates a new immutable `route_version`, makes it active, and deactivates whichever version was active before. The station list has the same shape as `POST /admin/routes` (minus `name`) and requires at least 2 stations.

Response `201 Created`.

```json
{
  "version": { "id": 4, "route_id": 3, "version_no": "1.1", "created_at": "2026-08-05T11:00:00Z", "is_active": true },
  "stations": [ ... ]
}
```

Note this response has no `route` key, unlike `POST /admin/routes`.

| Status | Condition |
|---|---|
| `400` | Fewer than 2 stations, or a bad `distance_from_origin` value. |
| `404` | The route has no active version. |

### Coaches

#### `POST /admin/coaches`

Creates a coach and auto-generates its seats from the class's row layout. Third class runs 5 seats across a row (A to E), First AC and Second class run 4 (A to D).

```json
{ "coach_name": "2nd Class Reserved A", "class": "SECOND", "is_reservable": true, "row_count": 8 }
```

Response `201 Created`.

```json
{
  "coach": { "id": 5, "coach_name": "2nd Class Reserved A", "is_reservable": true, "capacity": 32, "class": "SECOND", "row_count": 8 },
  "seats": [
    { "id": 101, "coach_id": 5, "seat_number": "1A" }
  ]
}
```

| Status | Condition |
|---|---|
| `400` | `coach_name` is empty, `class` isn't one of `FIRST_AC`, `SECOND`, `THIRD`, or `row_count` is less than 1. |

#### `GET /admin/coaches`

Lists coaches. Response `200 OK`, wrapped in the pagination envelope, `{ "id": 5, "coach_name": "2nd Class Reserved A", "class": "SECOND", "is_reservable": true, "row_count": 8, "capacity": 32 }`

#### `GET /admin/coaches/{id}`

Response `200 OK` has the same shape as `POST /admin/coaches`'s response.

| Status | Condition |
|---|---|
| `404` | No coach with this ID. |

#### `DELETE /admin/coaches/{id}`

Response `204 No Content`.

| Status | Condition |
|---|---|
| `404` | No coach with this ID. |
| `409` | The coach is still attached to a trip. |

### Trips

#### `POST /admin/trips`

Schedules a trip on the route's currently active version, attaches the given coaches, and records a fare rate per (class, is_reservable) combination.

```json
{
  "route_id": 3,
  "departure_date": "2026-08-10",
  "departure_time": "06:00",
  "arrival_date": "2026-08-10",
  "arrival_time": "14:00",
  "coach_ids": [1, 2, 3, 4, 5],
  "fares": [
    { "class": "FIRST_AC", "is_reservable": true, "rate_per_km": 15.0 },
    { "class": "THIRD", "is_reservable": false, "rate_per_km": 2.5 }
  ]
}
```

Response `201 Created`. The trip object comes back unwrapped, unlike the route and coach create responses above.

```json
{ "id": 12, "route_version_id": 3, "departure_date": "2026-08-10", "departure_time": "06:00", "arrival_date": "2026-08-10", "arrival_time": "14:00", "status": "SCHEDULED" }
```

| Status | Condition |
|---|---|
| `400` | Empty `coach_ids` or `fares`, a malformed date or time, an invalid fare `class`, or a non-positive `rate_per_km`. |
| `404` | The route has no active version. |

#### `GET /admin/trips`

Lists trips. Response `200 OK`, wrapped in the pagination envelope, `{ "id": 12, "route_version_id": 3, "departure_date": "2026-08-10", "departure_time": "06:00", "arrival_date": "2026-08-10", "arrival_time": "14:00", "status": "SCHEDULED", "route_id": 3, "route_name": "Colombo Fort - Badulla Main Line" }`

#### `DELETE /admin/trips/{id}`

Response `204 No Content`.

| Status | Condition |
|---|---|
| `404` | No trip with this ID. |
| `409` | The trip has a booking or unreserved ticket sold against it. |

#### `GET /trips`

Searches trips departing on a given date whose route passes through both stations in that order. Matches by station pair rather than by route ID since customers search by where they are going, not by which route serves them.

Query params are `start_station_id` (required), `end_station_id` (required), `date` (required, `YYYY-MM-DD`), plus `page` and `page_size`.

Response `200 OK`, wrapped in the pagination envelope, `{ "id": 12, "route_version_id": 3, "departure_date": "2026-08-10", "departure_time": "06:00", "arrival_date": "2026-08-10", "arrival_time": "14:00", "status": "SCHEDULED", "route_name": "Colombo Fort - Badulla Main Line" }`

| Status | Condition |
|---|---|
| `400` | `start_station_id`, `end_station_id`, or `date` is missing or malformed. |

#### `GET /trips/{id}`

Returns full detail for a trip, including its route's stations (with sequence and distance), attached coaches, and fare rates. Used by both the admin trip page and the customer booking flow.

Response `200 OK`.

```json
{
  "trip": { "id": 12, "route_version_id": 3, "departure_date": "2026-08-10", "departure_time": "06:00", "arrival_date": "2026-08-10", "arrival_time": "14:00", "status": "SCHEDULED" },
  "stations": [ { "id": 12, "route_version_id": 3, "station_id": 1, "station_name": "Colombo Fort", "stop_sequence": 0, "distance_from_origin": 0 } ],
  "coaches": [ { "id": 40, "trip_id": 12, "coach_id": 5, "position": 2, "coach_name": "2nd Class Reserved A", "class": "SECOND", "is_reservable": true, "row_count": 8, "capacity": 32 } ],
  "fares": [ { "id": 30, "trip_id": 12, "class": "SECOND", "is_reservable": true, "rate_per_km": 9.0 } ]
}
```

| Status | Condition |
|---|---|
| `404` | No trip with this ID. |

#### `GET /trips/{id}/seats`

Returns the seat map for a trip's reserved coaches, each seat annotated with whether it's free for the requested leg. Unreserved coaches never appear here, they have no per-seat inventory.

Query params are `start_station_id` (required) and `end_station_id` (required).

Response `200 OK`.

```json
{
  "seats": [
    { "trip_seat_id": 501, "seat_id": 101, "seat_number": "1A", "coach_id": 5, "coach_name": "2nd Class Reserved A", "class": "SECOND", "is_reservable": true, "coach_position": 2, "is_available": true }
  ],
  "fares": [ { "id": 30, "trip_id": 12, "class": "SECOND", "is_reservable": true, "rate_per_km": 9.0 } ],
  "distance_km": 90.1
}
```

| Status | Condition |
|---|---|
| `400` | `start_station_id` or `end_station_id` is missing, either isn't on this trip's route, or start doesn't come before end. |
| `404` | No trip with this ID. |

### Bookings

#### `POST /bookings`

Places a 5-minute hold on up to 5 reserved seats for one passenger on one leg, all in a single transaction. Each hold is blocked from overlapping any other pending or confirmed booking on that seat by a database exclusion constraint, so a race between two concurrent requests for the same seat and overlapping range is resolved atomically by Postgres, not by application-level locking. If any one seat in the request fails, the whole request fails and no seats are held.

```json
{
  "trip_seat_ids": [501, 502],
  "start_station_id": 1,
  "end_station_id": 44,
  "passenger": { "name": "Nimal Perera", "email": "nimal@example.com", "phone": "0771234567" }
}
```

`email` and `phone` are optional.

Response `201 Created`.

```json
{
  "bookings": [
    {
      "id": 900,
      "trip_seat_id": 501,
      "start_station_id": 1,
      "end_station_id": 44,
      "fare": 900,
      "status": "PENDING",
      "booking_reference": "BK1a2b3c4d",
      "held_until": "2026-08-05T10:05:00Z"
    }
  ]
}
```

| Status | Condition |
|---|---|
| `400` | Empty or duplicate `trip_seat_ids`, more than 5 seats, missing passenger `name`, a seat belongs to an unreserved coach, a station isn't on the route, or start doesn't come before end. |
| `404` | A `trip_seat_id` doesn't exist. |
| `409` | No fare is configured for a seat's class on this trip. |
| `409` | A seat is already held or booked for an overlapping segment (the exclusion constraint collision, see the shared error table above). |

#### `GET /bookings/{id}`

Response `200 OK` has the same shape as one entry in `POST /bookings`'s `bookings` array. `held_until` is omitted once the booking is no longer `PENDING`.

| Status | Condition |
|---|---|
| `404` | No booking with this ID. |

#### `POST /bookings/{id}/confirm`

Finalizes a hold. Empty request body.

Response `200 OK` uses the same shape as `GET /bookings/{id}`, but without `held_until`.

| Status | Condition |
|---|---|
| `409` | The hold already expired, or the booking was already confirmed or cancelled. The seat may since have been taken by someone else, the client should re-search. |

### Counter

#### `POST /counter/tickets`

Records a first-come-first-served unreserved ticket sale. There is no seat, no hold, and no coach-capacity check. Unreserved coaches have no per-seat inventory. The row exists purely for revenue tracking. Capped at 5 tickets per sale.

```json
{ "trip_id": 12, "start_station_id": 1, "end_station_id": 44, "class": "SECOND", "quantity": 2 }
```

Response `201 Created`.

```json
{
  "id": 77,
  "trip_id": 12,
  "start_station_id": 1,
  "end_station_id": 44,
  "class": "SECOND",
  "quantity": 2,
  "fare_per_ticket": 405,
  "total_fare": 810
}
```

| Status | Condition |
|---|---|
| `400` | Invalid `class`, `quantity` less than 1 or more than 5, a station isn't on the route, or start doesn't come before end. |
| `404` | No trip with this ID. |
| `409` | No unreserved fare is configured for this class on this trip. |
