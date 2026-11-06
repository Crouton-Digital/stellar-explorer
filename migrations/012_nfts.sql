SELECT
    pg_advisory_xact_lock(834502);

CREATE TABLE nft_collections (
    network text NOT NULL,
    contract_id text NOT NULL,
    first_ledger bigint NOT NULL CHECK (first_ledger > 0),
    payload jsonb NOT NULL,
    PRIMARY KEY (network, contract_id)
);

CREATE TABLE nft_transfers (
    network text NOT NULL,
    transaction_hash text NOT NULL,
    event_index integer NOT NULL CHECK (event_index >= 0),
    ledger_sequence bigint NOT NULL CHECK (ledger_sequence > 0),
    tx_index integer NOT NULL CHECK (tx_index > 0),
    op_index integer NOT NULL CHECK (op_index >= 0),
    contract_id text NOT NULL,
    token_id numeric(78, 0) NOT NULL CHECK (
        token_id >= 0
        AND token_id <= 115792089237316195423570985008687907853269984665640564039457584007913129639935
    ),
    token_id_type text NOT NULL CHECK (token_id_type IN ('u32', 'u64', 'u128', 'u256')),
    kind text NOT NULL CHECK (kind IN ('transfer', 'mint', 'burn')),
    from_address text NOT NULL,
    to_address text NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (network, transaction_hash, event_index),
    FOREIGN KEY (network, transaction_hash)
        REFERENCES transactions (network, hash),
    FOREIGN KEY (network, contract_id)
        REFERENCES nft_collections (network, contract_id),
    FOREIGN KEY (network, transaction_hash, event_index)
        REFERENCES contract_events (network, transaction_hash, event_index)
);

CREATE INDEX nft_transfers_order ON nft_transfers (
    network, ledger_sequence DESC, tx_index DESC, event_index DESC
);

CREATE INDEX nft_transfers_collection ON nft_transfers (
    network, contract_id, ledger_sequence DESC, tx_index DESC, event_index DESC
);

CREATE INDEX nft_transfers_token ON nft_transfers (
    network, contract_id, token_id, ledger_sequence DESC, tx_index DESC, event_index DESC
);

CREATE INDEX nft_transfers_from ON nft_transfers (network, from_address, ledger_sequence DESC);

CREATE INDEX nft_transfers_to ON nft_transfers (network, to_address, ledger_sequence DESC);
