ALTER TABLE seats
    DROP CONSTRAINT seats_coach_id_fkey,
    ADD CONSTRAINT seats_coach_id_fkey
        FOREIGN KEY (coach_id) REFERENCES coaches(id) ON DELETE RESTRICT;
