SELECT
    pg_advisory_xact_lock(834502);

ALTER TABLE contract_events
    ADD COLUMN event_scope text NOT NULL DEFAULT 'operation'
        CHECK (event_scope IN ('operation', 'transaction'));

CREATE INDEX events_scope ON contract_events (
    network, event_scope, ledger_sequence DESC, transaction_hash DESC, event_index DESC
);

CREATE TABLE fee_events (
    network text NOT NULL,
    transaction_hash text NOT NULL,
    event_index integer NOT NULL,
    account text NOT NULL,
    amount numeric(39, 0) NOT NULL,
    stage text NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (network, transaction_hash, event_index),
    FOREIGN KEY (network, transaction_hash, event_index)
        REFERENCES contract_events (network, transaction_hash, event_index)
);
