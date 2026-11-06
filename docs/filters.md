# Activity filters

The Transactions, Events, Token movements, and NFT movements pages provide filter forms. Applying or resetting filters starts a new result set without the old cursor. URLs preserve the applied conditions, including across reloads, browser navigation, and pagination. A filtered view with no matches suggests adjusting the filters rather than starting the indexer.

## API parameters

| Endpoint                     | Optional filters                                                                  |
| ---------------------------- | --------------------------------------------------------------------------------- |
| `GET /api/v1/transactions`   | `address`, `contract`, `function`, `status`, `min_ledger`, `max_ledger`           |
| `GET /api/v1/events`         | `contract`, `event_type`, `event_name`, `event_scope`, `min_ledger`, `max_ledger` |
| `GET /api/v1/transfers`      | `asset`, `from`, `to`, `min_ledger`, `max_ledger`                                 |
| `GET /api/v1/nfts/transfers` | `collection`, `token_id`, `kind`, `from`, `to`, `min_ledger`, `max_ledger`        |

The same transaction filters apply to account/contract detail, ledger detail, and account transaction lists. Event filters also apply to contract event lists. Movement filters apply to asset detail and asset/account/contract movement lists. The path scope is always enforced together with the filters: a conflicting filter returns an empty page and does not broaden the query. Detail metadata remains the aggregate indexed metadata; filters affect the paginated activity section.

All conditions use **AND**. Addresses require the correct StrKey type and checksum. Functions and event names are exact, case-sensitive ASCII symbols of 1–32 letters, digits, or underscores. Ledger bounds are inclusive positive uint32 values; the minimum must not exceed the maximum. Unknown, repeated, or empty filter parameters return `400 invalid_filters`. Invalid limits or cursors return `400 invalid_pagination`. Valid filters with no matching indexed rows return `200` with `items: []` and `next_cursor: null`.

- `address` matches an indexed G-account participant or C-contract observation. It follows the existing partial account/contract history semantics, including token participants and instance observations. It is not a complete effects index.
- `contract` without `function` matches observed contract participation. When both are present, one top-level call must match both the contract and function. A function from a different call cannot satisfy the condition.
- `status` accepts `success` or `failed` and refers to transaction execution.
- `event_name` matches the first topic only when that topic is an SCVal symbol. `event_type` accepts `contract`, `system`, or `diagnostic` and selects stored committed events. It does not include separate raw diagnostic traces.
- `event_scope` accepts `operation` or `transaction`. Without it, both sources are included. Transaction events include CAP-67 fees/refunds, even for failed transactions. Their operation index is null and their stage is explicit. Event index ordering is not execution chronology; see [standards and fees](standards.md).
- `asset` is the emitting token contract C-address. `from` and `to` match the movement direction; a mint may have no sender and a burn may have no recipient.

## Pagination

Keep the same filter values for every page. Cursors are bound to the dataset, query kind, entity scope, filters, and an initial checkpoint. Query parameter order and page size can change. Changing or removing a filter while retaining the cursor returns `400`. Existing unfiltered cursors remain valid for unfiltered queries.

The snapshot excludes transactions arriving after the first page. Aggregate contract and asset metadata can still improve as indexing continues. Results cover the retained indexed range only.

```http
GET /api/v1/transactions?contract=C...&function=transfer&status=success&min_ledger=5569284
GET /api/v1/events?event_name=transfer&event_type=contract&limit=10
GET /api/v1/events?event_scope=transaction&event_name=fee&limit=10
GET /api/v1/transfers?asset=C...&from=G...&to=G...
```

The shortened addresses above are placeholders. Use complete valid StrKeys in requests. The [OpenAPI specification](openapi.json) and [example requests](requests.http) document concrete endpoints.

Migration 5 adds indexes for function lookup, global event pagination, and the first symbol topic. No source history is rewritten. Deploy with the normal Compose build/update process; keep the existing database volume.

NFT movement filters also apply to collection/item and account/contract NFT histories. IDs are exact canonical unsigned decimal strings through u256; zero is valid. Collection/item counts cover the entire entity through `snapshot_ledger`, while filters constrain only the movement page. See [NFT scope and snapshots](nfts.md). Bounded [activity statistics](activity.md) reuse their corresponding list filters and compatible cursor, but reject the list-only `limit` parameter.
