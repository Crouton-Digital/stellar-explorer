# Crouton Stellar Explorer

Standalone explorer MVP for **Stellar Testnet**, built with a Go indexer, PostgreSQL, a read-only REST API, and Nuxt 4 / Vue 3. Displays real ledgers, transactions, classic payments and path-payment swaps, supported Soroswap and Aquarius exchanges, NFT event history, top-level Soroban calls, committed events, and before/after state changes. Recent activity charts use a bounded indexed range. No private Crouton packages are required.

Documentation: [startup guide](docs/run.md), [UI guide and screen map](docs/ui-guide.md), [architecture](docs/architecture.md), [database schema and ER diagram](docs/database.md), [token movements](docs/tokens.md), [NFT history](docs/nfts.md), [activity statistics](docs/activity.md), [contract metadata](docs/contracts.md), [activity filters](docs/filters.md), [standards and fee accounting](docs/standards.md), [transaction interpretation](docs/interpretation.md), and [testing](tests/README.md).

## Quick start

Requires Docker Engine and Docker Compose v2 or later. Create the configuration if it does not already exist, then start the services:

```bash
if [ ! -f .env ]; then
    cp .env.example .env
fi

docker compose up --build -d
```

Explorer: **http://localhost:3000**, API: http://localhost:8080/api/v1/overview, OpenAPI: http://localhost:8080/openapi.json. On first startup, the services apply database migrations, then the indexer loads up to 100 previous available ledgers and continues following the network. Tables may be empty until the first ledger is committed.

By default, ports are bound to the loopback interface. `.env.example` contains values for local development only. If you change the PostgreSQL password, also update the URL-encoded password in `DATABASE_URL`. When running locally without Docker, replace the hostname `db` with `localhost`. Do not publish `.env`.

Follow indexer logs with `docker compose logs -f indexer`. Stop the services with `docker compose down`; the database volume and indexed history are preserved. API readiness depends on database availability. The overview displays synchronization status and the latest RPC observation. If RPC becomes unavailable, the explorer continues serving stored data.

## Reproducible demo without RPC

The repository includes real Testnet ledger **5569284**, captured on November 5, 2026: 12 transactions covering a classic payment, a Soroban invocation, events, and a failed transaction. For offline replay, use a separate empty database/volume and start only `db`, `api`, and `web`:

```bash
docker compose up --build -d db api web
docker compose run --rm indexer /app/replay /fixtures/testnet.json
```

Open ledger 5569284, navigate to a Soroban transaction, and inspect its call, typed arguments, events, and raw XDR. Try searching by transaction hash, ledger sequence, or G/C address. In offline mode, the network tip and indexer lag are unknown; importing a fixture does not count as a live RPC observation.

Use a separate database for live indexing. The historical fixture may already be outside the RPC retention window, in which case resuming from it fails with a history gap. Do not automatically delete existing data to restart the demo.

## Configuration

| Environment variable | Default / description                                                                                          |
| -------------------- | -------------------------------------------------------------------------------------------------------------- |
| `DATABASE_URL`       | Required; PostgreSQL DSN                                                                                       |
| `POSTGRES_PORT`      | `5432`; database host port for Docker Compose. If occupied, use `5433`; the internal address remains `db:5432` |
| `STELLAR_NETWORK`    | `testnet`; other values are rejected                                                                           |
| `STELLAR_RPC_URL`    | `https://soroban-testnet.stellar.org`                                                                          |
| `START_LEDGER`       | `0` → `max(oldest, latest-100)` for an empty database only                                                     |
| `BATCH_SIZE`         | `10`; range: 1–200                                                                                             |
| `POLL_INTERVAL`      | `5s`; a positive Go duration                                                                                   |
| `RETRY_MAX`          | `30s`; maximum exponential backoff delay                                                                       |
| `API_LISTEN`         | `:8080`                                                                                                        |
| `API_MAX_LIMIT`      | `100`; range: 1–200; default page limit: `min(20, max)`                                                        |
| `NUXT_API_BASE`      | Server-side Go API address; Compose sets `http://api:8080`                                                     |

A saved checkpoint takes precedence over `START_LEDGER`. An unavailable explicit starting ledger, a history gap, a network mismatch, or a broken ledger chain stops the indexer with a diagnostic message. After a Testnet reset, explicitly create a new database/volume; existing data is neither mixed with the new network history nor deleted. Cursors from another database are rejected.

## Code style

Readability is a project requirement: use expanded functions and structures, multiline SQL/Vue/CSS, and blank lines between logical blocks. Write documentation and code comments in English. Editor settings are in `.editorconfig`, and frontend formatting settings are in `web/.prettierrc.json`.

```bash
make format
make format-check
```

`make format` uses a pinned version of golines for Go and Prettier for the frontend. CI also checks Prettier formatting. To format or check only the frontend, run `npm run format` or `npm run format:check` from `web/`.

## Development and checks

The project pins Go **1.27.3**, `github.com/stellar/go-stellar-sdk` **v0.8.0**, Node **22.21.1**, Nuxt **4.2.2**, and PostgreSQL 16. Go/npm lockfiles are included. The SQL migration is embedded in the Go binaries; both the API and indexer apply it under an advisory lock.

