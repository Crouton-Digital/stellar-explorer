# Standards support

This matrix records implemented explorer behavior, not certification of arbitrary contracts. Reviewed on November 5, 2026 against the primary specifications linked below and the pinned Stellar Go SDK v0.8.0.

| Standard or format                                                                                                                                         | Implemented                                                                                                                                                                                                                                           | Remaining scope                                                                                                                                                                                        |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [SEP-41, draft v0.5.2](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0041.md)                                                      | Committed fungible transfer/burn events, exact nonnegative i128 amounts, scalar/map amount bodies, optional muxed destination data, raw fallback                                                                                                      | Custom-contract mint/clawback interpretation, complete interface/approval interpretation and token metadata discovery; event shape alone does not establish contract compliance                        |
| [Stellar Asset Contract / CAP-46-6](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0046-06.md)                                           | Network-specific asset identity, classic code/issuer, seven decimals, verified mint/clawback events and legacy issuer mint/burn semantics                                                                                                             | Complete historical reconstruction and reconciliation of movements against ledger entry changes                                                                                                        |
| [CAP-67, final, protocol 23](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0067.md)                                                     | Metadata-v4 operation and transaction events, source/stage provenance, committed fees/refunds including failed and fee-bump transactions, exact fee totals reconciled with the transaction result                                                     | Pre-event classic history reconstruction, full asset-balance reconciliation and dedicated authorization interpretation; no claim of complete historical coverage                                       |
| [SEP-50, pinned draft](https://github.com/stellar/stellar-protocol/blob/ced008c3727005ca4a775f430fe6fe346c30f58f/ecosystem/sep-0050.md)                    | Committed transfer/mint event shapes with scalar or token_id-map unsigned IDs through u256; collection/item history, snapshot counts and separate fungible evidence; labeled OpenZeppelin burnable extension                                          | Approvals, current ownership, complete lifecycle/supply, metadata/artwork, full interface verification and arbitrary collection certification                                                          |
| Transaction metadata v0–v4                                                                                                                                 | Committed state writes with before/after projections, baseline provenance, unknown/archived states and raw fallback; v3/v4 add top-level call/action descriptions, readable failures, typed diagnostics, available return values and committed events | Current balances, ledger-level state changes, internal call trees, complete application narratives and other application-specific adapters                                                             |
| Classic path payments                                                                                                                                      | Strict-send/receive limits and reconciled actual swaps from successful order-book and liquidity-pool result claims                                                                                                                                    | Cyclic same-asset conversion reconstruction and unsupported result formats                                                                                                                             |
| [Soroswap router, pinned Testnet deployment](https://github.com/soroswap/core/blob/6eade008ee6794075ad941609e0fef0d1124d3eb/public/testnet.contracts.json) | Exact-input/output swaps with registered router, typed request, matching return/event amounts, request limits, deterministic pool route and committed transfers; actual trader and source evidence retained                                           | Other routers/protocols, nested swaps, aggregators, ambiguous additional token effects, repeated-token paths, pool-price reconstruction and historical WASM identity when unobserved                   |
| [Aquarius router, published Testnet deployment](https://docs.aqua.network/developers/code-examples/prerequisites-and-basics)                               | Direct/chained exact input, chained strict receive with net funding/return settlement, and bounded single-group exact-input splits with exactly divisible weights; committed result, router/pool events and transfers retained                        | Other split groups/rounding forms, exact-output splits, pre-payout input returns, nested swaps, independent pool-registry mapping, pool fee interpretation and pinned/audited deployment-code identity |

SEP-50 describes **non-fungible tokens**. Its token IDs must be modeled separately from fungible transfer amounts. Draft interfaces can change; future upgrades require fixture review and a documented matrix update.

## Evidence and interpretation boundaries

Only successful operation events create token movements. Diagnostic events never create activity, even when marked as belonging to a successful internal call. CAP-67 transaction-level events are processed independently of transaction success: paying fees remains committed when operations fail.

A reserved SAC address identifies an asset; it does not prove that a contract instance was deployed. Fee events do not create a token movement, contract deployment or contract-call participation record. Unknown transaction event kinds remain accessible with their original XDR.

Existing operation event indexes are preserved. Transaction events are appended to each transaction's event array, with `event_scope: transaction`, `op_index: null` and an explicit `stage`. Event indexes and paginated ordering are identifiers, not a reconstruction of execution order across a ledger. Use stages to distinguish the initial fee phase from operation application and the refund phase.

## Fee and refund accounting

Transaction details and summaries expose a `fees` object:

```json
{
  "status": "complete",
  "source": "transaction_events",
  "charged": "1000",
  "refunded": "900",
  "net": "100",
  "events": [
    {
      "event_index": 1,
      "account": "G...",
      "amount": "1000",
      "stage": "before_all_transactions"
    },
    {
      "event_index": 2,
      "account": "G...",
      "amount": "-900",
      "stage": "after_all_transactions"
    }
  ],
  "reason": null
}
```

All amounts are exact stroop strings. A positive or zero signed event amount represents a charge; a negative amount represents a refund. `refunded` is returned as a nonnegative magnitude, and `net = charged - refunded`. The account is the outer fee payer for fee-bump transactions.

`complete` requires the native asset's Testnet contract ID, the correct fee payer, valid topic/value structure, protocol-appropriate stages, one charge and at most one refund, a nonnegative net amount, and agreement with `TransactionResult.feeCharged`. Refunds before protocol 23 occur after the transaction; starting with protocol 23 they occur after all transactions. No refund event means zero refund only within an otherwise complete, reconciled fee stream.

When transaction fee events are absent, `status` is `unavailable`. Malformed or inconsistent fee events produce `invalid`. Both statuses keep `charged`, `refunded` and `net` null, with a machine-readable `reason`. Original events are retained for inspection. Structurally valid fee records may remain in `events` for an invalid summary; they are not a verified total. The existing `fee_charged` field always retains the transaction result value.

Metadata v3 and older do not provide this CAP-67 fee stream. This implementation does not infer their original charge/refund breakdown from the result alone.

## Upgrade and verification

Migration 6 adds normalized `fee_events`, event source indexing, and fee summaries for retained transactions. It reads saved metadata under the ingestion lock in one atomic migration. Existing operation/movement IDs, source XDR, dataset identity, checkpoint and counters remain unchanged. Unreadable metadata or disagreement with a saved operation event rolls back the migration.

Build first, then stop the old indexer before updating services:

```bash
docker compose build
docker compose stop indexer
docker compose up -d
```

Keep the existing database volume. This enrichment does not download historical network data.

Decoder tests cover success/failure, fee bumps, refunds, protocol stages, missing/invalid data, signed i128 arithmetic and every transaction in the real Testnet fixture. PostgreSQL tests cover atomic writes, migration rollback, source preservation, replay and filtered pagination. Browser tests verify reconciled/unknown summaries and the custom event-source filter on desktop and mobile. See [testing](../tests/README.md).
