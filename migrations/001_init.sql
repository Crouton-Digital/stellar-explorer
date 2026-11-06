CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY);

CREATE TABLE IF NOT EXISTS indexer_metadata (
    network text PRIMARY KEY,
    dataset_id text NOT NULL DEFAULT gen_random_uuid()::text
);

INSERT INTO
    indexer_metadata (network)
VALUES
    ('testnet')
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS ledgers (
    network text NOT NULL CHECK (network = 'testnet'),
    sequence bigint NOT NULL CHECK (sequence > 0),
    hash text NOT NULL,
    closed_at timestamptz NOT NULL,
    protocol_version integer NOT NULL,
    raw_ref text NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (network, sequence),
    UNIQUE (network, hash)
);

CREATE TABLE IF NOT EXISTS transactions (
    network text NOT NULL,
    hash text NOT NULL,
    ledger_sequence bigint NOT NULL,
    tx_index integer NOT NULL,
    source_account text NOT NULL,
    successful boolean NOT NULL,
    fee_charged numeric(20, 0) NOT NULL,
    created_at timestamptz NOT NULL,
    envelope_xdr text NOT NULL,
    result_xdr text NOT NULL,
    meta_xdr text NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (network, hash),
    UNIQUE (network, ledger_sequence, tx_index),
    FOREIGN KEY (network, ledger_sequence) REFERENCES ledgers (network, sequence)
);

CREATE INDEX IF NOT EXISTS transactions_order ON transactions (network, ledger_sequence DESC, tx_index DESC);

CREATE INDEX IF NOT EXISTS transactions_source ON transactions (network, source_account, ledger_sequence DESC);

CREATE TABLE IF NOT EXISTS operations (
    network text NOT NULL,
    transaction_hash text NOT NULL,
    op_index integer NOT NULL,
    type text NOT NULL,
    source_account text NOT NULL,
    details_json jsonb NOT NULL,
    PRIMARY KEY (network, transaction_hash, op_index),
    FOREIGN KEY (network, transaction_hash) REFERENCES transactions (network, hash)
);

CREATE TABLE IF NOT EXISTS contract_calls (
    network text NOT NULL,
    transaction_hash text NOT NULL,
    op_index integer NOT NULL,
    contract_id text NOT NULL,
    function_name text NOT NULL,
    arguments_json jsonb NOT NULL,
    decode_status text NOT NULL,
    PRIMARY KEY (network, transaction_hash, op_index),
    FOREIGN KEY (network, transaction_hash) REFERENCES transactions (network, hash)
);

CREATE INDEX IF NOT EXISTS calls_contract ON contract_calls (network, contract_id);

CREATE TABLE IF NOT EXISTS contract_events (
    network text NOT NULL,
    transaction_hash text NOT NULL,
    event_index integer NOT NULL,
    ledger_sequence bigint NOT NULL,
    contract_id text NOT NULL,
    event_type text NOT NULL,
    topics_json jsonb NOT NULL,
    value_json jsonb NOT NULL,
    raw_xdr text NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (network, transaction_hash, event_index),
    FOREIGN KEY (network, transaction_hash) REFERENCES transactions (network, hash)
);

CREATE INDEX IF NOT EXISTS events_contract ON contract_events (
    network,
    contract_id,
    ledger_sequence DESC,
    transaction_hash DESC,
    event_index DESC
);

CREATE TABLE IF NOT EXISTS transaction_accounts (
    network text NOT NULL,
    address text NOT NULL,
    transaction_hash text NOT NULL,
    PRIMARY KEY (network, address, transaction_hash),
    FOREIGN KEY (network, transaction_hash) REFERENCES transactions (network, hash)
);

CREATE TABLE IF NOT EXISTS indexer_checkpoints (
    network text PRIMARY KEY,
    first_ledger bigint NOT NULL,
    last_completed_ledger bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS indexer_status (
    network text PRIMARY KEY,
    latest_network_ledger bigint,
    observed_at timestamptz,
    last_error text,
    processed_transactions bigint NOT NULL DEFAULT 0,
    error_count bigint NOT NULL DEFAULT 0
);

INSERT INTO
    schema_migrations (version)
VALUES
    (1)
ON CONFLICT DO NOTHING;
