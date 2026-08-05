import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { api } from '../api/client';
import { AdminNav, PageHeading } from './AdminStations';

let nextStopId = 0;
const emptyStop = () => ({ id: nextStopId++, station_id: '', distance_from_origin: '' });

// Stops don't carry a manual position. Order is always derived from distance_from_origin, so departure (smallest distance) always leads and arrival (largest) always trails.
// A stop with no distance yet sorts last so it doesn't jump around mid-edit.
function sortByDistance(stops) {
  const distanceOf = (s) => (s.distance_from_origin === '' || isNaN(Number(s.distance_from_origin))
    ? Infinity
    : Number(s.distance_from_origin));
  return [...stops].sort((a, b) => distanceOf(a) - distanceOf(b));
}

export default function AdminRouteDetail() {
  const { id } = useParams();
  const isNew = id === undefined;
  const navigate = useNavigate();

  const [allStations, setAllStations] = useState([]);
  const [name, setName] = useState('');
  const [versionInfo, setVersionInfo] = useState(null); // { version_no, is_active } or null for new
  const [stops, setStops] = useState([emptyStop(), emptyStop()]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const stationList = await api.listAllStations();
        setAllStations(stationList.items);

        if (!isNew) {
          const data = await api.getRoute(id);
          setName(data.route.name);
          setVersionInfo(data.version);
          if (data.stations && data.stations.length > 0) {
            setStops(
              data.stations.map((s) => ({
                id: nextStopId++,
                station_id: String(s.station_id),
                distance_from_origin: String(s.distance_from_origin),
              }))
            );
          }
        }
      } catch (e) {
        setError(e.message);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [id, isNew]);

  function stationName(stationId) {
    const s = allStations.find((s) => String(s.id) === String(stationId));
    return s ? s.name : '';
  }

  function updateStop(stopId, field, value) {
    setStops((prev) => prev.map((s) => (s.id === stopId ? { ...s, [field]: value } : s)));
  }

  function addStop() {
    setStops((prev) => [...prev, emptyStop()]);
  }

  function removeStop(stopId) {
    setStops((prev) => prev.filter((s) => s.id !== stopId));
  }

  const sortedStops = useMemo(() => sortByDistance(stops), [stops]);

  // The departure stop is always km 0 by definition. Force it rather than let it drift, including when a station added earlier gets edited into or out of first place.
  useEffect(() => {
    if (sortedStops.length === 0) return;
    const origin = sortedStops[0];
    if (origin.distance_from_origin === '0') return; // already correct, skip to avoid a render loop
    setStops((prev) =>
      prev.map((s) => (s.id === origin.id ? { ...s, distance_from_origin: '0' } : s))
    );
  }, [sortedStops]);

  function validate() {
    if (!isNew && stops.length < 2) return 'A route needs at least 2 stations.';
    if (isNew && (!name.trim() || stops.length < 2)) {
      return 'Give the route a name and at least 2 stations.';
    }
    for (const s of stops) {
      if (!s.station_id) return 'Every stop needs a station selected.';
      if (s.distance_from_origin === '' || isNaN(Number(s.distance_from_origin))) {
        return 'Every stop needs a valid distance from origin.';
      }
    }
    const ids = stops.map((s) => s.station_id);
    if (new Set(ids).size !== ids.length) return 'The same station cannot appear twice in one route.';
    for (let i = 1; i < sortedStops.length; i++) {
      const prevDistance = Number(sortedStops[i - 1].distance_from_origin);
      const curDistance = Number(sortedStops[i].distance_from_origin);
      if (curDistance <= prevDistance) {
        const label = stationName(sortedStops[i].station_id) || `Stop ${i + 1}`;
        return `"${label}"'s distance (${curDistance} km) must be greater than the stop before it (${prevDistance} km).`;
      }
    }
    return null;
  }

  async function handleSave(e) {
    e.preventDefault();
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }
    setSaving(true);
    setError(null);
    // sortedStops is already departure-to-arrival order. That order becomes each stop's stop_sequence on the backend.
    const payload = sortedStops.map((s) => ({
      station_id: Number(s.station_id),
      distance_from_origin: Number(s.distance_from_origin),
    }));
    try {
      if (isNew) {
        const created = await api.createRoute(name.trim(), payload);
        navigate(`/admin/routes/${created.route.id}`, { replace: true });
      } else {
        await api.updateRouteStations(id, payload);
        navigate(0); // reload to show the new version
      }
    } catch (e) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  }

  async function handleDeleteRoute() {
    if (!confirm('Delete this route? This cannot be undone.')) return;
    setError(null);
    try {
      await api.deleteRoute(id);
      navigate('/admin/routes', { replace: true });
    } catch (e) {
      setError(e.message);
    }
  }

  if (loading) {
    return (
      <div className="page-container">
        <AdminNav />
        <div className="admin-content"><p className="empty-state">Loading…</p></div>
      </div>
    );
  }

  // Build track diagram positions from current stops. Only meaningful once distances are numbers.
  const trackStops = sortedStops
    .filter((s) => s.station_id && s.distance_from_origin !== '')
    .map((s) => ({ ...s, distance: Number(s.distance_from_origin) }));
  const maxDistance = Math.max(1, ...trackStops.map((s) => s.distance));

  return (
    <div className="page-container">
      <AdminNav />
      <div className="admin-content">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <PageHeading
            title={isNew ? 'New route' : name}
            subtitle={!isNew && versionInfo ? `Current version ${versionInfo.version_no}` : null}
          />
          {!isNew && (
            <button className="btn btn-danger" onClick={handleDeleteRoute}>Delete route</button>
          )}
        </div>

        {error && <div className="error-banner">{error}</div>}

        {isNew && (
          <div className="panel">
            <h2>Route name</h2>
            <div className="form-row">
              <input
                placeholder="e.g. Colombo Fort - Badulla"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
          </div>
        )}

        {trackStops.length >= 2 && (
          <div className="panel">
            <h2>Route preview</h2>
            {/* Segment-distance labels between every pair of dots only stay
                legible up to a moderate stop count — past that they're just
                overlapping clutter, so drop them and let the station labels
                and overall span speak for themselves. */}
            {trackStops.length > 20 && (
              <p style={{ color: '#889', fontSize: '0.8rem', marginTop: '-0.5rem', marginBottom: '0.75rem' }}>
                {trackStops.length} stops over {maxDistance.toFixed(1)} km. Scroll to see the full line.
              </p>
            )}
            <div className="track">
              <div className="track-line" style={{ minWidth: `${Math.max(480, trackStops.length * 46)}px` }}>
                {trackStops.map((s, i) => (
                  <div
                    key={s.id}
                    className={[
                      'track-stop',
                      i === 0 ? 'origin' : '',
                      i === trackStops.length - 1 ? 'destination' : '',
                    ].join(' ').trim()}
                    style={{ left: `${(s.distance / maxDistance) * 100}%` }}
                  >
                    <span className="track-label">{stationName(s.station_id) || '-'}</span>
                  </div>
                ))}
                {trackStops.length <= 20 && trackStops.slice(1).map((s, i) => {
                  const prev = trackStops[i];
                  const midpointPercent = ((prev.distance + s.distance) / 2 / maxDistance) * 100;
                  return (
                    <span
                      key={`seg-${prev.id}-${s.id}`}
                      className="track-distance"
                      style={{ left: `${midpointPercent}%` }}
                    >
                      {(s.distance - prev.distance).toFixed(1)} km
                    </span>
                  );
                })}
              </div>
            </div>
          </div>
        )}

        <div className="panel">
          <h2>Stops, in order</h2>
          <p style={{ color: '#889', fontSize: '0.8rem', marginTop: '-0.5rem', marginBottom: '1rem' }}>
            Ordered automatically by distance from origin: departure first, arrival last. Change a
            distance and the stop moves to where it belongs.
          </p>
          {sortedStops.map((stop, i) => {
            const isOrigin = i === 0;
            const prevDistance = isOrigin ? null : Number(sortedStops[i - 1].distance_from_origin || 0);
            return (
              <div className="stop-editor-row" key={stop.id}>
                <span className="stop-index">{i + 1}</span>
                <select
                  value={stop.station_id}
                  onChange={(e) => updateStop(stop.id, 'station_id', e.target.value)}
                  style={{ flex: 2, padding: '0.4rem', borderRadius: '6px', border: '1px solid rgba(0,0,0,0.15)' }}
                >
                  <option value="">Select a station…</option>
                  {allStations.map((s) => (
                    <option key={s.id} value={s.id}>{s.name}</option>
                  ))}
                </select>
                <input
                  type="number"
                  step="0.1"
                  min={isOrigin ? undefined : prevDistance}
                  placeholder={isOrigin ? undefined : `> ${prevDistance} km`}
                  title={isOrigin ? 'The departure point is always 0 km' : `Must be greater than ${prevDistance} km`}
                  value={isOrigin ? '0' : stop.distance_from_origin}
                  disabled={isOrigin}
                  onChange={(e) => updateStop(stop.id, 'distance_from_origin', e.target.value)}
                  style={{ width: '140px' }}
                />
                <button
                  type="button"
                  className="btn btn-danger btn-sm"
                  disabled={stops.length <= 2}
                  onClick={() => removeStop(stop.id)}
                >Remove</button>
              </div>
            );
          })}

          <div style={{ marginTop: '1rem', display: 'flex', gap: '0.5rem' }}>
            <button type="button" className="btn btn-ghost" onClick={addStop}>+ Add stop</button>
          </div>

          <div style={{ marginTop: '1.5rem' }}>
            <button className="btn btn-primary" onClick={handleSave} disabled={saving}>
              {saving ? 'Saving…' : isNew ? 'Create route' : 'Save changes'}
            </button>
            {!isNew && (
              <p style={{ color: '#889', fontSize: '0.8rem', marginTop: '0.5rem' }}>
                Saving creates a new version of this route. Existing trips keep running on the version they were scheduled under.
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
