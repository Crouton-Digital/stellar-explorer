SELECT
    pg_advisory_xact_lock(834502);

CREATE TABLE contracts (
    network text NOT NULL,
    contract_id text NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (network, contract_id)
);

CREATE TABLE transaction_contracts (
    network text NOT NULL,
    contract_id text NOT NULL,
    transaction_hash text NOT NULL,
    PRIMARY KEY (network, contract_id, transaction_hash),
    FOREIGN KEY (network, contract_id)
        REFERENCES contracts (network, contract_id),
    FOREIGN KEY (network, transaction_hash)
        REFERENCES transactions (network, hash)
);
