import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { AdminNav, PageHeading, Pagination } from './AdminStations';

const DAYS = 30;
const PAGE_SIZE = 10;

function todayStr() {
  return new Date().toISOString().slice(0, 10);
}

function RevenueBookingsTable({ bookings }) {
  return (
    <table className="admin-table">
      <thead>
        <tr>
          <th>Reference</th>
          <th>Passenger</th>
          <th>Route</th>
          <th>Trip</th>
          <th>Coach</th>
          <th>Seat</th>
          <th>Fare</th>
        </tr>
      </thead>
      <tbody>
        {bookings.map((b) => (
          <tr key={b.id}>
            <td>{b.booking_reference}</td>
            <td>{b.passenger_name}</td>
            <td>{b.start_station_name} → {b.end_station_name}</td>
            <td>{b.route_name} · {b.departure_date} {b.departure_time}</td>
            <td>{b.coach_name}</td>
            <td>{b.seat_number}</td>
            <td>Rs {b.fare}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function PreviousDayRow({ day }) {
  const [open, setOpen] = useState(false);
  const [bookings, setBookings] = useState([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  function load(targetPage = 1) {
    setLoading(true);
    setError(null);
    api
      .listRevenueBookings(day.date, { page: targetPage, pageSize: PAGE_SIZE })
      .then((data) => {
        setBookings(data.items);
        setPage(data.page);
        setTotalPages(data.total_pages);
        setTotal(data.total);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }

  function toggle() {
    const next = !open;
    setOpen(next);
    if (next && bookings.length === 0) load(1);
  }

  return (
    <div className="panel" style={{ marginBottom: '0.75rem' }}>
      <button
        type="button"
        onClick={toggle}
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          width: '100%',
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          padding: 0,
          font: 'inherit',
          color: 'inherit',
        }}
      >
        <span>{open ? '▾' : '▸'} {day.date}</span>
        <span style={{ color: '#889' }}>{day.booking_count} booking{day.booking_count === 1 ? '' : 's'} · Rs {day.revenue}</span>
      </button>

      {open && (
        <div style={{ marginTop: '1rem' }}>
          {error && <div className="error-banner">{error}</div>}
          {loading ? (
            <p className="empty-state">Loading…</p>
          ) : (
            <>
              <RevenueBookingsTable bookings={bookings} />
              <p style={{ textAlign: 'right', marginTop: '0.75rem', fontWeight: 600 }}>
                Total: Rs {day.revenue}
              </p>
              <Pagination page={page} totalPages={totalPages} total={total} onChange={load} />
            </>
          )}
        </div>
      )}
    </div>
  );
}

export default function AdminRevenue() {
  const [today, setToday] = useState(null);
  const [todayBookings, setTodayBookings] = useState([]);
  const [todayPage, setTodayPage] = useState(1);
  const [todayTotalPages, setTodayTotalPages] = useState(1);
  const [todayTotal, setTodayTotal] = useState(0);
  const [previousDays, setPreviousDays] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  function loadTodayBookings(targetPage = 1) {
    api
      .listRevenueBookings(todayStr(), { page: targetPage, pageSize: PAGE_SIZE })
      .then((data) => {
        setTodayBookings(data.items);
        setTodayPage(data.page);
        setTodayTotalPages(data.total_pages);
        setTodayTotal(data.total);
      })
      .catch((e) => setError(e.message));
  }

  useEffect(() => {
    setLoading(true);
    setError(null);
    Promise.all([api.getTodayRevenue(), api.listDailyRevenue(DAYS), api.listRevenueBookings(todayStr(), { page: 1, pageSize: PAGE_SIZE })])
      .then(([todayData, dailyData, todayBookingsData]) => {
        setToday(todayData);
        setPreviousDays(dailyData.days.filter((d) => d.date !== todayStr()));
        setTodayBookings(todayBookingsData.items);
        setTodayPage(todayBookingsData.page);
        setTodayTotalPages(todayBookingsData.total_pages);
        setTodayTotal(todayBookingsData.total);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="page-container">
      <AdminNav />
      <div className="admin-content">
        <PageHeading title="Revenue" subtitle="Confirmed reserved-seat bookings only. Unreserved counter sales are not yet included." />

        {error && <div className="error-banner">{error}</div>}

        {loading ? (
          <p className="empty-state">Loading…</p>
        ) : (
          <>
            <div className="panel">
              <h2>Today</h2>
              <p style={{ fontSize: '2rem', fontWeight: 700, margin: '0.5rem 0' }}>Rs {today.revenue}</p>
              <p style={{ color: '#889' }}>{today.booking_count} confirmed booking{today.booking_count === 1 ? '' : 's'}</p>
            </div>

            <div className="panel">
              <h2>Today's bookings</h2>
              {todayBookings.length === 0 ? (
                <p className="empty-state">No confirmed bookings today yet.</p>
              ) : (
                <>
                  <RevenueBookingsTable bookings={todayBookings} />
                  <Pagination page={todayPage} totalPages={todayTotalPages} total={todayTotal} onChange={loadTodayBookings} />
                </>
              )}
            </div>

            <h2 style={{ margin: '1.5rem 0 0.75rem' }}>Previous days</h2>
            {previousDays.length === 0 ? (
              <p className="empty-state">No confirmed bookings on earlier days in this period.</p>
            ) : (
              previousDays.map((day) => <PreviousDayRow key={day.date} day={day} />)
            )}
          </>
        )}
      </div>
    </div>
  );
}
