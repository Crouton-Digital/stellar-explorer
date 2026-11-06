SELECT
    pg_advisory_xact_lock(834502);

CREATE TABLE assets (
    network text NOT NULL,
    contract_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('sac', 'contract')),
    first_ledger bigint NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (network, contract_id)
);

CREATE INDEX assets_order ON assets (network, first_ledger DESC, contract_id DESC);

CREATE TABLE token_transfers (
    network text NOT NULL,
    transaction_hash text NOT NULL,
    event_index integer NOT NULL,
    ledger_sequence bigint NOT NULL,
    tx_index integer NOT NULL,
    op_index integer NOT NULL,
    contract_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('transfer', 'mint', 'burn', 'clawback')),
    from_address text NOT NULL,
    to_address text NOT NULL,
    amount numeric(39, 0) NOT NULL CHECK (amount >= 0),
    payload jsonb NOT NULL,
    PRIMARY KEY (network, transaction_hash, event_index),
    FOREIGN KEY (network, transaction_hash) REFERENCES transactions (network, hash),
    FOREIGN KEY (network, contract_id) REFERENCES assets (network, contract_id),
    FOREIGN KEY (network, transaction_hash, event_index)
        REFERENCES contract_events (network, transaction_hash, event_index)
);

CREATE INDEX transfers_order ON token_transfers (
    network, ledger_sequence DESC, tx_index DESC, event_index DESC
);

CREATE INDEX transfers_asset ON token_transfers (
    network, contract_id, ledger_sequence DESC, tx_index DESC, event_index DESC
);

CREATE INDEX transfers_from ON token_transfers (network, from_address, ledger_sequence DESC);

CREATE INDEX transfers_to ON token_transfers (network, to_address, ledger_sequence DESC);
