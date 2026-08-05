import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api } from '../api/client';

const PAGE_SIZE = 20;

export default function AdminStations() {
  const [stations, setStations] = useState([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);

  async function load(targetPage = page) {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listStations({ page: targetPage, pageSize: PAGE_SIZE });
      setStations(data.items);
      setPage(data.page);
      setTotalPages(data.total_pages);
      setTotal(data.total);
    } catch (e) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(1); }, []);

  async function handleCreate(e) {
    e.preventDefault();
    if (!name.trim()) return;
    setCreating(true);
    setError(null);
    try {
      await api.createStation(name.trim());
      setName('');
      await load(1);
    } catch (e) {
      setError(e.message);
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id) {
    if (!confirm('Delete this station?')) return;
    setError(null);
    try {
      await api.deleteStation(id);
      const isLastRowOnPage = stations.length === 1 && page > 1;
      await load(isLastRowOnPage ? page - 1 : page);
    } catch (e) {
      setError(e.message);
    }
  }

  return (
    <div className="page-container">
      <AdminNav />
      <div className="admin-content">
        <PageHeading title="Stations" />

        {error && <div className="error-banner">{error}</div>}

        <div className="panel">
          <h2>Add a station</h2>
          <form className="form-row" onSubmit={handleCreate}>
            <input
              placeholder="Station name, e.g. Kandy"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <button className="btn btn-primary" disabled={creating}>
              {creating ? 'Adding…' : 'Add station'}
            </button>
          </form>
        </div>

        <div className="panel">
          <h2>All stations</h2>
          {loading ? (
            <p className="empty-state">Loading…</p>
          ) : stations.length === 0 ? (
            <p className="empty-state">No stations yet. Add one above to get started.</p>
          ) : (
            <>
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Name</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {stations.map((s) => (
                    <tr key={s.id}>
                      <td>{s.id}</td>
                      <td>{s.name}</td>
                      <td style={{ textAlign: 'right' }}>
                        {s.in_use ? (
                          <span style={{ color: '#889', fontSize: '0.8rem' }}>In use on a route</span>
                        ) : (
                          <button className="btn btn-danger btn-sm" onClick={() => handleDelete(s.id)}>
                            Delete
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <Pagination page={page} totalPages={totalPages} total={total} onChange={load} />
            </>
          )}
        </div>
      </div>
    </div>
  );
}

export function AdminNav() {
  return (
    <nav className="admin-nav">
      <span className="brand">Sri Lanka Railways</span>
    </nav>
  );
}

export function PageHeading({ title, subtitle, extra }) {
  const navigate = useNavigate();
  return (
    <div className="page-heading">
      <div className="page-heading-row">
        <button className="back-btn" onClick={() => navigate(-1)}>← Back</button>
        <h1>{title}</h1>
        {extra}
      </div>
      {subtitle && <p className="page-heading-subtitle">{subtitle}</p>}
    </div>
  );
}

// Shared page, next, and prev control for every paginated admin table.
// Which page numbers to render as buttons. Always first, last, and a window of 1 neighbor around the current page, with "…" filling gaps so a 50-page list doesn't render 50 buttons.
function pageWindow(current, total) {
  const keep = new Set([1, total, current - 1, current, current + 1]);
  const pages = [...keep].filter((p) => p >= 1 && p <= total).sort((a, b) => a - b);

  const withGaps = [];
  let prev = null;
  for (const p of pages) {
    if (prev !== null && p - prev > 1) withGaps.push('…');
    withGaps.push(p);
    prev = p;
  }
  return withGaps;
}

export function Pagination({ page, totalPages, total, onChange }) {
  if (totalPages <= 1) return null;
  return (
    <div className="pagination">
      <button
        type="button"
        className="btn btn-ghost btn-sm"
        disabled={page <= 1}
        onClick={() => onChange(page - 1)}
      >
        ← Prev
      </button>

      {pageWindow(page, totalPages).map((p, i) =>
        p === '…' ? (
          <span key={`gap-${i}`} className="pagination-ellipsis">…</span>
        ) : (
          <button
            key={p}
            type="button"
            className={`btn btn-sm ${p === page ? 'btn-primary' : 'btn-ghost'}`}
            disabled={p === page}
            onClick={() => onChange(p)}
          >
            {p}
          </button>
        )
      )}

      <button
        type="button"
        className="btn btn-ghost btn-sm"
        disabled={page >= totalPages}
        onClick={() => onChange(page + 1)}
      >
        Next →
      </button>
      <span className="pagination-status">({total} total)</span>
    </div>
  );
}
