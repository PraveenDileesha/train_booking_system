import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { AdminNav } from './AdminStations';

const CLASS_LABELS = { FIRST_AC: 'First Class AC', SECOND: 'Second Class', THIRD: 'Third Class' };
// Mirrors maxTicketsPerIssue in api/internal/handlers/counter.go.
const MAX_TICKETS = 5;

function todayStr() {
  return new Date().toISOString().slice(0, 10);
}

export default function CounterView() {
  const [stations, setStations] = useState([]);
  const [fromId, setFromId] = useState('');
  const [toId, setToId] = useState('');
  const [date, setDate] = useState(todayStr());
  const [trips, setTrips] = useState([]);
  const [tripId, setTripId] = useState('');
  const [tripFares, setTripFares] = useState([]); // unreserved fares for the chosen trip

  const [ticketClass, setTicketClass] = useState('');
  const [quantity, setQuantity] = useState(1);

  const [searching, setSearching] = useState(false);
  const [issuing, setIssuing] = useState(false);
  const [error, setError] = useState(null);
  const [receipt, setReceipt] = useState(null);

  useEffect(() => {
    api.listAllStations().then((data) => setStations(data.items)).catch((e) => setError(e.message));
  }, []);

  async function handleFindTrips(e) {
    e.preventDefault();
    if (!fromId || !toId || !date) {
      setError('Origin, destination and date are all required.');
      return;
    }
    if (fromId === toId) {
      setError('Origin and destination must be different.');
      return;
    }
    setSearching(true);
    setError(null);
    setTripId('');
    setReceipt(null);
    try {
      const data = await api.searchTrips(fromId, toId, date, { pageSize: 100 });
      setTrips(data.items);
    } catch (e) {
      setError(e.message);
    } finally {
      setSearching(false);
    }
  }

  useEffect(() => {
    setTripFares([]);
    setTicketClass('');
    if (!tripId) return;
    api
      .getTrip(tripId)
      .then((data) => setTripFares((data.fares || []).filter((f) => !f.is_reservable)))
      .catch((e) => setError(e.message));
  }, [tripId]);

  async function handleIssue(e) {
    e.preventDefault();
    if (!tripId || !ticketClass || quantity < 1) {
      setError('Trip, class and quantity are all required.');
      return;
    }
    if (quantity > MAX_TICKETS) {
      setError(`Cannot issue more than ${MAX_TICKETS} tickets at a time.`);
      return;
    }
    setIssuing(true);
    setError(null);
    try {
      const ticket = await api.issueUnreservedTicket({
        trip_id: Number(tripId),
        start_station_id: Number(fromId),
        end_station_id: Number(toId),
        class: ticketClass,
        quantity: Number(quantity),
      });
      setReceipt(ticket);
    } catch (e) {
      setError(e.message);
    } finally {
      setIssuing(false);
    }
  }

  function stationName(id) {
    const s = stations.find((s) => String(s.id) === String(id));
    return s ? s.name : '';
  }

  function resetForm() {
    setFromId('');
    setToId('');
    setDate(todayStr());
    setTrips([]);
    setTripId('');
    setTicketClass('');
    setQuantity(1);
    setReceipt(null);
  }

  return (
    <div className="page-container admin-bg">
      <AdminNav />
      <div className="admin-content">
        <h1 style={{ marginBottom: '1.5rem' }}>Ticket Counter Dashboard</h1>

        {error && <div className="error-banner">{error}</div>}

        {receipt ? (
          <div className="panel">
            <h2>Ticket issued</h2>
            <p>Reference <strong>UT-{receipt.id}</strong></p>
            <p>{CLASS_LABELS[receipt.class] || receipt.class} · {stationName(fromId)} → {stationName(toId)}</p>
            <p>{receipt.quantity} × Rs {receipt.fare_per_ticket} = <strong>Rs {receipt.total_fare}</strong></p>
            <button className="btn btn-primary" style={{ marginTop: '1rem' }} onClick={resetForm}>
              Issue another
            </button>
          </div>
        ) : (
          <>
            <div className="panel">
              <h2>Find a trip</h2>
              <p style={{ marginBottom: '1rem', color: '#889', fontSize: '0.85rem' }}>
                First-come-first-served: no seat assignment, no capacity limit.
              </p>
              <form onSubmit={handleFindTrips}>
                <div className="form-row">
                  <select value={fromId} onChange={(e) => setFromId(e.target.value)}>
                    <option value="">From…</option>
                    {stations.map((s) => (
                      <option key={s.id} value={s.id}>{s.name}</option>
                    ))}
                  </select>
                  <select value={toId} onChange={(e) => setToId(e.target.value)}>
                    <option value="">To…</option>
                    {stations.map((s) => (
                      <option key={s.id} value={s.id}>{s.name}</option>
                    ))}
                  </select>
                </div>
                <div className="form-row">
                  <input type="date" min={todayStr()} value={date} onChange={(e) => setDate(e.target.value)} />
                  <button className="btn btn-ghost" disabled={searching}>
                    {searching ? 'Finding…' : 'Find trips'}
                  </button>
                </div>
              </form>

              {trips.length > 0 && (
                <div className="form-row">
                  <select value={tripId} onChange={(e) => setTripId(e.target.value)}>
                    <option value="">Select a trip…</option>
                    {trips.map((t) => (
                      <option key={t.id} value={t.id}>{t.departure_time} → {t.arrival_time} · {t.route_name} · {t.status}</option>
                    ))}
                  </select>
                </div>
              )}
            </div>

            {tripId && (
              <div className="panel">
                <h2>Issue unreserved ticket</h2>
                <p style={{ marginBottom: '1rem', color: '#889', fontSize: '0.85rem' }}>
                  {stationName(fromId)} → {stationName(toId)}
                </p>
                <form onSubmit={handleIssue}>
                  <div className="form-row">
                    <select value={ticketClass} onChange={(e) => setTicketClass(e.target.value)}>
                      <option value="">Class…</option>
                      {tripFares.map((f) => (
                        <option key={f.class} value={f.class}>
                          {CLASS_LABELS[f.class] || f.class} (Rs {f.rate_per_km}/km)
                        </option>
                      ))}
                    </select>
                    <input
                      type="number"
                      min="1"
                      max={MAX_TICKETS}
                      value={quantity}
                      onChange={(e) => setQuantity(e.target.value)}
                      style={{ maxWidth: '100px' }}
                    />
                  </div>
                  <p style={{ color: '#889', fontSize: '0.78rem', marginTop: '-0.25rem', marginBottom: '0.75rem' }}>
                    Up to {MAX_TICKETS} tickets per issue.
                  </p>
                  {tripFares.length === 0 && (
                    <p className="empty-state">This trip has no unreserved coaches attached.</p>
                  )}
                  <button className="btn btn-primary" disabled={issuing || tripFares.length === 0}>
                    {issuing ? 'Issuing…' : 'Issue ticket'}
                  </button>
                </form>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
