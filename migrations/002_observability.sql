SELECT
    pg_advisory_xact_lock(834502);

ALTER TABLE indexer_status
ADD COLUMN IF NOT EXISTS processed_transactions bigint NOT NULL DEFAULT 0;

ALTER TABLE indexer_status
ADD COLUMN IF NOT EXISTS error_count bigint NOT NULL DEFAULT 0;

INSERT INTO
    indexer_status (network, processed_transactions)
SELECT
    network,
    count(*)
FROM
    transactions
GROUP BY
    network
ON CONFLICT (network) DO UPDATE
SET
    processed_transactions = excluded.processed_transactions;
