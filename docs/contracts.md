# Contract metadata

Contract pages combine activity with metadata derived from retained transaction XDR. No RPC state lookup is required. The response from `GET /api/v1/contracts/{address}` includes a `contract` object alongside the existing transactions, events, indexed range, and history semantics. Transaction detail responses contain the observations made in that transaction under `contracts`.

## Evidence and unknown fields

| Field | Meaning |
|---|---|
| `kind` | `wasm`, `sac`, or `unknown`. A contract instance entry identifies its executable. A network-verified classic asset contract ID can also establish SAC identity. Calls or event shapes alone do not identify an executable. |
| `wasm_hash` | The last observed instance's WASM hash, or `null` for SAC and unknown executables. Uploading WASM alone does not establish a contract instance. |
| `executable_source` | `instance_entry`, `asset_contract_id`, or `null`. |
| `executable_ledger`, `executable_tx_index` | The indexed transaction position where the executable evidence was observed. |
| `first_observed_ledger`, `last_observed_ledger` | Indexed observations, including calls, events, token participation, and instance entries. First observed is not a creation date. |
| `creation_ledger`, `creation_transaction` | Present only when an invocation committed a newly created instance entry in that ledger. Explicit restorations and restore-footprint operations do not establish creation. |
| `creator` | The address in a top-level `createContract` or `createContractV2` preimage whose network-specific hash matches the created instance. This can be a G-address or C-address. The transaction source and fee payer are not used as substitutes. |

Factory-created instances can have a known creation ledger and transaction with an unknown creator. Internal creation preimages are not reconstructed. Asset-derived contract creation has no deployer address in its preimage, so its creator remains unknown.

The decoder reads persistent instance entries from transaction metadata before, during, and after successful operations, including v3 and v4 metadata. State, updated, created, and restored entries can supply executable evidence. Non-instance storage entries are excluded. A state entry's last-modified ledger is not treated as its creation ledger. Failed calls may establish that an address was referenced, but do not establish creation or executable details.

Contract aggregation follows ledger and transaction application order, including multiple code updates in one ledger. Later calls without instance evidence preserve the last confirmed executable. Verified asset identity does not overwrite stronger instance evidence. Unsupported executable variants remain `unknown`, with their source XDR preserved in transaction detail.

Metadata describes the latest evidence in the indexed range. It does not certify current deployment, liveness, token-interface compliance, source-code verification, ownership, or a complete invocation tree. Archived or removed contracts can still have historical metadata. Pagination fixes transaction membership at a checkpoint; aggregate metadata may improve as indexing advances.

## Storage and upgrades

Migration 4 adds `contracts` and `transaction_contracts`, then replays the existing envelope and metadata XDR in application order. It uses the ingestion advisory lock and commits the schema, derived metadata, transaction observations, and migration version together. Unreadable source XDR rolls back the migration. Original envelope/result/meta XDR, other transaction payload fields, dataset ID, checkpoint, and processing counters are preserved.

Stop old indexer binaries before upgrading so they cannot write transactions without the new derived metadata:

```bash
docker compose build
docker compose stop indexer
docker compose up -d
docker compose logs --tail=50 api indexer
```

Keep the existing database volume. Do not use `down -v` or replay an old fixture into the live index. The API starts serving once migration completes. On a large retained history, allow time for the atomic backfill before readiness succeeds.

## Protocol references

- [CAP-46-02: contract IDs, instance entries, and creation](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0046-02.md)
- [CAP-58: createContractV2 and constructors](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0058.md)
- [CAP-66: restoration and metadata changes](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0066.md)
- [Stellar documentation: contract code and instance entries](https://developers.stellar.org/docs/build/guides/rpc/retrieve-contract-code-js)
