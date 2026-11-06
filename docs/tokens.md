# Token movements

The explorer derives token activity from committed operation events in successful transactions. Failed transaction diagnostics and transaction-level fee events do not create token movements. Every movement retains its transaction hash, zero-based operation/event indexes, ledger sequence, and exact integer amount. Original event XDR remains available on the transaction page.

## Supported formats

- SEP-41-compatible `transfer` and `burn` events, with address topics and nonnegative `i128` amounts.
- Scalar amounts and map bodies containing `amount`, with optional `to_muxed_id`. Unknown map fields and additional topics are tolerated. Duplicate map keys, malformed amounts, and invalid addresses remain raw events without a decoded movement.
- Verified SAC `mint` and `clawback` events. Metadata v3 uses legacy admin topics; v4 uses the updated topics. Pre-v23 SAC transfers involving the issuer are classified as mint or burn.

SAC identity is verified by deriving the network-specific contract ID from the classic asset identifier and comparing it with the emitting contract. Native XLM is also identified by its derived contract ID. Verified Stellar assets use seven decimals. An arbitrary contract emitting a compatible event is labeled `event_shape`; its code, issuer, and decimals remain unknown. Event compatibility does not certify contract behavior or prove SEP-41 compliance.

Amounts are JSON strings and use `BigInt` in the UI. Unknown decimals are displayed as base units. The index does not calculate balances, supply, or complete historical coverage.

Primary references: [SEP-41](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0041.md), [CAP-67 event transition](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0067.md), and the [pinned SDK's SAC validation](https://github.com/stellar/go-stellar-sdk/blob/v0.8.0/processors/token_transfer/contract_events.go).

## API and UI

| Endpoint | Purpose |
|---|---|
| `/api/v1/assets` | Observed token contracts |
| `/api/v1/assets/{address}` | Asset identity, indexed range, and movement page |
| `/api/v1/assets/{address}/transfers` | Movements emitted by a token contract |
| `/api/v1/transfers` | All decoded token movements |
| `/api/v1/accounts/{address}/transfers` | Movements involving an account |
| `/api/v1/contracts/{address}/transfers` | Movements emitted by or involving a contract |

Lists support `limit` and dataset-bound cursors. Movement order is ledger, transaction application index, then event index, descending. Asset lists use first-observed ledger and contract ID. Cursors preserve the indexed range and reject reuse for another query or address. Asset identification reflects the strongest evidence observed so far.

Open **Assets** in the navigation bar. Transaction, account, and contract pages also display token movements. Recipient accounts enter indexed transaction history even when they were not transaction sources.

## Upgrade existing data

Migration 3 creates `assets` and `token_transfers`, then reconstructs token activity from retained transaction metadata and raw events. It runs atomically under the ingestion lock. A decoding or write error rolls back the migration; checkpoint, raw XDR, dataset ID, and ingestion counters are preserved. Migration reruns do not duplicate movements.

Stop the old indexer before upgrading so an older binary cannot keep writing transactions without token data:

```bash
docker compose build
docker compose stop indexer
docker compose up -d
```

The API and indexer apply the migration on startup. Existing database volumes remain in place. No RPC history download or database reset is required for this enrichment.

## Boundaries

This is initial SEP-41/SAC movement support. Transaction-level fees/refunds are handled separately; see [standards and fee accounting](standards.md). Supported NFT event history uses separate models and tables; see [NFT evidence and scope](nfts.md). Full CAP-67 asset-balance reconciliation, pre-event classic transfer reconstruction, arbitrary token metadata discovery, historical backfill, and internal call-tree reconstruction remain unimplemented. Approvals are retained as raw events rather than treated as movements. SEP-41 custom-contract mint/clawback events remain raw; this decoder interprets mint/clawback only for verified SACs.
