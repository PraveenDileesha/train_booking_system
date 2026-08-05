import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { TripResults } from './BookingResults';

function todayStr() {
  return new Date().toISOString().slice(0, 10);
}

export default function CustomerSearch() {
  const [stations, setStations] = useState([]);
  const [fromId, setFromId] = useState('');
  const [toId, setToId] = useState('');
  const [date, setDate] = useState(todayStr());
  const [error, setError] = useState(null);
  const [search, setSearch] = useState(null); // { fromId, toId, date } 

  useEffect(() => {
    api.listAllStations().then((data) => setStations(data.items)).catch((e) => setError(e.message));
  }, []);

  function handleSearch(e) {
    e.preventDefault();
    setError(null);
    if (!fromId || !toId || !date) {
      setError('Please select an origin, destination and date.');
      return;
    }
    if (fromId === toId) {
      setError('Origin and destination must be different.');
      return;
    }
    setSearch({ fromId, toId, date });
  }

  function resetToSearch() {
    setFromId('');
    setToId('');
    setDate(todayStr());
    setError(null);
    setSearch(null);
  }

  return (
    <div className="page-container">
      <header className="hero customer-hero">
        <h1>Sri Lanka Railways</h1>
        <p>Book your train journeys effortlessly.</p>
      </header>
      <main className="content" style={{ flexDirection: 'column', alignItems: 'center', gap: '1.5rem' }}>
        <div className="glass-card">
          <h2>Find a Seat</h2>
          <p style={{ marginBottom: '0.5rem', color: '#556' }}>
            Search available trains and reserve your journey.
          </p>
          <p style={{ marginBottom: '1rem', color: '#889', fontSize: '0.85rem' }}>
            Online booking closes 2 hours before departure. Visit the ticket counter after that.
          </p>

          {error && <div className="error-banner">{error}</div>}

          <form onSubmit={handleSearch}>
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
              <button className="btn btn-primary">Search trains</button>
            </div>
          </form>
        </div>

        {search && (
          <div style={{ width: '100%', maxWidth: '840px' }}>
            <TripResults fromId={search.fromId} toId={search.toId} date={search.date} onBookingComplete={resetToSearch} />
          </div>
        )}
      </main>
    </div>
  );
}
