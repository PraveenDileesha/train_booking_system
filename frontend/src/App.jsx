import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom';
import AdminStations, { AdminNav } from './pages/AdminStations';
import AdminRoutes from './pages/AdminRoutes';
import AdminRouteDetail from './pages/AdminRouteDetail';
import AdminCoaches from './pages/AdminCoaches';
import AdminTrips from './pages/AdminTrips';
import AdminRevenue from './pages/AdminRevenue';
import CustomerSearch from './pages/CustomerSearch';
import CounterView from './pages/CounterView';

function AdminHome() {
  return (
    <div className="page-container admin-bg">
      <AdminNav />
      <div className="admin-content">
        <h1 style={{ marginBottom: '1.5rem' }}>Admin Dashboard</h1>
        <div className="card-grid">
          <Link to="/admin/stations" className="nav-card">
            <h3>Stations</h3>
            <p>Add, view and remove stations on the network.</p>
          </Link>
          <Link to="/admin/routes" className="nav-card">
            <h3>Routes</h3>
            <p>Build routes from stations and manage their versions.</p>
          </Link>
          <Link to="/admin/coaches" className="nav-card">
            <h3>Coaches</h3>
            <p>Add coaches by class and auto-generate their seat layout.</p>
          </Link>
          <Link to="/admin/trips" className="nav-card">
            <h3>Trips</h3>
            <p>Schedule trips, attach coaches and set per-class fares.</p>
          </Link>
          <Link to="/admin/revenue" className="nav-card">
            <h3>Revenue</h3>
            <p>Track confirmed booking revenue, today and day by day.</p>
          </Link>
        </div>
      </div>
    </div>
  );
}

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<CustomerSearch />} />
        <Route path="/counter" element={<CounterView />} />
        <Route path="/admin" element={<AdminHome />} />
        <Route path="/admin/stations" element={<AdminStations />} />
        <Route path="/admin/routes" element={<AdminRoutes />} />
        <Route path="/admin/routes/new" element={<AdminRouteDetail />} />
        <Route path="/admin/routes/:id" element={<AdminRouteDetail />} />
        <Route path="/admin/coaches" element={<AdminCoaches />} />
        <Route path="/admin/trips" element={<AdminTrips />} />
        <Route path="/admin/revenue" element={<AdminRevenue />} />
      </Routes>
    </Router>
  );
}

export default App;
