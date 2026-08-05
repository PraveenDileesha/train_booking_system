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
      });
      setSelectedCoachIds([]);
      setFareOverrides({});
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
                        <button className="btn btn-danger btn-sm" onClick={() => handleDeleteTrip(t.id)}>
                          Delete
                        </button>
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
