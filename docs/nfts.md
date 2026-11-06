# NFT event history

The NFT section records supported committed movement events in the indexed Testnet range. Collection and item pages expose observed identifiers and movement history; they do not establish current ownership, circulating supply, metadata, artwork or contract compliance. NFT identifiers are separate from fungible token amounts.

## Supported evidence

The decoder recognizes `transfer` and `mint` topics compatible with [draft SEP-50 at revision ced008c3](https://github.com/stellar/stellar-protocol/blob/ced008c3727005ca4a775f430fe6fe346c30f58f/ecosystem/sep-0050.md). It accepts an unsigned scalar ID or exactly `{"token_id": unsigned ID}` as the event body. Original unsigned `u32`, `u64`, `u128` and `u256` widths remain in `token_id_type`; `format` distinguishes `scalar` and `token_id_map`. IDs are canonical decimal strings, including zero, through the maximum u256 value. They are never converted to JavaScript numbers, formatted with token decimals or treated as a transfer quantity.

Historical [OpenZeppelin v0.4.0 event definitions](https://github.com/OpenZeppelin/stellar-contracts/blob/a62c805f1242aff369f2985e01f1371ba0fcfafe/packages/tokens/src/non_fungible/mod.rs) emitted scalar IDs; the [v0.8.0 implementation](https://github.com/OpenZeppelin/stellar-contracts/blob/ddc327eb92d63709436da3a8455f641c2d5476c5/packages/tokens/src/non_fungible/mod.rs) uses the default token-ID map. A supported `burn` event is explicitly labeled `event_standard: openzeppelin_burnable`, following the [burnable extension](https://github.com/OpenZeppelin/stellar-contracts/blob/ddc327eb92d63709436da3a8455f641c2d5476c5/packages/tokens/src/non_fungible/extensions/burnable/mod.rs); it is not an event defined by draft SEP-50. Source definitions explain event shapes, not the code identity of an arbitrary deployed emitter.

Only successful transaction operation events with a matching successful host-function result commitment create NFT records. The commitment covers the original operation events and return value. Failed operations, diagnostic traces, transaction-level fee events, malformed or signed IDs, extra map fields and mismatched commitments remain outside recognized NFT activity. Constructor-emitted events can be recognized when the same source commitment checks pass; a function name or call argument alone never creates an NFT movement. Fungible movement recognition remains separate. A marketplace transaction can contain an NFT transfer and an independent fungible payment without conflating ID and amount.

Each record retains its transaction, ledger, transaction application index, operation index, source event index, emitter, participants, exact ID, type and event format. `verification: event_shape` is not collection certification. No approval, live ownership, token URI, image or arbitrary external metadata request is performed.

## API and snapshots

| Endpoint                                             | Result                                                                                    |
| ---------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `GET /api/v1/nfts/collections`                       | Observed collection contracts, distinct observed ID counts and recorded movement counts   |
| `GET /api/v1/nfts/collections/{address}`             | Collection evidence, counts and a filtered movement page                                  |
| `GET /api/v1/nfts/collections/{address}/tokens/{id}` | Collection-scoped identifier, count and movement page                                     |
| `GET /api/v1/nfts/transfers`                         | Global NFT movements                                                                      |
| `GET /api/v1/nfts/collections/{address}/transfers`   | Collection movement history                                                               |
| `GET /api/v1/accounts/{address}/nft-transfers`       | Exact account participant history                                                         |
| `GET /api/v1/contracts/{address}/nft-transfers`      | Exact contract participant history, independent of generic contract metadata observations |

Movement filters are exact AND matches: `collection`, `token_id`, `kind`, `from`, `to`, `min_ledger` and `max_ledger`. `kind` is `transfer`, `mint` or `burn`. ID query/path values must be canonical unsigned decimal strings up to u256; signs, leading zeros other than zero itself, exponents and out-of-range values are rejected. An ID without a collection filter can match that identifier in several collections. Item identity is network, collection and exact canonical ID.

Lists use `network`, `limit` and `cursor`. NFT cursors bind to dataset, scope, filters and snapshot. Collection-list counts use that list's snapshot. Detail responses expose `snapshot_ledger`: counts cover the entire collection/item through that checkpoint, while filters constrain only the movement page. `range` is the current ingestion state and may advance beyond the cursor snapshot. Distinct observed IDs are not total supply, and movement counts are not the number of owners. Missing entries only mean absence from this index.

The UI provides `/nfts`, `/nfts/transfers`, collection and item pages, transaction NFT evidence, and account/contract NFT movement sections. Participant links target filtered normalized NFT history so a contract participant without a generic metadata record remains reachable. Full IDs wrap on narrow screens and can be copied from item pages. No current owner is inferred from the last indexed recipient.

## Upgrade and verification

Migration 12 derives normalized NFT collections/movements from retained source XDR under the ingestion lock and adds `nft_transfers` to transaction details. Existing interpretation version 5 remains unchanged. Source consistency is validated before enrichment; a source decoding or identity inconsistency rolls back the migration. Source XDR, prior interpretation/fungible evidence, dataset, checkpoint and counters are preserved. Replay of an unchanged ledger remains a no-op.

```bash
docker compose build
docker compose stop indexer
docker compose up -d
```

Keep the existing database volume. This source enrichment does not download older network history. See [testing](../tests/README.md) for real event fixtures, synthetic edge cases, PostgreSQL/API checks and desktop/mobile coverage, and [OpenAPI](openapi.json) for exact fields.
