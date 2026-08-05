const ADMIN_BASE = '/api/v1/admin';
const BASE = '/api/v1';

async function requestFrom(base, path, options = {}) {
  const res = await fetch(`${base}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }

  if (res.status === 204) return null;
  return res.json();
}

const request = (path, options) => requestFrom(ADMIN_BASE, path, options);
const publicRequest = (path, options) => requestFrom(BASE, path, options);

// Every list endpoint returns { items, page, page_size, total, total_pages }.
// pageParams(...) turns { page, pageSize, sort } into the query string for it.
function pageParams({ page, pageSize, sort } = {}) {
  const params = new URLSearchParams();
  if (page) params.set('page', page);
  if (pageSize) params.set('page_size', pageSize);
  if (sort) params.set('sort', sort);
  const qs = params.toString();
  return qs ? `?${qs}` : '';
}

// Dropdown and select inputs need the whole list, not one page of it. Real pagination controls are only worth showing for the admin table views.
const ALL_PAGE_SIZE = 200;

export const api = {
  // Admin table uses numeric ID order (the backend's default).
  listStations: (opts) => request(`/stations${pageParams(opts)}`),
  // Every other picker or dropdown uses alphabetical order, since users scan by name.
  listAllStations: () => request(`/stations${pageParams({ pageSize: ALL_PAGE_SIZE, sort: 'name' })}`),
  createStation: (name) =>
    request('/stations', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteStation: (id) => request(`/stations/${id}`, { method: 'DELETE' }),

  listRoutes: (opts) => request(`/routes${pageParams(opts)}`),
  listAllRoutes: () => request(`/routes${pageParams({ pageSize: ALL_PAGE_SIZE })}`),
  getRoute: (id) => request(`/routes/${id}`),
  createRoute: (name, stations) =>
    request('/routes', { method: 'POST', body: JSON.stringify({ name, stations }) }),
  updateRouteStations: (id, stations) =>
    request(`/routes/${id}/stations`, { method: 'PUT', body: JSON.stringify({ stations }) }),
  deleteRoute: (id) => request(`/routes/${id}`, { method: 'DELETE' }),

  listCoaches: (opts) => request(`/coaches${pageParams(opts)}`),
  listAllCoaches: () => request(`/coaches${pageParams({ pageSize: ALL_PAGE_SIZE })}`),
  getCoach: (id) => request(`/coaches/${id}`),
  createCoach: (coach) => request('/coaches', { method: 'POST', body: JSON.stringify(coach) }),
  deleteCoach: (id) => request(`/coaches/${id}`, { method: 'DELETE' }),

  listTrips: (opts) => request(`/trips${pageParams(opts)}`),
  createTrip: (trip) => request('/trips', { method: 'POST', body: JSON.stringify(trip) }),
  deleteTrip: (id) => request(`/trips/${id}`, { method: 'DELETE' }),

  getTodayRevenue: () => request('/revenue/today'),
  listDailyRevenue: (days) => request(`/revenue/daily?days=${days}`),
  listRevenueBookings: (date, opts) => request(`/revenue/bookings?date=${date}${pageParams(opts).replace('?', '&')}`),

  searchTrips: (startStationId, endStationId, date, opts) =>
    publicRequest(
      `/trips?start_station_id=${startStationId}&end_station_id=${endStationId}&date=${date}` +
        pageParams(opts).replace('?', '&')
    ),
  getTrip: (id) => publicRequest(`/trips/${id}`),
  getTripSeats: (tripId, startStationId, endStationId) =>
    publicRequest(
      `/trips/${tripId}/seats?start_station_id=${startStationId}&end_station_id=${endStationId}`
    ),

  createBooking: (booking) =>
    publicRequest('/bookings', { method: 'POST', body: JSON.stringify(booking) }),
  confirmBooking: (id) => publicRequest(`/bookings/${id}/confirm`, { method: 'POST' }),
  getBooking: (id) => publicRequest(`/bookings/${id}`),

  issueUnreservedTicket: (ticket) =>
    publicRequest('/counter/tickets', { method: 'POST', body: JSON.stringify(ticket) }),
};
