import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../api/client';
import { AdminNav, PageHeading, Pagination } from './AdminStations';

const PAGE_SIZE = 20;

export default function AdminRoutes() {
  const [routes, setRoutes] = useState([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  async function load(targetPage = page) {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listRoutes({ page: targetPage, pageSize: PAGE_SIZE });
      setRoutes(data.items);
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

  return (
    <div className="page-container">
      <AdminNav />
      <div className="admin-content">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
          <PageHeading title="Routes" />
          <Link to="/admin/routes/new" className="btn btn-primary">New route</Link>
        </div>

        {error && <div className="error-banner">{error}</div>}

        <div className="panel">
          {loading ? (
            <p className="empty-state">Loading…</p>
          ) : routes.length === 0 ? (
            <p className="empty-state">No routes yet. Create one to get started.</p>
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
                  {routes.map((r) => (
                    <tr key={r.id}>
                      <td>{r.id}</td>
                      <td>{r.name}</td>
                      <td style={{ textAlign: 'right' }}>
                        <Link to={`/admin/routes/${r.id}`} className="btn btn-ghost btn-sm">
                          View / edit
                        </Link>
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
