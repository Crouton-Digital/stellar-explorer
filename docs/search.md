# Live indexed search

Click the header search button, press `/`, or use Ctrl/Cmd+K to open a centered search palette, following the interaction of explorer-v2. The background dims and blurs while the palette is open; background scrolling and interaction are blocked. Keyboard shortcuts leave text inputs alone.

The empty palette is a single input row, with the supported search types in its placeholder: transaction hash, account, contract and ledger. It grows only when a query needs results, loading feedback or an error. Clearing the input collapses it again. The panel is at most 640px wide and 480px tall, with smaller limits inside narrow or short viewports.

Matching entities appear while typing. Results show their type and full identifier, remain in the Crouton theme, and open only when selected. Use Up/Down and Enter, or click a result. Tab stays inside the palette. Press Escape, click the close button, or click the backdrop to dismiss it and return focus to the header button. Closing clears the query and cancels pending requests. The palette adapts to narrow screens and the visible viewport when a mobile keyboard opens; results scroll inside it.

`GET /api/v1/search/suggestions?q=...` accepts a canonical positive decimal ledger prefix, at least four hexadecimal hash characters, or at least four characters of a G/C address. Hash matching is case-insensitive; address suggestions normalize case to uppercase. Complete address checksums are required by the existing exact resolver, rather than for an incomplete prefix.

The response contains `network`, the trimmed `query`, `limit` and `items`. Each item has `type`, `key` and `path`. Types are `ledger`, `transaction`, `account` and `contract`. There are at most five matches per type and ten overall; decimal input can match both ledgers and transaction hashes. Exact matches precede longer matches. Empty results describe the local indexed range and do not prove absence on Testnet. The existing `/api/v1/search` endpoint still resolves complete identifiers and returns explicit errors on submission.

The UI waits 220ms after input before querying, cancels obsolete requests, and ignores late responses for a previous query. Loading, empty and unavailable states are explicit. Search previews do not navigate automatically, and unavailable requests can be retried. Account identifiers are deduplicated even when present in many indexed transactions.

Migration 13 adds three C-collation prefix indexes. Ledger prefixes use bounded numeric ranges; identifiers use bounded byte-wise index scans. Account lookup skips each previously returned address to avoid scanning its entire transaction history. The migration adds indexes without rebuilding history, changing the dataset or resetting ingestion checkpoints.

See [OpenAPI](openapi.json) for the response schema and [activity statistics](activity.md) for entity-specific charts.
