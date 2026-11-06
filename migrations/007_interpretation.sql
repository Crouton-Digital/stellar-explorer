SELECT
    pg_advisory_xact_lock(834502);

ALTER TABLE transactions
    ADD COLUMN interpretation_version integer NOT NULL DEFAULT 0
        CHECK (interpretation_version >= 0);
