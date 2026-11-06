# Bounded activity statistics

Overview, transaction lists and fungible movement lists display activity counts over a bounded indexed window. These help compare recent record density, transaction outcomes and movement kinds without implying complete network history, token volumes, balances or supply.

`GET /api/v1/activity/{kind}` accepts `transactions`, `transfers` or `events`. It uses exactly that list's supported filters and can use a compatible list cursor to anchor the same dataset and snapshot. The list-only `limit` parameter is rejected. Changing filters requires discarding the old cursor, as with the corresponding list endpoint.

Within selected ledger bounds and snapshot, the query chooses at most the latest **120 actual indexed ledger rows**, then groups ascending rows into at most twelve buckets of ten ledgers each. Each bucket includes first/last ledger, the last ledger's close time and a matching record count. Zero-match buckets remain present. Bucket duration can vary; these counts are not a transactions-per-second estimate. They cover the chosen window, rather than just the current list page or all stored history.

`first_ledger`, `last_ledger`, `from` and `to` identify the actual window. When no indexed ledgers fall in the selected window, bounds/timestamps are null, ledger count is zero and buckets are empty. A valid indexed window with no matching records keeps its bounds and zero-valued buckets. `total` and `breakdown` use the same window and filter conditions:

| Kind           | Breakdown keys                              |
| -------------- | ------------------------------------------- |
| `transactions` | `success`, `failed`                         |
| `transfers`    | `transfer`, `mint`, `burn`, `clawback`      |
| `events`       | `contract`, `system`, `diagnostic`, `other` |

Transaction success rate is the successful matching count divided by the matching transaction total; it is absent when no transactions match. Movement charts count records, never sum amounts from different assets. Event counts describe the committed indexed event store, not raw diagnostic trace volume. The `other` category is included even when zero and preserves counts of unrecognized future event types. These statistics do not include NFT movements in fungible totals; NFT collection/item counts have their own [snapshot semantics](nfts.md).

The chart shows a zero-based vertical count scale and selected bucket details. Pointer inspection, Left/Right/Home/End keyboard navigation and full-size previous/next controls expose each bucket. Expand **Activity data** for the same counts as readable text. Mobile views preserve the caption, scope and controls. Overview refreshes when its indexed checkpoint changes; list charts follow the selected filters and cursor.

The event statistics endpoint is available to API clients; the global UI uses charts on the three views above. Lists and transaction evidence continue to work when optional activity statistics are unavailable. See [OpenAPI](openapi.json), [filter rules](filters.md) and [testing](../tests/README.md).

Account and contract pages also show line charts of their own indexed transaction counts and outcomes. Their window ends at the newest transaction on the current page, keeping older or sparse entity activity visible. The request uses the corresponding exact account/contract filter and inclusive ledger bound. Moving to an older page moves the chart window with that page. Counts still come from the complete bounded query rather than only the displayed rows; lines connect bucket counts, not balances or token prices. Pages without observed transactions do not invent an activity series.
