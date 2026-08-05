import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { AdminNav, PageHeading, Pagination } from './AdminStations';

const CLASS_LABELS = {
  FIRST_AC: 'First Class AC (2+2)',
  SECOND: 'Second Class (2+2)',
  THIRD: 'Third Class (3+2)',
};

const PAGE_SIZE = 20;

export default function AdminCoaches() {
  const [coaches, setCoaches] = useState([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [creating, setCreating] = useState(false);

  const [coachName, setCoachName] = useState('');
  const [coachClass, setCoachClass] = useState('SECOND');
  const [isReservable, setIsReservable] = useState(true);
  const [rowCount, setRowCount] = useState(8);

  async function load(targetPage = page) {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listCoaches({ page: targetPage, pageSize: PAGE_SIZE });
      setCoaches(data.items);
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
    if (!coachName.trim() || rowCount < 1) return;
    setCreating(true);
    setError(null);
    try {
      await api.createCoach({
        coach_name: coachName.trim(),
        class: coachClass,
        is_reservable: isReservable,
        row_count: Number(rowCount),
      });
      setCoachName('');
      setRowCount(8);
      await load(1);
    } catch (e) {
      setError(e.message);
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id) {
    if (!confirm('Delete this coach and all its seats?')) return;
    setError(null);
    try {
      await api.deleteCoach(id);
      const isLastRowOnPage = coaches.length === 1 && page > 1;
      await load(isLastRowOnPage ? page - 1 : page);
    } catch (e) {
      setError(e.message);
    }
  }

  return (
    <div className="page-container">
      <AdminNav />
      <div className="admin-content">
        <PageHeading title="Coaches" />

        {error && <div className="error-banner">{error}</div>}

        <div className="panel">
          <h2>Add a coach</h2>
          <form onSubmit={handleCreate}>
            <div className="field-grid field-grid-coach">
              <label className="field">
                <span className="field-label">Coach name</span>
                <input
                  placeholder="e.g. 1st AC Saloon A"
                  value={coachName}
                  onChange={(e) => setCoachName(e.target.value)}
                />
              </label>
              <label className="field">
                <span className="field-label">Class</span>
                <select value={coachClass} onChange={(e) => setCoachClass(e.target.value)}>
                  {Object.entries(CLASS_LABELS).map(([value, label]) => (
                    <option key={value} value={value}>{label}</option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field-label">Rows</span>
                <input
                  type="number"
                  min="1"
                  value={rowCount}
                  onChange={(e) => setRowCount(e.target.value)}
                />
              </label>
            </div>

            <p className="field-hint">
              Seats are generated automatically from the class layout and row count.
            </p>

            <div className="form-footer">
              <label className="toggle-pill">
                <input
                  type="checkbox"
                  checked={isReservable}
                  onChange={(e) => setIsReservable(e.target.checked)}
                />
                Reserved seating (booked in advance)
              </label>
              <button className="btn btn-primary" disabled={creating}>
                {creating ? 'Adding…' : 'Add coach'}
              </button>
            </div>
          </form>
        </div>

        <div className="panel">
          <h2>All coaches</h2>
          {loading ? (
            <p className="empty-state">Loading…</p>
          ) : coaches.length === 0 ? (
            <p className="empty-state">No coaches yet. Add one above to get started.</p>
          ) : (
            <>
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Name</th>
                    <th>Class</th>
                    <th>Type</th>
                    <th>Rows</th>
                    <th>Capacity</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {coaches.map((c) => (
                    <tr key={c.id}>
                      <td>{c.id}</td>
                      <td>{c.coach_name}</td>
                      <td>{CLASS_LABELS[c.class] || c.class}</td>
                      <td>{c.is_reservable ? 'Reserved' : 'Unreserved'}</td>
                      <td>{c.row_count}</td>
                      <td>{c.capacity}</td>
                      <td style={{ textAlign: 'right' }}>
                        <button className="btn btn-danger btn-sm" onClick={() => handleDelete(c.id)}>
                          Delete
                        </button>
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
