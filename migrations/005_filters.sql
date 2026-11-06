CREATE INDEX calls_function ON contract_calls (
    network,
    function_name,
    contract_id,
    transaction_hash
);

CREATE INDEX events_order ON contract_events (
    network,
    ledger_sequence DESC,
    transaction_hash DESC,
    event_index DESC
);

CREATE INDEX events_name ON contract_events (
    network,
    (topics_json->0->>'value'),
    ledger_sequence DESC,
    transaction_hash DESC,
    event_index DESC
)
WHERE topics_json->0->>'type' = 'ScValTypeScvSymbol';
