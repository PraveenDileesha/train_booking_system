import { useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import { Pagination } from './AdminStations';

const CLASS_LABELS = { FIRST_AC: 'First Class AC', SECOND: 'Second Class', THIRD: 'Third Class' };
const PAGE_SIZE = 10;
// Mirrors maxSeatsPerBooking in api/internal/handlers/bookings.go. The backend is the real enforcement point, this just keeps the UI from letting someone build a selection the API would reject anyway.
const MAX_SEATS = 5;

// 2+2 for First and Second, 3+2 for Third, mirroring the layout used when coaches generate their seats.
function seatColumns(cls) {
  return cls === 'THIRD' ? [['A', 'B', 'C'], ['D', 'E']] : [['A', 'B'], ['C', 'D']];
}

function parseSeat(seatNumber) {
  const match = seatNumber.match(/^(\d+)([A-Z])$/);
  if (!match) return { row: seatNumber, letter: '' };
  return { row: Number(match[1]), letter: match[2] };
}

function groupByCoach(seats) {
  const coaches = new Map();
  for (const s of seats) {
    if (!coaches.has(s.coach_id)) {
      coaches.set(s.coach_id, { coach_id: s.coach_id, coach_name: s.coach_name, class: s.class, seats: [] });
    }
    coaches.get(s.coach_id).seats.push(s);
  }
  return [...coaches.values()];
}

function groupByRow(seats) {
  const rows = new Map();
  for (const s of seats) {
    const { row, letter } = parseSeat(s.seat_number);
    if (!rows.has(row)) rows.set(row, new Map());
    rows.get(row).set(letter, s);
  }
  return [...rows.entries()].sort((a, b) => a[0] - b[0]);
}

// Mirrors roundToNearest5 in api/internal/handlers/bookings.go, so the price shown while picking a seat matches what the booking actually charges.
function fareForSeat(seat, fares, distanceKm) {
  const rate = fares.find((f) => f.class === seat.class && f.is_reservable);
  if (!rate) return null;
  return Math.round((distanceKm * Number(rate.rate_per_km)) / 5) * 5;
}

function SeatMap({ seatData, onToggle, selectedSeatIds }) {
  const coaches = useMemo(() => groupByCoach(seatData.seats), [seatData]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      {coaches.map((coach) => {
        const columns = seatColumns(coach.class);
        const rows = groupByRow(coach.seats);
        return (
          <div key={coach.coach_id}>
            <h4 style={{ fontSize: '0.85rem', color: '#556', marginBottom: '0.5rem' }}>
              {coach.coach_name}: {CLASS_LABELS[coach.class] || coach.class}
            </h4>
            <div className="seat-coach">
              {rows.map(([rowNo, seatsByLetter]) => (
                <div className="seat-row" key={rowNo}>
                  {columns.map((group, gi) => (
                    <div className="seat-group" key={gi}>
                      {group.map((letter) => {
                        const seat = seatsByLetter.get(letter);
                        if (!seat) return <div className="seat seat-empty" key={letter} />;
                        const fare = fareForSeat(seat, seatData.fares || [], seatData.distance_km);
                        const isSelected = selectedSeatIds.has(seat.trip_seat_id);
                        const cls = [
                          'seat',
                          seat.is_available ? 'seat-available' : 'seat-taken',
                          isSelected ? 'seat-selected' : '',
                        ].join(' ');
                        return (
                          <button
                            type="button"
                            key={letter}
                            className={cls}
                            disabled={!seat.is_available}
                            title={seat.is_available ? `Rs ${fare}` : 'Not available for this leg'}
                            onClick={() => onToggle(seat, fare)}
                          >
                            {seat.seat_number}
                          </button>
                        );
                      })}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          </div>
        );
      })}
      <div className="seat-legend">
        <span><i className="seat-dot seat-available" /> Available</span>
        <span><i className="seat-dot seat-taken" /> Taken for this leg</span>
        <span><i className="seat-dot seat-selected" /> Selected</span>
      </div>
    </div>
  );
}

// bookings holds the array of PENDING bookings returned by one createBooking call. Held together and confirmed together, since they were created in the same transaction and share a hold window.
function HoldPanel({ bookings, onConfirmed, onExpired }) {
  const [secondsLeft, setSecondsLeft] = useState(() =>
    Math.max(0, Math.round((new Date(bookings[0].held_until).getTime() - Date.now()) / 1000))
  );
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (secondsLeft <= 0) {
      onExpired();
      return;
    }
    const t = setTimeout(() => setSecondsLeft((s) => s - 1), 1000);
    return () => clearTimeout(t);
  }, [secondsLeft]);

  async function handleConfirm() {
    setConfirming(true);
    setError(null);
    try {
      const confirmed = await Promise.all(bookings.map((b) => api.confirmBooking(b.id)));
      onConfirmed(confirmed);
    } catch (e) {
      setError(e.message);
    } finally {
      setConfirming(false);
    }
  }

  const mm = String(Math.floor(secondsLeft / 60)).padStart(2, '0');
  const ss = String(secondsLeft % 60).padStart(2, '0');
  const totalFare = bookings.reduce((sum, b) => sum + Number(b.fare), 0);

  return (
    <div className="panel" style={{ borderColor: 'var(--accent)' }}>
      <h2>{bookings.length} seat{bookings.length > 1 ? 's' : ''} held: {mm}:{ss} remaining</h2>
      <p style={{ marginBottom: '1rem' }}>
        Total fare: <strong>Rs {totalFare}</strong>
        {' · '}Reference{bookings.length > 1 ? 's' : ''} {bookings.map((b) => b.booking_reference).join(', ')}
      </p>
      {error && <div className="error-banner">{error}</div>}
      <button className="btn btn-primary" onClick={handleConfirm} disabled={confirming}>
        {confirming ? 'Confirming…' : 'Confirm booking'}
      </button>
    </div>
  );
}

function BookingConfirmationModal({ bookings, onClose }) {
  const totalFare = bookings.reduce((sum, b) => sum + Number(b.fare), 0);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <h2>Booking confirmed</h2>
        {bookings.map((b) => (
          <div className="modal-receipt-row" key={b.id}>
            <span>Reference</span>
            <strong>{b.booking_reference}</strong>
          </div>
        ))}
        <div className="modal-receipt-total">
          <span>Total fare</span>
          <span>Rs {totalFare}</span>
        </div>
        <button className="btn btn-primary" style={{ marginTop: '1.5rem', width: '100%' }} onClick={onClose}>
          Close
        </button>
      </div>
    </div>
  );
}

function PassengerForm({ onSubmit, submitting, error }) {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (!name.trim()) return;
        onSubmit({ name: name.trim(), email: email.trim(), phone: phone.trim() });
      }}
    >
      {error && <div className="error-banner">{error}</div>}
      <div className="form-row">
        <input placeholder="Passenger name" value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      <div className="form-row">
        <input placeholder="Email (optional)" value={email} onChange={(e) => setEmail(e.target.value)} />
        <input placeholder="Phone (optional)" value={phone} onChange={(e) => setPhone(e.target.value)} />
      </div>
      <button className="btn btn-primary" disabled={submitting}>
        {submitting ? 'Holding seats…' : 'Hold selected seats'}
      </button>
    </form>
  );
}

// Renders inline below the search form on the same screen. No route change, so a new search just updates this section in place rather than navigating the customer to a different page.
export function TripResults({ fromId, toId, date, onBookingComplete }) {
  const [trips, setTrips] = useState([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [expandedTripId, setExpandedTripId] = useState(null);
  const [seatDataByTrip, setSeatDataByTrip] = useState({});
  const [seatLoading, setSeatLoading] = useState(false);

  // Selection is scoped to whichever trip is currently expanded. Picking seats on a different trip only makes sense one hold at a time.
  const [selectedTripId, setSelectedTripId] = useState(null);
  const [selectedSeats, setSelectedSeats] = useState([]); // [{ seat, fare }]
  const [seatLimitNotice, setSeatLimitNotice] = useState(null);
  const [bookings, setBookings] = useState(null); // PENDING bookings from API
  const [confirmedBookings, setConfirmedBookings] = useState(null);
  const [bookingError, setBookingError] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  function loadTrips(targetPage = 1) {
    if (!fromId || !toId || !date) return;
    setLoading(true);
    setError(null);
    api
      .searchTrips(fromId, toId, date, { page: targetPage, pageSize: PAGE_SIZE })
      .then((data) => {
        setTrips(data.items);
        setPage(data.page);
        setTotalPages(data.total_pages);
        setTotal(data.total);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }

  useEffect(() => { loadTrips(1); }, [fromId, toId, date]);

  function resetSelection() {
    setSelectedTripId(null);
    setSelectedSeats([]);
    setSeatLimitNotice(null);
    setBookings(null);
    setConfirmedBookings(null);
    setBookingError(null);
  }

  async function toggleTrip(tripId) {
    if (expandedTripId === tripId) {
      setExpandedTripId(null);
      return;
    }
    setExpandedTripId(tripId);
    resetSelection();
    if (seatDataByTrip[tripId]) return;

    setSeatLoading(true);
    try {
      const seatResp = await api.getTripSeats(tripId, fromId, toId);
      setSeatDataByTrip((prev) => ({ ...prev, [tripId]: seatResp }));
    } catch (e) {
      setError(e.message);
    } finally {
      setSeatLoading(false);
    }
  }

  function handleToggleSeat(tripId, seat, fare) {
    setBookings(null);
    setConfirmedBookings(null);
    setBookingError(null);
    setSeatLimitNotice(null);

    setSelectedSeats((prev) => {
      const inThisTrip = selectedTripId === tripId ? prev : [];
      const alreadySelected = inThisTrip.some((s) => s.seat.trip_seat_id === seat.trip_seat_id);
      if (alreadySelected) {
        return inThisTrip.filter((s) => s.seat.trip_seat_id !== seat.trip_seat_id);
      }
      if (inThisTrip.length >= MAX_SEATS) {
        setSeatLimitNotice(`You can select up to ${MAX_SEATS} seats per booking.`);
        return inThisTrip;
      }
      return [...inThisTrip, { seat, fare }];
    });
    setSelectedTripId(tripId);
  }

  async function handleHold(passenger) {
    setSubmitting(true);
    setBookingError(null);
    try {
      const created = await api.createBooking({
        trip_seat_ids: selectedSeats.map((s) => s.seat.trip_seat_id),
        start_station_id: Number(fromId),
        end_station_id: Number(toId),
        passenger,
      });
      setBookings(created.bookings);
      // Every held seat should now show as taken on this trip's seat map.
      const heldSeatIds = new Set(selectedSeats.map((s) => s.seat.trip_seat_id));
      setSeatDataByTrip((prev) => {
        const data = prev[selectedTripId];
        if (!data) return prev;
        return {
          ...prev,
          [selectedTripId]: {
            ...data,
            seats: data.seats.map((s) =>
              heldSeatIds.has(s.trip_seat_id) ? { ...s, is_available: false } : s
            ),
          },
        };
      });
    } catch (e) {
      setBookingError(e.message);
    } finally {
      setSubmitting(false);
    }
  }

  function handleExpired() {
    setBookings(null);
    setBookingError('Your hold expired. Please select seats again.');
    setSelectedSeats([]);
  }

  const selectedSeatIds = useMemo(
    () => new Set(selectedSeats.map((s) => s.seat.trip_seat_id)),
    [selectedSeats]
  );
  const selectedTotalFare = selectedSeats.reduce((sum, s) => sum + Number(s.fare || 0), 0);

  return (
    <div style={{ marginTop: '1.5rem' }}>
      <h3 style={{ marginBottom: '1rem', color: 'var(--primary)' }}>Available trains · {date}</h3>

      {error && <div className="error-banner">{error}</div>}

      {confirmedBookings && (
        <BookingConfirmationModal bookings={confirmedBookings} onClose={onBookingComplete} />
      )}

      {loading ? (
        <p className="empty-state">Searching…</p>
      ) : trips.length === 0 ? (
        <p className="empty-state">No trains found for this route and date.</p>
      ) : (
        <>
        {trips.map((trip) => (
          <div className="panel" key={trip.id}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <strong>{trip.departure_time} → {trip.arrival_time}</strong> · {trip.route_name} · {trip.status}
              </div>
              <button className="btn btn-ghost btn-sm" onClick={() => toggleTrip(trip.id)}>
                {expandedTripId === trip.id ? 'Hide seats' : 'View seats'}
              </button>
            </div>

            {expandedTripId === trip.id && (
              <div style={{ marginTop: '1rem' }}>
                {seatLoading && !seatDataByTrip[trip.id] ? (
                  <p className="empty-state">Loading seat map…</p>
                ) : seatDataByTrip[trip.id] ? (
                  seatDataByTrip[trip.id].seats.length === 0 ? (
                    <p className="empty-state">No reserved coaches on this trip.</p>
                  ) : (
                    <SeatMap
                      seatData={seatDataByTrip[trip.id]}
                      selectedSeatIds={selectedTripId === trip.id ? selectedSeatIds : new Set()}
                      onToggle={(seat, fare) => handleToggleSeat(trip.id, seat, fare)}
                    />
                  )
                ) : null}

                {selectedTripId === trip.id && seatLimitNotice && (
                  <p style={{ color: '#c0392b', fontSize: '0.85rem', marginTop: '0.75rem' }}>{seatLimitNotice}</p>
                )}

                {selectedTripId === trip.id && selectedSeats.length > 0 && !bookings && !confirmedBookings && (
                  <div style={{ marginTop: '1rem' }}>
                    <p style={{ marginBottom: '0.5rem' }}>
                      Seat{selectedSeats.length > 1 ? 's' : ''}{' '}
                      {selectedSeats.map((s) => s.seat.seat_number).join(', ')} · Rs {selectedTotalFare}
                      {' '}({selectedSeats.length}/{MAX_SEATS} selected)
                    </p>
                    <PassengerForm onSubmit={handleHold} submitting={submitting} error={bookingError} />
                  </div>
                )}

                {selectedTripId === trip.id && bookings && !confirmedBookings && (
                  <div style={{ marginTop: '1rem' }}>
                    <HoldPanel
                      bookings={bookings}
                      onConfirmed={(b) => setConfirmedBookings(b)}
                      onExpired={handleExpired}
                    />
                  </div>
                )}
              </div>
            )}
          </div>
        ))}
        <Pagination page={page} totalPages={totalPages} total={total} onChange={loadTrips} />
        </>
      )}
    </div>
  );
}
