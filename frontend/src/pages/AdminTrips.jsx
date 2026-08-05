import { useEffect, useMemo, useState } from 'react';
import { api } from '../api/client';
import { AdminNav, PageHeading, Pagination } from './AdminStations';

const CLASS_LABELS = { FIRST_AC: 'First AC', SECOND: 'Second', THIRD: 'Third' };
const DEFAULT_RATIO = { FIRST_AC: 3.0, SECOND: 1.8, THIRD: 1.0 };
const PAGE_SIZE = 20;

export default function AdminTrips() {
  const [trips, setTrips] = useState([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [routes, setRoutes] = useState([]);
  const [coaches, setCoaches] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [creating, setCreating] = useState(false);

  const [routeId, setRouteId] = useState('');
  const [date, setDate] = useState('');
  const [time, setTime] = useState('08:30');
  const [arrivalDate, setArrivalDate] = useState('');
  const [arrivalTime, setArrivalTime] = useState('16:30');
  const [selectedCoachIds, setSelectedCoachIds] = useState([]);
  const [baseRate, setBaseRate] = useState(5);
  const [routeStations, setRouteStations] = useState([]);
  const [stopTimes, setStopTimes] = useState({});

  // Trips get a real paginated table. Routes and coaches here are just dropdown and checkbox source data for the create-trip form, so they're fetched in full rather than paginated.
  async function loadTrips(targetPage = page) {
    setLoading(true);
    setError(null);
    try {
      const tripData = await api.listTrips({ page: targetPage, pageSize: PAGE_SIZE });
      setTrips(tripData.items);
      setPage(tripData.page);
      setTotalPages(tripData.total_pages);
      setTotal(tripData.total);
    } catch (e) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  async function loadFormData() {
    try {
      const [routeData, coachData] = await Promise.all([api.listAllRoutes(), api.listAllCoaches()]);
      setRoutes(routeData.items);
      setCoaches(coachData.items);
    } catch (e) {
      setError(e.message);
    }
  }

  useEffect(() => { loadTrips(1); loadFormData(); }, []);

  // Most journeys arrive the same day they depart. Default the arrival date to match, but leave it editable for overnight services.
  useEffect(() => {
    if (date) setArrivalDate((prev) => prev || date);
  }, [date]);

  // Intermediate stops need their own arrival and departure time, fetched fresh whenever the selected route changes.
  useEffect(() => {
    setStopTimes({});
    if (!routeId) {
      setRouteStations([]);
      return;
    }
    api.getRoute(routeId).then((data) => setRouteStations(data.stations || [])).catch((e) => setError(e.message));
  }, [routeId]);

  const intermediateStops = useMemo(() => routeStations.slice(1, -1), [routeStations]);

  function setStopTime(stationId, field, value) {
    setStopTimes((prev) => ({ ...prev, [stationId]: { ...prev[stationId], [field]: value } }));
  }

  // Mirrors nextInstant in api/internal/handlers/trips.go. Combines hhmm with the same calendar day as after, rolling to the next day if that would land before after.
  function nextInstant(after, hhmm) {
    const [h, m] = hhmm.split(':').map(Number);
    const candidate = new Date(after.getFullYear(), after.getMonth(), after.getDate(), h, m, 0, 0);
    if (candidate < after) candidate.setDate(candidate.getDate() + 1);
    return candidate;
  }

  // Walks the intermediate stops in route order the same way the backend does, so an out-of-order stop is caught before submit rather than after a 400.
  function validateStopTimes() {
    if (intermediateStops.length === 0) return null;
    let cursor = new Date(`${date}T${time}`);
    const tripArrival = new Date(`${arrivalDate}T${arrivalTime}`);
    for (const stop of intermediateStops) {
      const times = stopTimes[stop.id];
      if (!times || !times.arrival || !times.departure) {
        return `${stop.station_name} is missing an arrival or departure time.`;
      }
      const arrival = nextInstant(cursor, times.arrival);
      const departure = nextInstant(arrival, times.departure);
      cursor = departure;
    }
    if (cursor > tripArrival) {
      return "Stop times run past the trip's arrival time.";
    }
    return null;
  }

  function toggleCoach(id) {
    setSelectedCoachIds((prev) =>
      prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]
    );
  }

  // Every distinct (class, is_reservable) combination among the chosen coaches needs its own admin-entered rate for this trip.
  const fareSlots = useMemo(() => {
    const selected = coaches.filter((c) => selectedCoachIds.includes(c.id));
    const seen = new Map();
    for (const c of selected) {
      const key = `${c.class}:${c.is_reservable}`;
      if (!seen.has(key)) seen.set(key, { class: c.class, is_reservable: c.is_reservable });
    }
    return [...seen.values()];
  }, [coaches, selectedCoachIds]);

  const [fareOverrides, setFareOverrides] = useState({});

  function fareFor(slot) {
    const key = `${slot.class}:${slot.is_reservable}`;
    if (fareOverrides[key] !== undefined) return fareOverrides[key];
    const ratio = slot.is_reservable ? DEFAULT_RATIO[slot.class] ?? 1 : (DEFAULT_RATIO[slot.class] ?? 1) / 2;
    return Number((baseRate * ratio).toFixed(2));
  }

  function setFare(slot, value) {
    const key = `${slot.class}:${slot.is_reservable}`;
    setFareOverrides((prev) => ({ ...prev, [key]: value }));
  }

  async function handleCreate(e) {
    e.preventDefault();
    if (!routeId || !date || !time || !arrivalDate || !arrivalTime || selectedCoachIds.length === 0) {
      setError('Route, departure, arrival and at least one coach are required.');
      return;
    }
    const stopTimesError = validateStopTimes();
    if (stopTimesError) {
      setError(stopTimesError);
      return;
    }
    setCreating(true);
    setError(null);
    try {
      await api.createTrip({
        route_id: Number(routeId),
        departure_date: date,
        departure_time: time,
        arrival_date: arrivalDate,
        arrival_time: arrivalTime,
        coach_ids: selectedCoachIds,
        fares: fareSlots.map((slot) => ({
          class: slot.class,
          is_reservable: slot.is_reservable,
          rate_per_km: Number(fareFor(slot)),
        })),
        stops: intermediateStops.map((stop) => ({
          route_station_id: stop.id,
          arrival_time: stopTimes[stop.id].arrival,
          departure_time: stopTimes[stop.id].departure,
        })),
      });
      setSelectedCoachIds([]);
      setFareOverrides({});
      setStopTimes({});
      setArrivalDate('');
      await loadTrips(1);
    } catch (e) {
      setError(e.message);
    } finally {
      setCreating(false);
    }
  }

  async function handleDeleteTrip(id) {
    if (!confirm('Delete this trip? This only works if no bookings or tickets have been issued for it.')) return;
    setError(null);
    try {
      await api.deleteTrip(id);
      const isLastRowOnPage = trips.length === 1 && page > 1;
      await loadTrips(isLastRowOnPage ? page - 1 : page);
    } catch (e) {
      setError(e.message);
    }
  }

  return (
    <div className="page-container">
      <AdminNav />
      <div className="admin-content">
        <PageHeading title="Trips" />

        {error && <div className="error-banner">{error}</div>}

        <div className="panel">
          <h2>Schedule a trip</h2>
          <form onSubmit={handleCreate}>
            <div className="form-row">
              <select value={routeId} onChange={(e) => setRouteId(e.target.value)}>
                <option value="">Select a route…</option>
                {routes.map((r) => (
                  <option key={r.id} value={r.id}>{r.name}</option>
                ))}
              </select>
            </div>
            <div className="form-row">
              <label style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', fontSize: '0.78rem', color: '#889', flex: 1 }}>
                Departs
                <span style={{ display: 'flex', gap: '0.5rem' }}>
                  <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
                  <input type="time" value={time} onChange={(e) => setTime(e.target.value)} />
                </span>
              </label>
              <label style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', fontSize: '0.78rem', color: '#889', flex: 1 }}>
                Arrives
                <span style={{ display: 'flex', gap: '0.5rem' }}>
                  <input type="date" value={arrivalDate} onChange={(e) => setArrivalDate(e.target.value)} />
                  <input type="time" value={arrivalTime} onChange={(e) => setArrivalTime(e.target.value)} />
                </span>
              </label>
            </div>

            {intermediateStops.length > 0 && (
              <>
                <h3 style={{ fontSize: '0.9rem', margin: '1rem 0 0.5rem' }}>Stop times</h3>
                <table className="admin-table">
                  <thead>
                    <tr><th>Station</th><th>Distance (km)</th><th>Arrival</th><th>Departure</th></tr>
                  </thead>
                  <tbody>
                    {intermediateStops.map((stop) => (
                      <tr key={stop.id}>
                        <td>{stop.station_name}</td>
                        <td>{stop.distance_from_origin}</td>
                        <td>
                          <input
                            type="time"
                            value={stopTimes[stop.id]?.arrival || ''}
                            onChange={(e) => setStopTime(stop.id, 'arrival', e.target.value)}
                          />
                        </td>
                        <td>
                          <input
                            type="time"
                            value={stopTimes[stop.id]?.departure || ''}
                            onChange={(e) => setStopTime(stop.id, 'departure', e.target.value)}
                          />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}

            <h3 style={{ fontSize: '0.9rem', margin: '1rem 0 0.5rem' }}>Attach coaches</h3>
            {coaches.length === 0 ? (
              <p className="empty-state">No coaches yet. Add some on the Coaches page first.</p>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
                {coaches.map((c) => (
                  <label key={c.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <input
                      type="checkbox"
                      style={{ width: 'auto' }}
                      checked={selectedCoachIds.includes(c.id)}
                      onChange={() => toggleCoach(c.id)}
                    />
                    {c.coach_name}: {CLASS_LABELS[c.class] || c.class}, {c.is_reservable ? 'Reserved' : 'Unreserved'} ({c.capacity} seats)
                  </label>
                ))}
              </div>
            )}

            {fareSlots.length > 0 && (
              <>
                <h3 style={{ fontSize: '0.9rem', margin: '1rem 0 0.5rem' }}>
                  Fares (Rs / km): base rate below, class ratios editable per row
                </h3>
                <div className="form-row" style={{ maxWidth: '220px' }}>
                  <label style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', fontSize: '0.8rem', color: '#889' }}>
                    Third-class unreserved base rate
                    <input
                      type="number"
                      step="0.5"
                      value={baseRate}
                      onChange={(e) => setBaseRate(Number(e.target.value))}
                    />
                  </label>
                </div>
                <table className="admin-table">
                  <thead>
                    <tr><th>Class</th><th>Type</th><th>Rate / km (Rs)</th></tr>
                  </thead>
                  <tbody>
                    {fareSlots.map((slot) => {
                      const key = `${slot.class}:${slot.is_reservable}`;
                      return (
                        <tr key={key}>
                          <td>{CLASS_LABELS[slot.class] || slot.class}</td>
                          <td>{slot.is_reservable ? 'Reserved' : 'Unreserved'}</td>
                          <td>
                            <input
                              type="number"
                              step="0.5"
                              value={fareFor(slot)}
                              onChange={(e) => setFare(slot, e.target.value)}
                              style={{ width: '120px', padding: '0.3rem 0.5rem' }}
                            />
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </>
            )}

            <div style={{ marginTop: '1rem' }}>
              <button className="btn btn-primary" disabled={creating}>
                {creating ? 'Scheduling…' : 'Schedule trip'}
              </button>
            </div>
          </form>
        </div>

        <div className="panel">
          <h2>All trips</h2>
          {loading ? (
            <p className="empty-state">Loading…</p>
          ) : trips.length === 0 ? (
            <p className="empty-state">No trips scheduled yet.</p>
          ) : (
            <>
              <table className="admin-table">
                <thead>
                  <tr><th>ID</th><th>Route</th><th>Departs</th><th>Arrives</th><th>Status</th><th></th></tr>
                </thead>
                <tbody>
                  {trips.map((t) => (
                    <tr key={t.id}>
                      <td>{t.id}</td>
                      <td>{t.route_name}</td>
                      <td>{t.departure_date} {t.departure_time}</td>
                      <td>{t.arrival_date} {t.arrival_time}</td>
                      <td>{t.status}</td>
                      <td style={{ textAlign: 'right' }}>
                        {!t.has_activity && (
                          <button className="btn btn-danger btn-sm" onClick={() => handleDeleteTrip(t.id)}>
                            Delete
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <Pagination page={page} totalPages={totalPages} total={total} onChange={loadTrips} />
            </>
          )}
        </div>
      </div>
    </div>
  );
}
