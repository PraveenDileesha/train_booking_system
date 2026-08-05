-- seats are a coach's own composition data (auto-created alongside the
-- coach), not usage of it, so deleting a coach cascades to its seats.
-- trip_coaches.coach_id stays ON DELETE RESTRICT: a coach actually
-- attached to a trip still cannot be deleted.

ALTER TABLE seats
    DROP CONSTRAINT seats_coach_id_fkey,
    ADD CONSTRAINT seats_coach_id_fkey
        FOREIGN KEY (coach_id) REFERENCES coaches(id) ON DELETE CASCADE;