```bash
export DATABASE_URL='postgres://crouton:crouton-local@localhost:5432/crouton?sslmode=disable'
go run ./cmd/indexer
# In another terminal with the same DATABASE_URL:
go run ./cmd/api
# In a third terminal:
cd web
npm ci
npm run dev
```

Run `go test ./...`, `go vet ./...`, and `go build -buildvcs=false ./cmd/...` for Go. For the frontend, run `npm test`, `npm run typecheck`, and `npm run build` from `web/`. The `-buildvcs=false` flag allows builds outside a complete Git checkout. PostgreSQL integration tests require `TEST_DATABASE_URL` and use a separate schema for each test. CI runs these integration tests and Playwright tests for desktop and mobile. Full commands and coverage of AC-01…AC-09 are documented in [tests/README.md](tests/README.md).

The API uses a consistent JSON error format and cursor pagination. See the [OpenAPI 3.0.3 specification](docs/openapi.json) and [example requests](docs/requests.http). Transaction lists return summaries; raw envelope/result/meta XDR and operations are available from the transaction detail endpoint.

Transaction details also provide versioned actions, failure explanations, classic path-payment swaps, supported Soroswap and Aquarius router exchanges and state changes. Failed operations remain attempts, committed token effects use metadata evidence, and diagnostic errors never determine transaction success. The UI separates actual swap amounts from request limits and operation sources from token traders, and shows before/after state values while retaining raw data. Router interpretation requires registered Testnet deployments and reconciled source evidence. Aquarius supports direct/chained exact input, chained strict receive with net spending and unused-input returns, and bounded single-group exact-input split routes with exactly divisible allocations. Split branches retain separate paths and actual totals; unsupported rounding or group forms remain generic calls. Pool addresses are matched from events and transfers, not independently registry-verified. An unobserved router WASM hash remains unknown. See [transaction interpretation](docs/interpretation.md) for scope and migration of retained history.

## Data sources and limitations

Initial token support decodes committed SEP-41-compatible transfer/burn events and verified SAC mint/clawback events. The Assets section and entity pages show exact amounts and identification provenance. Unknown token decimals remain unknown. Existing histories are enriched automatically by migration 3; see the [upgrade procedure and supported formats](docs/tokens.md).

NFT history recognizes supported SEP-50-shaped transfer/mint events and a labeled burnable extension. Collection-scoped unsigned IDs remain exact decimal strings through u256, with scalar/map format provenance. Observed ID/movement counts are snapshot counts, not supply or owners. Migration 12 enriches retained source XDR; no live metadata, images, approval inventory or current ownership is inferred. See [NFT evidence and scope](docs/nfts.md).

Overview and transaction/movement lists show matching counts over at most 120 indexed ledgers, including zero buckets, outcomes or movement-kind breakdowns. Filters and compatible list cursors anchor the same query scope. These charts do not sum token amounts or establish full-network throughput. Account and contract pages also show line charts ending at the newest transaction on the current page. See [bounded statistics](docs/activity.md).

Contract pages show observed SAC/WASM identity, the last observed WASM hash, and creation ledger, transaction, and deployer when supported by retained XDR. Missing evidence remains unknown; first observation is not a creation date. Migration 4 enriches existing history without resetting the index. See [contract metadata and upgrades](docs/contracts.md).

Transactions, Events, and Token movements have filter forms with shareable URLs. Filters include indexed participants, contracts, top-level functions, execution status, event names/types, token assets, movement direction, and inclusive ledger bounds. Pagination preserves the conditions and rejects cursors from a different filter set. See [activity filters](docs/filters.md) for endpoint-specific parameters and semantics.

Data comes from [Stellar RPC getLedgers](https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getLedgers), with the available ledger range determined through [getHealth](https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getHealth). The explorer maintains a partial index rather than a full network archive. Testnet is [reset periodically](https://developers.stellar.org/docs/networks). Local fixtures remain available even after the original transaction leaves the RPC retention window.

Account history includes transaction source accounts, fee accounts, operation source accounts, recognized destinations for payment, create-account, and account-merge operations, and decoded token senders/recipients. It is not a complete effects index. Balances are neither calculated from partial history nor displayed. The header button opens a centered search palette over a dimmed, blurred background. It previews partial ledger, hash and G/C-address matches while typing, with keyboard selection and explicit loading/empty states. See [live search](docs/search.md). Search results and 404 responses refer only to indexed data.

Soroban support covers top-level `invokeContract` calls, return values when present in meta v3/v4, committed events, and available raw diagnostics. Internal calls are not reconstructed. Unsupported operations and SCVal types are marked `unknown`, with raw XDR preserved. Structurally unreadable XDR stops ledger import. Amounts are returned as strings. CAP-67 metadata-v4 transaction events expose initial fees, refunds, stages, and net fees, including failed transactions and fee bumps. A complete breakdown must reconcile with `TransactionResult.feeCharged`; absent or invalid breakdowns remain explicitly unknown. An event does not necessarily represent a transfer. See the [support matrix and accounting rules](docs/standards.md).

The MVP does not include Mainnet support, historical backfill, complete SEP-50/CAP-67 support, arbitrary Soroban swap protocols or nested swaps, balance/supply analytics, a production SLA, a security audit, API keys, or rate limits. Before publication, the owner must decide on public hosting and licensing, verify permission to reuse third-party code, and confirm SCF eligibility. This local implementation does not establish completion of a grant milestone or entitlement to future tranches.
