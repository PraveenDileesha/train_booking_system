-- confirmed_at marks the moment a hold becomes a paid, confirmed booking, distinct from booking_timestamp, which marks when the hold was first placed.
-- Revenue reporting groups by confirmed_at, not booking_timestamp, since that is the day the passenger actually paid.
ALTER TABLE bookings
    ADD COLUMN confirmed_at TIMESTAMP;
